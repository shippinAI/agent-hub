package guides

import (
	"context"
	"encoding/json"
	"time"

	"agenthub/internal/guidekit"
	"agenthub/internal/store"
)

// Ctx is the caller of a hook: request context, principal and request id.
type Ctx struct {
	context.Context
	P     *store.Principal
	ReqID string
}

// Guide is a guide row with its current version summary.
type Guide struct {
	ID            string          `json:"id"`
	Slug          string          `json:"slug"`
	Title         string          `json:"title"`
	Goal          string          `json:"goal"`
	Tags          []string        `json:"tags"`
	AppliesTo     json.RawMessage `json:"applies_to"`
	Source        string          `json:"source"`
	Overrides     *string         `json:"overrides_guide_id"`
	Channel       string          `json:"channel"`
	Enabled       bool            `json:"enabled"`
	Version       int64           `json:"version"`
	CurrentID     *string         `json:"current_version_id"`
	Semver        *string         `json:"semver"`
	Steps         json.RawMessage `json:"steps,omitempty"`
	RiskIDs       []string        `json:"risk_ids"`
	PendingUpdate bool            `json:"pending_update"`
}

const guideCols = `g.id,g.slug,g.title,g.goal,g.tags,g.applies_to,g.source,g.overrides_guide_id,g.channel,g.enabled,g.version,
	g.current_version_id::text,v.semver,coalesce(v.risk_ids,'{}'),EXISTS(SELECT 1 FROM guide_updates u WHERE u.guide_id=g.id AND u.status='pending')`

func scanGuide(r interface{ Scan(...any) error }, extra ...any) (Guide, error) {
	var g Guide
	err := r.Scan(append([]any{&g.ID, &g.Slug, &g.Title, &g.Goal, &g.Tags, &g.AppliesTo, &g.Source, &g.Overrides, &g.Channel, &g.Enabled,
		&g.Version, &g.CurrentID, &g.Semver, &g.RiskIDs, &g.PendingUpdate}, extra...)...)
	return g, err
}

// List returns guides, optionally filtered by a search term over title, goal, slug and tags.
func (s *Service) List(ctx context.Context, q string) ([]Guide, error) {
	rows, err := s.St.DB.Query(ctx, `SELECT `+guideCols+` FROM guides g LEFT JOIN guide_versions v ON v.id=g.current_version_id
		WHERE $1='' OR g.title ILIKE '%'||$1||'%' OR g.goal ILIKE '%'||$1||'%' OR g.slug ILIKE '%'||$1||'%' OR $1=ANY(g.tags)
		ORDER BY g.source, g.title`, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Guide{}
	for rows.Next() {
		g, err := scanGuide(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

// Get returns one guide with its current steps.
func (s *Service) Get(ctx context.Context, id string) (Guide, error) {
	var steps []byte
	g, err := scanGuide(s.St.DB.QueryRow(ctx, `SELECT `+guideCols+`,coalesce(v.steps,'[]') FROM guides g
		LEFT JOIN guide_versions v ON v.id=g.current_version_id WHERE g.id=$1 OR g.slug=$1`, id), &steps)
	if err != nil {
		return g, store.ErrNotFound("guide " + id)
	}
	g.Steps = steps
	return g, nil
}

// Effective resolves a guide id to what runs: an enabled tenant override with a published version wins.
func (s *Service) Effective(ctx context.Context, id string) (Guide, error) {
	var oid string
	if s.St.DB.QueryRow(ctx, `SELECT id FROM guides WHERE overrides_guide_id=$1 AND enabled AND current_version_id IS NOT NULL LIMIT 1`, id).Scan(&oid) == nil {
		return s.Get(ctx, oid)
	}
	return s.Get(ctx, id)
}

// VersionRow is one stored version.
type VersionRow struct {
	ID          string          `json:"id"`
	Semver      string          `json:"semver"`
	Status      string          `json:"status"`
	ChangeNotes string          `json:"change_notes"`
	AuthoredBy  string          `json:"authored_by"`
	SHA         string          `json:"content_sha256"`
	CreatedAt   time.Time       `json:"created_at"`
	Steps       json.RawMessage `json:"steps"`
}

// Versions lists a guide's versions, newest first.
func (s *Service) Versions(ctx context.Context, id string) ([]VersionRow, error) {
	rows, err := s.St.DB.Query(ctx, `SELECT id::text,semver,status,change_notes,authored_by,content_sha256,created_at,steps FROM guide_versions
		WHERE guide_id=$1 ORDER BY created_at DESC`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []VersionRow{}
	for rows.Next() {
		var v VersionRow
		if err := rows.Scan(&v.ID, &v.Semver, &v.Status, &v.ChangeNotes, &v.AuthoredBy, &v.SHA, &v.CreatedAt, &v.Steps); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// Diff compares two versions (by version id) step by step.
func (s *Service) Diff(ctx context.Context, a, b string) ([]guidekit.Change, error) {
	load := func(id string) ([]guidekit.Step, error) {
		var raw []byte
		if err := s.St.DB.QueryRow(ctx, `SELECT steps FROM guide_versions WHERE id::text=$1`, id).Scan(&raw); err != nil {
			return nil, store.ErrNotFound("guide version " + id)
		}
		return guidekit.Parse(raw)
	}
	x, err := load(a)
	if err != nil {
		return nil, err
	}
	y, err := load(b)
	if err != nil {
		return nil, err
	}
	return guidekit.Diff(x, y), nil
}
