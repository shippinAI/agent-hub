package guides

import (
	"context"
	"time"

	"agenthub/internal/guidekit"
	"agenthub/internal/store"
)

// StepReq reports (or asks the hub to perform) the current step of a run.
type StepReq struct {
	RunID   string `json:"run_id"`
	StepID  string `json:"step_id"`
	Outcome string `json:"outcome,omitempty"`
	Value   string `json:"value,omitempty"`
	Note    string `json:"note,omitempty"`
}

// Step completes the current step. An empty outcome on an agent step makes the hub run the step's op on the
// machine through the access flow; on request_access it files or re-checks the request.
func (s *Service) Step(ctx context.Context, p *store.Principal, reqID string, q StepReq) (any, error) {
	r, err := s.GetRun(ctx, q.RunID)
	if err != nil {
		return nil, err
	}
	if r.Status != "running" && !(r.Status == "paused" && q.Outcome == "resume") {
		return nil, store.Fail(409, "run_not_running", "run is %s", r.Status)
	}
	if r.Current == nil || *r.Current != q.StepID {
		return nil, &store.Error{Status: 409, Code: "wrong_step", Message: "that is not the current step", Current: r.Current}
	}
	set, _ := s.St.LoadSettings(ctx)
	for _, re := range append(SecretRes(set), guidekit.SecretNote) {
		if re.MatchString(q.Note + " " + q.Value) {
			return nil, store.Fail(422, "secret_in_note", "notes and values must not carry secrets")
		}
	}
	st := guidekit.Find(r.Steps, q.StepID)
	if (st.Kind == "operator_action" || st.Kind == "confirm" || q.Outcome == "resume") && !p.IsHuman {
		return nil, store.Fail(403, "humans_only", "only the operator can complete %s steps", st.Kind)
	}
	var result any
	if q.Outcome == "" {
		hc := Ctx{Context: ctx, P: p, ReqID: reqID}
		switch {
		case st.Kind == "request_access" && !r.DryRun && s.Hooks != nil:
			status, ref, err := s.Hooks.Request(hc, r, *st)
			if err != nil {
				return nil, err
			}
			if status == "pending" {
				return map[string]any{"run_id": r.ID, "pending": true, "request": ref}, nil
			}
			q.Outcome, result = map[bool]string{true: "ok", false: "fail"}[status == "approved"], ref
		case st.Op != "" && !r.DryRun && s.Hooks != nil && r.MachineID != nil:
			out, res, err := s.Hooks.Exec(hc, *r.MachineID, *st)
			if err != nil {
				return nil, err
			}
			q.Outcome, result = out, res
		default:
			q.Outcome = "ok"
		}
	}
	return s.record(ctx, p, reqID, r, st, q, result)
}

func (s *Service) record(ctx context.Context, p *store.Principal, reqID string, r Run, st *guidekit.Step, q StepReq, result any) (any, error) {
	return s.St.Write(ctx, p, reqID, "guide_step", q, func(w *store.W) (any, error) {
		var cur *string
		if err := w.Row(`SELECT current_step FROM guide_runs WHERE id::text=$1 FOR UPDATE`, r.ID).Scan(&cur); err != nil || cur == nil || *cur != q.StepID {
			return nil, store.Fail(409, "wrong_step", "the run moved on")
		}
		c := r.ctx()
		res := guidekit.Result{Next: q.StepID}
		if q.Outcome == "resume" {
			r.Retries[q.StepID] = 0
		} else {
			if st.SaveAs != "" {
				c.Vars[st.SaveAs] = firstNonEmpty(q.Value, q.Outcome)
			}
			var err error
			if res, err = guidekit.Advance(r.Steps, q.StepID, q.Outcome, r.Retries[q.StepID], c); err != nil {
				return nil, store.ErrInvalid("%v", err)
			}
			if res.Retry > 0 {
				r.Retries[q.StepID] = res.Retry
			}
		}
		status := map[bool]string{true: "paused", false: "running"}[res.Paused]
		if res.Done {
			status = "done"
		}
		entry := map[string]any{"step": q.StepID, "outcome": q.Outcome, "by": w.P.ID, "at": time.Now().UTC(), "note": q.Note, "result": result}
		if _, err := w.Exec(`UPDATE guide_runs SET current_step=nullif($2,''), status=$3, vars=$4, retries=$5, step_log=step_log||jsonb_build_array($6::jsonb),
			ended_at=CASE WHEN $3='done' THEN now() END WHERE id::text=$1`, r.ID, res.Next, status, c.Vars, r.Retries, entry); err != nil {
			return nil, err
		}
		out := map[string]any{"run_id": r.ID, "outcome": q.Outcome, "next": res.Next, "status": status, "result": result}
		return out, w.Event("guide.step", r.TaskID, nil, map[string]any{"run": r.ID, "step": q.StepID, "outcome": q.Outcome, "status": status})
	})
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
