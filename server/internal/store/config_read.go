package store

import (
	"context"
	"encoding/json"

	"agenthub/internal/policy"
)

// Roles lists roles.
func (s *Store) Roles(ctx context.Context) ([]Role, error) {
	rows, err := s.DB.Query(ctx, `SELECT id,description,version FROM roles ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Role{}
	for rows.Next() {
		var r Role
		if err := rows.Scan(&r.ID, &r.Description, &r.Version); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// Policies lists rules, highest priority first.
func (s *Store) Policies(ctx context.Context) ([]policy.Rule, error) {
	rs, err := loadRules(ctx, s.DB)
	if rs == nil {
		rs = []policy.Rule{}
	}
	sortRules(rs)
	return rs, err
}

// Workflows lists workflows.
func (s *Store) Workflows(ctx context.Context) ([]Workflow, error) {
	rows, err := s.DB.Query(ctx, `SELECT id,name,states,transitions,lanes,initial_state,attention_state,version FROM workflows ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Workflow{}
	for rows.Next() {
		var id, name, ini, att string
		var st, tr, la []byte
		var v int64
		if err := rows.Scan(&id, &name, &st, &tr, &la, &ini, &att, &v); err != nil {
			return nil, err
		}
		wf, err := decodeWorkflow(id, name, st, tr, la, ini, att, v)
		if err != nil {
			return nil, err
		}
		out = append(out, wf)
	}
	return out, rows.Err()
}

// Projects lists projects.
func (s *Store) Projects(ctx context.Context) ([]Project, error) {
	rows, err := s.DB.Query(ctx, `SELECT id,name,workflow_id,version FROM projects ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Project{}
	for rows.Next() {
		var p Project
		if err := rows.Scan(&p.ID, &p.Name, &p.WorkflowID, &p.Version); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// SettingsList returns every setting with its description.
func (s *Store) SettingsList(ctx context.Context) ([]SettingRow, error) {
	rows, err := s.DB.Query(ctx, `SELECT key,value,description,version FROM settings ORDER BY key`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SettingRow{}
	for rows.Next() {
		var r SettingRow
		var v []byte
		if err := rows.Scan(&r.Key, &v, &r.Description, &r.Version); err != nil {
			return nil, err
		}
		r.Value = json.RawMessage(v)
		out = append(out, r)
	}
	return out, rows.Err()
}

// SettingsMap returns settings as a map (for handlers that need a value).
func (s *Store) SettingsMap(ctx context.Context) (Settings, error) { return loadSettings(ctx, s.DB) }
