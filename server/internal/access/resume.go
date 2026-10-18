package access

import (
	"context"
	"time"

	"agenthub/internal/store"
)

// risky selects the live grants that need a resume alert (01 invariant 7; cannot be turned off by data).
const risky = `g.ended_at IS NULL AND (g.sandbox_level='unsandboxed' OR g.full_access)`

// Alert is one resume alert waiting for the human.
type Alert struct {
	ID        string    `json:"id"`
	GrantID   string    `json:"grant_id"`
	Reason    string    `json:"reason"`
	ShownAt   time.Time `json:"shown_at"`
	MachineID string    `json:"machine_id"`
	Machine   string    `json:"machine_label"`
	Principal string    `json:"principal_id"`
	Scope     string    `json:"scope"`
	Level     string    `json:"sandbox_level"`
	Full      bool      `json:"full_access"`
	Since     time.Time `json:"approved_at"`
	Risks     []Risk    `json:"risks"`
}

// IdleMinutes is resume_alert_idle_minutes with its code clamp (1-60).
func IdleMinutes(set store.Settings) int { return min(60, max(1, set.Int("resume_alert_idle_minutes", 15))) }

// Check is called on every UI resume (sign-in, load, reload, return after idle): it records an alert for
// each risky live grant that the human has not already got an open alert for, and returns all open ones.
func (s *Service) Check(ctx context.Context, p *store.Principal, reqID, uiSession, reason string) ([]Alert, error) {
	if !p.IsHuman {
		return nil, store.Fail(403, "humans_only", "resume alerts are for humans")
	}
	_, err := s.St.Write(ctx, p, reqID, "resume_check", uiSession+reason, func(w *store.W) (any, error) {
		return nil, shown(w, w.P.ID, "", uiSession, reason)
	})
	if err != nil {
		return nil, err
	}
	return s.Open(ctx, p.ID)
}

// AgentResumed raises alerts for the approving humans when an agent session resumes under a risky grant
// (new MCP session, reconnect, or task claim). The agent's call itself proceeds while the grant is live.
func (s *Service) AgentResumed(ctx context.Context, agentID, reason string) error {
	var n int
	if s.St.DB.QueryRow(ctx, `SELECT count(*) FROM grants g WHERE `+risky+` AND g.principal_id=$1`, agentID).Scan(&n); n == 0 {
		return nil
	}
	_, err := s.St.Write(ctx, store.System, "", "resume_agent", nil, func(w *store.W) (any, error) {
		return nil, shown(w, "", agentID, "", reason)
	})
	return err
}

func shown(w *store.W, human, agent, uiSession, reason string) error {
	rows, err := w.Query(`INSERT INTO resume_acks(grant_id,principal_id,ui_session_id,reason)
		SELECT g.id, coalesce(nullif($1,''), g.approved_by), nullif($2,''), $3 FROM grants g WHERE `+risky+` AND ($4='' OR g.principal_id=$4)
		AND NOT EXISTS (SELECT 1 FROM resume_acks a WHERE a.grant_id=g.id AND a.principal_id=coalesce(nullif($1,''), g.approved_by) AND a.acknowledged_at IS NULL)
		RETURNING id::text, grant_id::text, principal_id`, human, uiSession, reason, agent)
	if err != nil {
		return err
	}
	type ev struct{ id, grant, who string }
	var evs []ev
	for rows.Next() {
		var e ev
		_ = rows.Scan(&e.id, &e.grant, &e.who)
		evs = append(evs, e)
	}
	rows.Close()
	for _, e := range evs {
		if err := w.Event("resume.alert_shown", nil, nil, map[string]any{"alert": e.id, "grant": e.grant, "human": e.who, "reason": reason}); err != nil {
			return err
		}
	}
	return nil
}
