// Package auditkeep owns action-feed retention: per-category settings, daily partitions, archive segments
// (gzip JSONL, hash-chained manifests, optional ed25519 signature) and the prune job (40-audit-retention.md).
package auditkeep

import (
	"context"
	"regexp"
	"sync"
	"time"

	"agenthub/internal/store"
)

// Category is one audit category row (audit_categories). Zero limits mean "no limit of that kind".
type Category struct {
	Category       string    `json:"category"`
	Enabled        bool      `json:"enabled"`
	RetainDays     int       `json:"retain_days"`
	MaxRows        int64     `json:"max_rows"`
	MaxBytes       int64     `json:"max_bytes"`
	Archive        bool      `json:"archive"`
	CaptureOutput  bool      `json:"capture_output"`
	MaxOutputBytes int       `json:"max_output_bytes"`
	Description    string    `json:"description"`
	Version        int64     `json:"version"`
	UpdatedAt      time.Time `json:"updated_at"`
	UpdatedBy      string    `json:"updated_by"`
}

var catRe = regexp.MustCompile(`^[a-z][a-z0-9_]{0,30}$`)

// Service is the retention service. Sink and Signer are optional: no sink = no archive.
type Service struct {
	St     *store.Store
	Sink   Sink
	Signer Signer
	Now    func() time.Time // tests move the clock

	mu     sync.Mutex
	cache  map[string]Category
	loaded time.Time
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}

const cols = `category, enabled, retain_days, max_rows, max_bytes, archive, capture_output, max_output_bytes, description,
	version, updated_at, updated_by`

// Categories lists every category row.
func (s *Service) Categories(ctx context.Context) ([]Category, error) {
	rows, err := s.St.DB.Query(ctx, `SELECT `+cols+` FROM audit_categories ORDER BY category`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Category{}
	for rows.Next() {
		var c Category
		if err := rows.Scan(&c.Category, &c.Enabled, &c.RetainDays, &c.MaxRows, &c.MaxBytes, &c.Archive, &c.CaptureOutput,
			&c.MaxOutputBytes, &c.Description, &c.Version, &c.UpdatedAt, &c.UpdatedBy); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// Lookup returns a category from a short-lived cache (writers call it on every event). ok=false: unknown.
func (s *Service) Lookup(ctx context.Context, cat string) (Category, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cache == nil || time.Since(s.loaded) > 2*time.Second {
		if all, err := s.Categories(ctx); err == nil {
			s.cache = map[string]Category{}
			for _, c := range all {
				s.cache[c.Category] = c
			}
			s.loaded = time.Now()
		}
	}
	c, ok := s.cache[cat]
	return c, ok
}

func (s *Service) forget() { s.mu.Lock(); s.cache = nil; s.mu.Unlock() }

// Put creates or updates a category (optimistic version check when Version > 0) and records it in the chain.
func (s *Service) Put(ctx context.Context, p *store.Principal, rid string, c Category) (any, error) {
	if !catRe.MatchString(c.Category) {
		return nil, store.ErrInvalid("category must match %s", catRe)
	}
	if c.RetainDays < 0 || c.MaxRows < 0 || c.MaxBytes < 0 || c.MaxOutputBytes < 0 || c.MaxOutputBytes > 16<<20 {
		return nil, store.ErrInvalid("limits must be >= 0 and max_output_bytes <= 16 MiB")
	}
	out, err := s.St.Write(ctx, p, rid, "audit_category_put", c, func(w *store.W) (any, error) {
		if err := w.Admin("audit"); err != nil {
			return nil, err
		}
		var cur int64
		err := w.Row(`SELECT version FROM audit_categories WHERE category=$1 FOR UPDATE`, c.Category).Scan(&cur)
		if err == nil && c.Version > 0 && c.Version != cur {
			return nil, store.ErrVersion(map[string]any{"version": cur})
		}
		_, err = w.Exec(`INSERT INTO audit_categories(category, enabled, retain_days, max_rows, max_bytes, archive, capture_output,
			max_output_bytes, description, updated_by) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
			ON CONFLICT (category) DO UPDATE SET enabled=$2, retain_days=$3, max_rows=$4, max_bytes=$5, archive=$6,
			capture_output=$7, max_output_bytes=$8, description=$9, updated_by=$10, updated_at=now(),
			version=audit_categories.version+1`,
			c.Category, c.Enabled, c.RetainDays, c.MaxRows, c.MaxBytes, c.Archive, c.CaptureOutput, c.MaxOutputBytes, c.Description, p.ID)
		if err != nil {
			return nil, err
		}
		return nil, w.Event("audit.category.put", nil, nil, c)
	})
	s.forget()
	if err != nil {
		return out, err
	}
	if err := s.EnsureCategory(ctx, c.Category); err != nil {
		return nil, err
	}
	return s.get(ctx, c.Category)
}

func (s *Service) get(ctx context.Context, cat string) (Category, error) {
	all, err := s.Categories(ctx)
	for _, c := range all {
		if c.Category == cat {
			return c, nil
		}
	}
	if err == nil {
		err = store.ErrNotFound("audit category " + cat)
	}
	return Category{}, err
}
