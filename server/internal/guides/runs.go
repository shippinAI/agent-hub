package guides

import (
	"context"
	"encoding/json"
	"time"

	"agenthub/internal/guidekit"
	"agenthub/internal/store"
)

// Run is a guide run with the steps of the version it pinned.
type Run struct {
	ID          string          `json:"id"`
	GuideID     string          `json:"guide_id"`
	VersionID   string          `json:"guide_version_id"`
	Semver      string          `json:"semver"`
	ParentRunID *string         `json:"parent_run_id"`
	MachineID   *string         `json:"machine_id"`
	TaskID      *int64          `json:"task_id"`
	StartedBy   string          `json:"started_by"`
	Agent       *string         `json:"agent_principal"`
	Vars        map[string]any  `json:"vars"`
	StepLog     json.RawMessage `json:"step_log"`
	Current     *string         `json:"current_step"`
	Retries     map[string]int  `json:"retries"`
	DryRun      bool            `json:"dry_run"`
	Status      string          `json:"status"`
	StartedAt   time.Time       `json:"started_at"`
	EndedAt     *time.Time      `json:"ended_at"`
	Steps       []guidekit.Step `json:"-"`
	Profile     map[string]any  `json:"profile"`
}

const runCols = `r.id::text,v.guide_id,v.id::text,v.semver,r.parent_run_id::text,r.machine_id,r.task_id,r.started_by,r.agent_principal,r.vars,
	r.step_log,r.current_step,r.retries,r.dry_run,r.status,r.started_at,r.ended_at,v.steps,
	coalesce((SELECT jsonb_build_object('os',m.os,'arch',m.arch,'kind',m.kind,'display_server',m.display_server) FROM machines m WHERE m.id=r.machine_id),'{}')`

func scanRun(r interface{ Scan(...any) error }) (Run, error) {
	var x Run
	var steps []byte
	err := r.Scan(&x.ID, &x.GuideID, &x.VersionID, &x.Semver, &x.ParentRunID, &x.MachineID, &x.TaskID, &x.StartedBy, &x.Agent, &x.Vars,
		&x.StepLog, &x.Current, &x.Retries, &x.DryRun, &x.Status, &x.StartedAt, &x.EndedAt, &steps, &x.Profile)
	if err == nil {
		x.Steps, err = guidekit.Parse(steps)
	}
	if p, ok := x.Vars["_profile"].(map[string]any); ok && x.MachineID == nil {
		x.Profile = p
	}
	return x, err
}

// GetRun loads one run.
func (s *Service) GetRun(ctx context.Context, id string) (Run, error) {
	r, err := scanRun(s.St.DB.QueryRow(ctx, `SELECT `+runCols+` FROM guide_runs r JOIN guide_versions v ON v.id=r.guide_version_id WHERE r.id::text=$1`, id))
	if err != nil {
		return r, store.ErrNotFound("guide run " + id)
	}
	return r, nil
}

// ListRuns lists recent runs, optionally for one machine.
func (s *Service) ListRuns(ctx context.Context, machineID string, limit int) ([]Run, error) {
	rows, err := s.St.DB.Query(ctx, `SELECT `+runCols+` FROM guide_runs r JOIN guide_versions v ON v.id=r.guide_version_id
		WHERE $1='' OR r.machine_id=$1 ORDER BY r.started_at DESC LIMIT $2`, machineID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Run{}
	for rows.Next() {
		r, err := scanRun(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (r Run) ctx() guidekit.Ctx {
	vars := map[string]any{}
	for k, v := range r.Vars {
		vars[k] = v
	}
	return guidekit.Ctx{Profile: r.Profile, Vars: vars}
}

// View is what a reader sees: the run plus the current step rendered for the machine's OS.
func (s *Service) View(ctx context.Context, r Run) map[string]any {
	os, _ := r.Profile["os"].(string)
	vals := map[string]any{"hub_url": s.HubURL}
	for k, v := range r.Vars {
		vals["vars."+k] = v
	}
	out := map[string]any{"run": r}
	titles := []map[string]string{}
	for _, st := range r.Steps {
		titles = append(titles, map[string]string{"id": st.ID, "title": st.Title, "kind": st.Kind})
	}
	out["outline"] = titles
	if r.Current != nil {
		if st := guidekit.Find(r.Steps, *r.Current); st != nil {
			out["step"] = guidekit.Render(*st, os, vals, s.Commands(ctx))
		}
	}
	return out
}
