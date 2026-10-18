package guides

import (
	"context"

	"agenthub/internal/guidekit"
	"agenthub/internal/store"
)

// StartReq starts a run. Profile is only used for dry runs without a machine.
type StartReq struct {
	Guide     string         `json:"guide"`
	MachineID string         `json:"machine_id"`
	TaskID    *int64         `json:"task_id"`
	Vars      map[string]any `json:"vars"`
	DryRun    bool           `json:"dry_run"`
	Profile   map[string]any `json:"profile"`
	Parent    string         `json:"parent_run_id"`
}

// Start resolves the effective guide (tenant override first), pins its current version and settles on
// the first step that needs someone.
func (s *Service) Start(ctx context.Context, p *store.Principal, reqID string, r StartReq) (any, error) {
	g, err := s.Effective(ctx, r.Guide)
	if err != nil {
		return nil, err
	}
	if !g.Enabled || g.CurrentID == nil {
		return nil, store.Fail(409, "guide_unavailable", "guide %s has no enabled published version", r.Guide)
	}
	steps, err := guidekit.Parse(g.Steps)
	if err != nil || len(steps) == 0 {
		return nil, store.ErrInvalid("guide %s has no steps", r.Guide)
	}
	if r.MachineID == "" && !r.DryRun {
		return nil, store.ErrInvalid("machine_id is required unless dry_run")
	}
	return s.St.Write(ctx, p, reqID, "guide_run", r, func(w *store.W) (any, error) {
		if err := w.Can("guide.run"); err != nil {
			return nil, err
		}
		vars := map[string]any{}
		for k, v := range r.Vars {
			vars[k] = v
		}
		profile := r.Profile
		if r.MachineID != "" {
			var os, arch, kind string
			var ds *string
			if err := w.Row(`SELECT os,arch,kind,display_server FROM machines WHERE id=$1 AND NOT disabled`, r.MachineID).Scan(&os, &arch, &kind, &ds); err != nil {
				return nil, store.ErrNotFound("machine " + r.MachineID)
			}
			profile = map[string]any{"os": os, "arch": arch, "kind": kind, "display_server": deref(ds)}
		} else {
			vars["_profile"] = profile
		}
		cur := guidekit.Settle(steps, steps[0].ID, guidekit.Ctx{Profile: profile, Vars: vars})
		var agent *string
		if !w.P.IsHuman {
			agent = &w.P.ID
		}
		var id string
		err := w.Row(`INSERT INTO guide_runs(guide_version_id,parent_run_id,machine_id,task_id,started_by,agent_principal,vars,current_step,dry_run,status)
			VALUES($1::uuid,nullif($2,'')::uuid,nullif($3,''),$4,$5,$6,$7,nullif($8,''),$9,CASE WHEN $8='' THEN 'done' ELSE 'running' END) RETURNING id::text`,
			*g.CurrentID, r.Parent, r.MachineID, r.TaskID, w.P.ID, agent, vars, cur, r.DryRun).Scan(&id)
		if err != nil {
			return nil, store.ErrInvalid("%v", err)
		}
		return map[string]any{"run_id": id, "guide_id": g.ID, "semver": deref(g.Semver), "current_step": cur},
			w.Event("guide.run_started", r.TaskID, nil, map[string]any{"run": id, "guide": g.ID, "machine": r.MachineID, "dry_run": r.DryRun})
	})
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
