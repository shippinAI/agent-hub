package store

import (
	"context"
	"encoding/json"

	"agenthub/internal/policy"
)

// versioned runs the create-or-update dance shared by every config table.
func (w *W) versioned(table, id string, version int64, insert func() error, update func() error) error {
	var cur int64
	err := w.row(`SELECT version FROM `+table+` WHERE id=$1 FOR UPDATE`, id).Scan(&cur)
	switch {
	case err != nil && version == 0:
		return insert()
	case err != nil:
		return errNotFound(table + " " + id)
	case version != cur:
		e := errVersion(nil)
		e.Current = map[string]any{"version": cur}
		return e
	}
	return update()
}

// Role is a named bag that policies refer to.
type Role struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	Version     int64  `json:"version"`
}

// PutRole creates or updates a role.
func (s *Store) PutRole(ctx context.Context, p *Principal, reqID string, r Role) (any, error) {
	return s.write(ctx, p, reqID, "put_role", r, func(w *W) (any, error) {
		if err := w.admin("roles.put"); err != nil {
			return nil, err
		}
		if r.ID == "" {
			return nil, errInvalid("role id required")
		}
		err := w.versioned("roles", r.ID, r.Version,
			func() error {
				_, e := w.exec(`INSERT INTO roles(id,description) VALUES($1,$2)`, r.ID, r.Description)
				return e
			},
			func() error {
				_, e := w.exec(`UPDATE roles SET description=$2,version=version+1 WHERE id=$1`, r.ID, r.Description)
				return e
			})
		if err != nil {
			return nil, err
		}
		return r, w.Event("admin.role.put", nil, nil, r)
	})
}

// PutPolicy creates (empty ID) or updates a rule.
func (s *Store) PutPolicy(ctx context.Context, p *Principal, reqID string, r policy.Rule) (any, error) {
	return s.write(ctx, p, reqID, "put_policy", r, func(w *W) (any, error) {
		if err := w.admin("policies.put"); err != nil {
			return nil, err
		}
		if r.Name == "" || r.Subject == "" || r.Action == "" || (r.Effect != "allow" && r.Effect != "deny") {
			return nil, errInvalid("policy needs name, subject, action and effect allow|deny")
		}
		sel, _ := json.Marshal(r.Selector)
		if r.ID == "" {
			err := w.row(`INSERT INTO policies(name,subject,action,selector,effect,priority,enabled) VALUES($1,$2,$3,$4,$5,$6,$7)
				RETURNING id::text`, r.Name, r.Subject, r.Action, sel, r.Effect, r.Priority, r.Enabled).Scan(&r.ID)
			if err != nil {
				return nil, errInvalid("policy: %v", err)
			}
			r.Version = 1
		} else {
			tag, err := w.exec(`UPDATE policies SET name=$2,subject=$3,action=$4,selector=$5,effect=$6,priority=$7,enabled=$8,
				version=version+1 WHERE id=$1::uuid AND version=$9`, r.ID, r.Name, r.Subject, r.Action, sel, r.Effect, r.Priority, r.Enabled, r.Version)
			if err != nil {
				return nil, errInvalid("policy: %v", err)
			}
			if tag.RowsAffected() == 0 {
				return nil, errVersion(nil)
			}
			r.Version++
		}
		return r, w.Event("admin.policy.put", nil, nil, r)
	})
}

// PutWorkflow validates and stores a workflow.
func (s *Store) PutWorkflow(ctx context.Context, p *Principal, reqID string, wf Workflow) (any, error) {
	return s.write(ctx, p, reqID, "put_workflow", wf, func(w *W) (any, error) {
		if err := w.admin("workflows.put"); err != nil {
			return nil, err
		}
		if err := wf.Validate(); err != nil {
			return nil, err
		}
		st, _ := json.Marshal(wf.States)
		tr, _ := json.Marshal(wf.Transitions)
		la, _ := json.Marshal(wf.Lanes)
		err := w.versioned("workflows", wf.ID, wf.Version, func() error {
			_, e := w.exec(`INSERT INTO workflows(id,name,states,transitions,lanes,initial_state,attention_state) VALUES($1,$2,$3,$4,$5,$6,$7)`,
				wf.ID, wf.Name, st, tr, la, wf.InitialState, wf.AttentionState)
			return e
		}, func() error {
			_, e := w.exec(`UPDATE workflows SET name=$2,states=$3,transitions=$4,lanes=$5,initial_state=$6,attention_state=$7,
				version=version+1 WHERE id=$1`, wf.ID, wf.Name, st, tr, la, wf.InitialState, wf.AttentionState)
			return e
		})
		if err != nil {
			return nil, err
		}
		return wf, w.Event("admin.workflow.put", nil, nil, map[string]any{"id": wf.ID, "states": len(wf.States), "lanes": len(wf.Lanes)})
	})
}

// Project binds tasks to a workflow.
type Project struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	WorkflowID string `json:"workflow_id"`
	Version    int64  `json:"version"`
}

// PutProject creates or updates a project.
func (s *Store) PutProject(ctx context.Context, p *Principal, reqID string, pr Project) (any, error) {
	return s.write(ctx, p, reqID, "put_project", pr, func(w *W) (any, error) {
		if err := w.admin("projects.put"); err != nil {
			return nil, err
		}
		err := w.versioned("projects", pr.ID, pr.Version, func() error {
			_, e := w.exec(`INSERT INTO projects(id,name,workflow_id) VALUES($1,$2,$3)`, pr.ID, pr.Name, pr.WorkflowID)
			return e
		}, func() error {
			_, e := w.exec(`UPDATE projects SET name=$2,workflow_id=$3,version=version+1 WHERE id=$1`, pr.ID, pr.Name, pr.WorkflowID)
			return e
		})
		if err != nil {
			if _, ok := AsError(err); ok {
				return nil, err
			}
			return nil, errInvalid("project: %v", err)
		}
		return pr, w.Event("admin.project.put", nil, nil, pr)
	})
}
