package store

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5"

	"agenthub/internal/policy"
)

type querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Settings is the settings table as a map; Int falls back only when a row was deleted.
type Settings map[string]json.RawMessage

// Int reads an integer setting.
func (s Settings) Int(key string, fallback int) int {
	var v int
	if raw, ok := s[key]; ok && json.Unmarshal(raw, &v) == nil {
		return v
	}
	return fallback
}

// Strings reads a list setting.
func (s Settings) Strings(key string) []string {
	var v []string
	_ = json.Unmarshal(s[key], &v)
	return v
}

func loadSettings(ctx context.Context, q querier) (Settings, error) {
	rows, err := q.Query(ctx, `SELECT key, value FROM settings`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := Settings{}
	for rows.Next() {
		var k string
		var v []byte
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		out[k] = v
	}
	return out, rows.Err()
}

func loadRules(ctx context.Context, q querier) ([]policy.Rule, error) {
	rows, err := q.Query(ctx, `SELECT id::text,name,subject,action,selector,effect,priority,enabled,version FROM policies`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []policy.Rule
	for rows.Next() {
		var r policy.Rule
		var sel []byte
		if err := rows.Scan(&r.ID, &r.Name, &r.Subject, &r.Action, &sel, &r.Effect, &r.Priority, &r.Enabled, &r.Version); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(sel, &r.Selector)
		out = append(out, r)
	}
	return out, rows.Err()
}

func loadWorkflowFor(ctx context.Context, q querier, project string) (Workflow, error) {
	var id, name, initial, attn string
	var st, tr, la []byte
	var ver int64
	err := q.QueryRow(ctx, `SELECT w.id,w.name,w.states,w.transitions,w.lanes,w.initial_state,w.attention_state,w.version
		FROM projects p JOIN workflows w ON w.id=p.workflow_id WHERE p.id=$1`, project).
		Scan(&id, &name, &st, &tr, &la, &initial, &attn, &ver)
	if err == pgx.ErrNoRows {
		return Workflow{}, errNotFound("project " + project)
	}
	if err != nil {
		return Workflow{}, err
	}
	return decodeWorkflow(id, name, st, tr, la, initial, attn, ver)
}

func subjectOf(p *Principal) policy.Subject {
	return policy.Subject{ID: p.ID, Kind: p.Kind, IsHuman: p.IsHuman, Roles: p.Roles}
}

// can evaluates a policy inside the current transaction.
func (w *W) can(action string, res policy.Resource) error {
	rules, err := loadRules(w.ctx, w.tx)
	if err != nil {
		return err
	}
	if d := policy.Decide(rules, subjectOf(w.P), action, res); !d.Allowed {
		return errDenied(action, d.Rule)
	}
	return nil
}

// Can evaluates a policy outside a write (used by the UI to show/hide actions).
func (s *Store) Can(ctx context.Context, p *Principal, action string, res policy.Resource) (policy.Decision, error) {
	rules, err := loadRules(ctx, s.DB)
	if err != nil {
		return policy.Decision{}, err
	}
	return policy.Decide(rules, subjectOf(p), action, res), nil
}

func jsonUnmarshal(raw []byte, v any) error { return json.Unmarshal(raw, v) }
