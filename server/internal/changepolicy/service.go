// Package changepolicy is the hub's policy module (docs/devagotchi/14): tenant-scoped, versioned policy documents
// evaluated by policyrules, change tickets with evidence and reviews, and a decision log in the hash-chained events.
// It is the in-process gate on the bridge authorize path and fails closed.
package changepolicy

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sync"

	"github.com/jackc/pgx/v5"

	"agenthub/internal/policyrules"
	"agenthub/internal/store"
)

// Service owns policy docs, tickets and reviews.
type Service struct {
	St    *store.Store
	mu    sync.Mutex
	cache map[string]cached // tenant -> last parsed doc
}

type cached struct {
	version int64
	sha     string
	doc     *policyrules.Doc
}

// q is the read side of a write transaction (store.W satisfies it).
type q interface {
	Row(sql string, args ...any) pgx.Row
	Query(sql string, args ...any) (pgx.Rows, error)
}

// DocRow is one stored version.
type DocRow struct {
	Tenant    string          `json:"tenant_id"`
	Version   int64           `json:"version"`
	SHA256    string          `json:"sha256"`
	Author    string          `json:"author"`
	Reason    string          `json:"reason"`
	CreatedAt string          `json:"created_at"`
	Doc       json.RawMessage `json:"doc,omitempty"`
}

// current loads the tenant's highest version, parsing it at most once per version.
func (s *Service) current(w q, tenant string) (cached, error) {
	var c cached
	if err := w.Row(`SELECT version, sha256 FROM policy_docs WHERE tenant_id=$1 ORDER BY version DESC LIMIT 1`, tenant).Scan(&c.version, &c.sha); err != nil {
		return c, store.Fail(503, "policy_unavailable", "no policy document for tenant %s (failing closed)", tenant)
	}
	s.mu.Lock()
	hit, ok := s.cache[tenant]
	s.mu.Unlock()
	if ok && hit.version == c.version && hit.sha == c.sha {
		return hit, nil
	}
	var raw []byte
	if err := w.Row(`SELECT doc FROM policy_docs WHERE tenant_id=$1 AND version=$2`, tenant, c.version).Scan(&raw); err != nil {
		return c, err
	}
	c.doc = &policyrules.Doc{}
	if err := json.Unmarshal(raw, c.doc); err != nil {
		return c, store.Fail(503, "policy_unavailable", "policy v%d does not parse (failing closed)", c.version)
	}
	c.doc.Version = c.version
	s.mu.Lock()
	if s.cache == nil {
		s.cache = map[string]cached{}
	}
	s.cache[tenant] = c
	s.mu.Unlock()
	return c, nil
}

// tenantOf is the machine's tenant ("local" when there is no machine).
func tenantOf(w q, machine string) string {
	t := "local"
	if machine != "" {
		_ = w.Row(`SELECT tenant_id FROM machines WHERE id=$1`, machine).Scan(&t)
	}
	return t
}

func shaJSON(v any) string {
	b, _ := json.Marshal(v)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

