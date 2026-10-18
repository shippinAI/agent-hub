package access

import (
	"context"
	"encoding/json"

	"agenthub/internal/scope"
	"agenthub/internal/store"
)

// Narrow is an approve body; any field left empty keeps what was requested. Nothing may be wider.
type Narrow struct {
	Selector     map[string]any `json:"selector"`
	SandboxLevel string         `json:"sandbox_level"`
	Duration     string         `json:"duration"`
	UsesMax      *int           `json:"uses_max"`
	Note         string         `json:"note"`
}

type pending struct {
	requester, machine, capability, level, dur, status, preset string
	sel                                                        map[string]any
	usesMax                                                    *int
	taskID                                                     *int64
	runID                                                      *string
	risks                                                      []byte
	expired                                                    bool
}

func loadPending(w *store.W, id string) (pending, error) {
	var p pending
	var selRaw, durRaw []byte
	err := w.Row(`SELECT requester,machine_id,capability,sandbox_level,duration,status,coalesce(preset_id,''),selector,uses_max,task_id,
		guide_run_id::text,risk_snapshot,expires_at<now() FROM access_requests WHERE id::text=$1 FOR UPDATE`, id).Scan(&p.requester, &p.machine,
		&p.capability, &p.level, &durRaw, &p.status, &p.preset, &selRaw, &p.usesMax, &p.taskID, &p.runID, &p.risks, &p.expired)
	if err != nil {
		return p, store.ErrNotFound("access request " + id)
	}
	_ = json.Unmarshal(selRaw, &p.sel)
	_ = json.Unmarshal(durRaw, &p.dur)
	if p.status != "pending" || p.expired {
		return p, store.Fail(409, "not_pending", "request is %s", map[bool]string{true: "expired", false: p.status}[p.expired])
	}
	if p.requester == w.P.ID {
		return p, store.Fail(403, "own_request", "nobody approves their own request")
	}
	return p, nil
}

// Approve approves (or narrows) one request and creates its grant.
func (s *Service) Approve(ctx context.Context, p *store.Principal, reqID, id string, n Narrow) (any, error) {
	return s.St.Write(ctx, p, reqID, "access_approve", map[string]any{"id": id, "n": n}, func(w *store.W) (any, error) {
		if err := w.Human("grant.approve"); err != nil {
			return nil, err
		}
		q, err := loadPending(w, id)
		if err != nil {
			return nil, err
		}
		g := Narrow{Selector: q.sel, SandboxLevel: q.level, Duration: q.dur, UsesMax: q.usesMax}
		if n.Selector != nil {
			g.Selector = n.Selector
		}
		g.SandboxLevel, g.Duration = firstOr(n.SandboxLevel, q.level), firstOr(n.Duration, q.dur)
		if n.UsesMax != nil {
			g.UsesMax = n.UsesMax
		}
		if err := wider(q, g); err != nil {
			return nil, &store.Error{Status: 422, Code: "scope_wider_than_request", Message: err.Error()}
		}
		if g.SandboxLevel == "unsandboxed" || (q.capability == "gui.input" && scope.Sel(g.Selector).Any("apps")) {
			if live, _ := policyLive(w, q.machine); !live {
				return nil, store.Fail(403, "policy_denied", "the unsandboxed policy is off for this machine")
			}
		}
		full := false
		if q.preset != "" {
			_ = w.Row(`SELECT full_access FROM grant_presets WHERE id=$1`, q.preset).Scan(&full)
		}
		if g.Duration == "once" {
			one := 1
			g.UsesMax = &one
		}
		var gid string
		err = w.Row(`INSERT INTO grants(principal_id,machine_id,capability,selector,sandbox_level,preset_id,full_access,duration,task_id,task_lease_id,
			guide_run_id,expires_at,uses_max,approved_by,risk_snapshot,request_id)
			VALUES($1,$2,$3,$4,$5,nullif($6,''),$7,$8,$9,(SELECT lease_id FROM tasks WHERE id=$9),$10::uuid,
			CASE WHEN $11 > 0 THEN now()+make_interval(hours => $11) END,$12,$13,$14,$15::uuid) RETURNING id::text`,
			q.requester, q.machine, q.capability, jsonb(g.Selector), g.SandboxLevel, q.preset, full, jsonb(g.Duration), q.taskID, q.runID,
			hours(g.Duration), g.UsesMax, w.P.ID, q.risks, id).Scan(&gid)
		if err != nil {
			return nil, store.ErrInvalid("%v", err)
		}
		status := "approved"
		if n.Selector != nil || n.SandboxLevel != "" || n.Duration != "" || n.UsesMax != nil {
			status = "narrowed"
		}
		if _, err := w.Exec(`UPDATE access_requests SET status=$2, decided_by=$3, decided_at=now(), decision_note=nullif($4,''), grant_id=$5::uuid
			WHERE id::text=$1`, id, status, w.P.ID, n.Note, gid); err != nil {
			return nil, err
		}
		return map[string]any{"request_id": id, "status": status, "grant_id": gid, "granted": g},
			w.Event("access."+status, q.taskID, nil, map[string]any{"request": id, "grant": gid, "machine": q.machine, "principal": q.requester, "capability": q.capability})
	})
}

func wider(q pending, g Narrow) error {
	if !scope.LevelWithin(g.SandboxLevel, q.level) {
		return errf("sandbox level %s is wider than %s", g.SandboxLevel, q.level)
	}
	if !scope.DurationWithin(g.Duration, q.dur) {
		return errf("duration %s is longer than %s", g.Duration, q.dur)
	}
	if q.usesMax != nil && (g.UsesMax == nil || *g.UsesMax > *q.usesMax) {
		return errf("uses_max raised")
	}
	return scope.SelectorWithin(g.Selector, q.sel)
}

// Deny denies one request (the agent sees access_denied and the note).
func (s *Service) Deny(ctx context.Context, p *store.Principal, reqID, id, note string) (any, error) {
	return s.St.Write(ctx, p, reqID, "access_deny", map[string]any{"id": id, "note": note}, func(w *store.W) (any, error) {
		if err := w.Human("grant.approve"); err != nil {
			return nil, err
		}
		q, err := loadPending(w, id)
		if err != nil {
			return nil, err
		}
		if _, err := w.Exec(`UPDATE access_requests SET status='denied', decided_by=$2, decided_at=now(), decision_note=nullif($3,'') WHERE id::text=$1`,
			id, w.P.ID, note); err != nil {
			return nil, err
		}
		return map[string]any{"request_id": id, "status": "denied"},
			w.Event("access.denied", q.taskID, nil, map[string]any{"request": id, "machine": q.machine, "principal": q.requester})
	})
}
