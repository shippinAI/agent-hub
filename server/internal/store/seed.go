package store

import (
	"context"
	"encoding/json"

	"agenthub/internal/policy"
)

// SeedData is the subset of a preset the seeder needs (kept here to avoid an import cycle).
type SeedData struct {
	Settings   []SettingRow
	Roles      []Role
	Principals []Principal
	Workflows  []Workflow
	Projects   []Project
	Policies   []policy.Rule
}

// Seed inserts preset rows that don't exist yet (never overwrites edits). Policies seed only into an empty table.
func (s *Store) Seed(ctx context.Context, d SeedData, adminLogin string) (map[string]int, error) {
	out, err := s.write(ctx, System, "", "seed", nil, func(w *W) (any, error) {
		n := map[string]int{}
		ins := func(kind, sql string, args ...any) error {
			tag, err := w.exec(sql+` ON CONFLICT DO NOTHING`, args...)
			n[kind] += int(tag.RowsAffected())
			return err
		}
		for _, r := range d.Settings {
			if err := ins("settings", `INSERT INTO settings(key,value,description) VALUES($1,$2,$3)`, r.Key, []byte(r.Value), r.Description); err != nil {
				return nil, err
			}
		}
		for _, r := range d.Roles {
			if err := ins("roles", `INSERT INTO roles(id,description) VALUES($1,$2)`, r.ID, r.Description); err != nil {
				return nil, err
			}
		}
		for _, wf := range d.Workflows {
			if err := wf.Validate(); err != nil {
				return nil, err
			}
			st, _ := json.Marshal(wf.States)
			tr, _ := json.Marshal(wf.Transitions)
			la, _ := json.Marshal(wf.Lanes)
			if err := ins("workflows", `INSERT INTO workflows(id,name,states,transitions,lanes,initial_state,attention_state) VALUES($1,$2,$3,$4,$5,$6,$7)`,
				wf.ID, wf.Name, st, tr, la, wf.InitialState, wf.AttentionState); err != nil {
				return nil, err
			}
		}
		for _, p := range d.Projects {
			if err := ins("projects", `INSERT INTO projects(id,name,workflow_id) VALUES($1,$2,$3)`, p.ID, p.Name, p.WorkflowID); err != nil {
				return nil, err
			}
		}
		for _, p := range d.Principals {
			var login *string
			if p.IsHuman && adminLogin != "" && hasRole(p.Roles, "admin") {
				login = &adminLogin
			}
			if err := ins("principals", `INSERT INTO principals(id,display_name,kind,is_human,login,roles,enabled) VALUES($1,$2,$3,$4,$5,$6,$7)`,
				p.ID, p.DisplayName, p.Kind, p.IsHuman, login, p.Roles, p.Enabled); err != nil {
				return nil, err
			}
		}
		var have int
		if err := w.row(`SELECT count(*) FROM policies`).Scan(&have); err != nil {
			return nil, err
		}
		for _, r := range d.Policies {
			if have > 0 {
				break
			}
			sel, _ := json.Marshal(r.Selector)
			if err := ins("policies", `INSERT INTO policies(name,subject,action,selector,effect,priority,enabled) VALUES($1,$2,$3,$4,$5,$6,true)`,
				r.Name, r.Subject, r.Action, sel, r.Effect, r.Priority); err != nil {
				return nil, err
			}
		}
		return n, w.Event("admin.seed", nil, nil, n)
	})
	if err != nil {
		return nil, err
	}
	return out.(map[string]int), nil
}

func hasRole(rs []string, r string) bool {
	for _, x := range rs {
		if x == r {
			return true
		}
	}
	return false
}
