package access

import (
	"context"
	"encoding/json"

	"agenthub/internal/store"
)

// Open lists the human's unacknowledged alerts with what each grant can do.
func (s *Service) Open(ctx context.Context, human string) ([]Alert, error) {
	rows, err := s.St.DB.Query(ctx, `SELECT a.id::text, a.grant_id::text, a.reason, a.shown_at, g.machine_id, m.label, g.principal_id,
		g.capability||' '||g.selector::text, g.sandbox_level, g.full_access, g.approved_at, g.risk_snapshot
		FROM resume_acks a JOIN grants g ON g.id=a.grant_id JOIN machines m ON m.id=g.machine_id
		WHERE a.principal_id=$1 AND a.acknowledged_at IS NULL AND g.ended_at IS NULL ORDER BY a.shown_at`, human)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Alert{}
	for rows.Next() {
		var a Alert
		var risks []byte
		if err := rows.Scan(&a.ID, &a.GrantID, &a.Reason, &a.ShownAt, &a.MachineID, &a.Machine, &a.Principal, &a.Scope, &a.Level, &a.Full, &a.Since, &risks); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(risks, &a.Risks)
		out = append(out, a)
	}
	return out, rows.Err()
}

// Ack answers an alert: continue, revoke (the grant) or pause (the machine).
func (s *Service) Ack(ctx context.Context, p *store.Principal, reqID, id, action string) (any, error) {
	var ended []Ended
	out, err := s.St.Write(ctx, p, reqID, "resume_ack", id+action, func(w *store.W) (any, error) {
		if !w.P.IsHuman {
			return nil, store.Fail(403, "humans_only", "resume alerts are for humans")
		}
		if action != "continue" && action != "revoke" && action != "pause" {
			return nil, store.ErrInvalid("action must be continue, revoke or pause")
		}
		var grant, machine string
		err := w.Row(`UPDATE resume_acks a SET acknowledged_at=now(), action=$3 FROM grants g WHERE a.id::text=$1 AND a.principal_id=$2
			AND a.acknowledged_at IS NULL AND g.id=a.grant_id RETURNING a.grant_id::text, g.machine_id`, id, w.P.ID, action).Scan(&grant, &machine)
		if err != nil {
			return nil, store.Fail(409, "not_open", "alert is not open for you")
		}
		switch action {
		case "revoke":
			if err := w.Human("grant.revoke"); err != nil {
				return nil, err
			}
			if ended, err = endGrants(w, "revoked", w.P.ID, `id::text = $1`, grant); err != nil {
				return nil, err
			}
		case "pause":
			if err := w.Human("machine.pause"); err != nil {
				return nil, err
			}
			if _, err := w.Exec(`UPDATE machines SET disabled=true, version=version+1 WHERE id=$1`, machine); err != nil {
				return nil, err
			}
			if err := w.Event("machine.paused", nil, nil, map[string]any{"machine": machine, "via": "resume_alert"}); err != nil {
				return nil, err
			}
		}
		// Other open alerts for the same grant and human are answered together.
		if _, err := w.Exec(`UPDATE resume_acks SET acknowledged_at=now(), action=$3 WHERE grant_id::text=$1 AND principal_id=$2 AND acknowledged_at IS NULL`,
			grant, w.P.ID, action); err != nil {
			return nil, err
		}
		return map[string]any{"alert": id, "action": action, "grant_id": grant},
			w.Event("resume.acknowledged", nil, nil, map[string]any{"alert": id, "grant": grant, "action": action})
	})
	if err == nil {
		s.notify(ctx, ended)
	}
	return out, err
}
