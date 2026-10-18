package changepolicy

import (
	"context"
	"encoding/json"
	"strings"

	"agenthub/internal/policyrules"
	"agenthub/internal/store"
)

// PutIn is an edit of the whole document. BaseVersion must equal the current version (optimistic concurrency).
type PutIn struct {
	Tenant      string          `json:"tenant_id"`
	BaseVersion int64           `json:"base_version"`
	Doc         json.RawMessage `json:"doc"`
	Reason      string          `json:"reason"`
}

// Put validates and appends a new version. Humans only, and a data policy must allow policy.doc.edit.
func (s *Service) Put(ctx context.Context, p *store.Principal, reqID string, in PutIn) (any, error) {
	return s.St.Write(ctx, p, reqID, "policy_doc_put", in, func(w *store.W) (any, error) {
		if p.ID != store.System.ID {
			if err := w.Human("policy.doc.edit"); err != nil {
				return nil, err
			}
		}
		return s.put(w, p, in)
	})
}

func (s *Service) put(w *store.W, p *store.Principal, in PutIn) (any, error) {
	in.Tenant = firstOr(in.Tenant, "local")
	if len(strings.TrimSpace(in.Reason)) < 3 {
		return nil, store.ErrInvalid("say why (reason, at least 3 characters)")
	}
	var d policyrules.Doc
	if err := json.Unmarshal(in.Doc, &d); err != nil {
		return nil, store.ErrInvalid("doc is not a policy document: %v", err)
	}
	if err := d.Validate(); err != nil {
		return nil, store.ErrInvalid("%v", err)
	}
	var cur int64
	_ = w.Row(`SELECT coalesce(max(version),0) FROM policy_docs WHERE tenant_id=$1`, in.Tenant).Scan(&cur)
	if cur != in.BaseVersion {
		return nil, store.ErrVersion(map[string]any{"version": cur})
	}
	d.Version, d.UpdatedAt, d.UpdatedBy = 0, "", ""
	sha := shaJSON(d)
	b, _ := json.Marshal(d)
	if _, err := w.Exec(`INSERT INTO policy_docs(tenant_id,version,doc,sha256,author,reason) VALUES($1,$2,$3,$4,$5,$6)`,
		in.Tenant, cur+1, b, sha, p.ID, strings.TrimSpace(in.Reason)); err != nil {
		return nil, err
	}
	ids := []string{}
	for _, r := range d.Rules {
		ids = append(ids, r.ID+map[bool]string{true: "", false: "(off)"}[r.Enabled])
	}
	err := w.Event("policy.doc.version", nil, nil, map[string]any{"tenant": in.Tenant, "version": cur + 1, "sha256": sha, "reason": in.Reason, "rules": ids})
	return map[string]any{"tenant_id": in.Tenant, "version": cur + 1, "sha256": sha}, err
}

// Rollback appends a new version whose content is a copy of an older one.
func (s *Service) Rollback(ctx context.Context, p *store.Principal, reqID, tenant string, to, base int64, reason string) (any, error) {
	return s.St.Write(ctx, p, reqID, "policy_doc_rollback", []any{tenant, to, base, reason}, func(w *store.W) (any, error) {
		if err := w.Human("policy.doc.edit"); err != nil {
			return nil, err
		}
		var raw []byte
		if err := w.Row(`SELECT doc FROM policy_docs WHERE tenant_id=$1 AND version=$2`, firstOr(tenant, "local"), to).Scan(&raw); err != nil {
			return nil, store.ErrNotFound("policy version")
		}
		return s.put(w, p, PutIn{Tenant: tenant, BaseVersion: base, Doc: raw, Reason: "rollback to v" + itoa(to) + ": " + reason})
	})
}

// Current returns the current version with its document.
func (s *Service) Current(ctx context.Context, tenant string) (DocRow, error) {
	var d DocRow
	err := s.St.DB.QueryRow(ctx, `SELECT tenant_id, version, sha256, author, reason, to_char(created_at,'YYYY-MM-DD"T"HH24:MI:SSOF'), doc
		FROM policy_docs WHERE tenant_id=$1 ORDER BY version DESC LIMIT 1`, firstOr(tenant, "local")).
		Scan(&d.Tenant, &d.Version, &d.SHA256, &d.Author, &d.Reason, &d.CreatedAt, &d.Doc)
	if err != nil {
		return d, store.ErrNotFound("policy document")
	}
	return d, nil
}

// Versions lists every version (newest first) without bodies, or one version with its body.
func (s *Service) Versions(ctx context.Context, tenant string, one int64) ([]DocRow, error) {
	rows, err := s.St.DB.Query(ctx, `SELECT tenant_id, version, sha256, author, reason, to_char(created_at,'YYYY-MM-DD"T"HH24:MI:SSOF'),
		CASE WHEN $2::bigint = version THEN doc END FROM policy_docs WHERE tenant_id=$1 AND ($2=0 OR version=$2) ORDER BY version DESC`, firstOr(tenant, "local"), one)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []DocRow{}
	for rows.Next() {
		var d DocRow
		var doc []byte
		if err := rows.Scan(&d.Tenant, &d.Version, &d.SHA256, &d.Author, &d.Reason, &d.CreatedAt, &doc); err != nil {
			return nil, err
		}
		d.Doc = doc
		out = append(out, d)
	}
	return out, rows.Err()
}

// SeedDoc writes v1 for a tenant that has no policy yet (first boot only; never overwrites edits).
func SeedDoc(w *store.W, tenant string, doc json.RawMessage) (int, error) {
	var n int
	if err := w.Row(`SELECT count(*) FROM policy_docs WHERE tenant_id=$1`, tenant).Scan(&n); err != nil || n > 0 {
		return 0, err
	}
	_, err := (&Service{}).put(w, store.System, PutIn{Tenant: tenant, Doc: doc, Reason: "preset changepolicy/default.json"})
	return 1, err
}
