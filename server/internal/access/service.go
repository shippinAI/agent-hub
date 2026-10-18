// Package access is the grant flow: requests, approve/narrow/deny, revoke, the unsandboxed policy, the
// default preset, resume alerts and the grant reaper. Every write goes through store.Write.
package access

import (
	"context"
	"encoding/json"
	"strconv"

	"agenthub/internal/store"
)

// Notifier is told after commit when grants end, so the bridge can stop calls under them at once.
type Notifier interface {
	GrantsEnded(ctx context.Context, ended []Ended)
}

// Ended is one grant that just ended.
type Ended struct {
	GrantID   string `json:"grant_id"`
	MachineID string `json:"machine_id"`
	Reason    string `json:"reason"`
}

// Service is the access domain.
type Service struct {
	St     *store.Store
	Notify Notifier
}

func (s *Service) notify(ctx context.Context, e []Ended) {
	if s.Notify != nil && len(e) > 0 {
		s.Notify.GrantsEnded(ctx, e)
	}
}

// livePolicy is the SQL test for an enabled unsandboxed policy covering machine $1 (its own row or tenant-wide).
const livePolicy = `EXISTS (SELECT 1 FROM sandbox_policies sp WHERE sp.unsandboxed_allowed AND (sp.expires_at IS NULL OR sp.expires_at > now())
	AND (sp.machine_id = $1 OR sp.machine_id IS NULL))`

func policyLive(w *store.W, machineID string) (bool, error) {
	var ok bool
	err := w.Row(`SELECT `+livePolicy, machineID).Scan(&ok)
	return ok, err
}

func jsonb(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

// endGrants ends the grants selected by where (args follow) and returns them; the caller notifies after commit.
func endGrants(w *store.W, reason, by string, where string, args ...any) ([]Ended, error) {
	n := len(args)
	q := `UPDATE grants SET ended_at=now(), end_reason=$` + strconv.Itoa(n+1) + `, version=version+1,
		revoked_at=CASE WHEN $` + strconv.Itoa(n+1) + ` IN ('revoked','policy_disabled','machine_removed') THEN now() END,
		revoked_by=nullif($` + strconv.Itoa(n+2) + `,'') WHERE ended_at IS NULL AND (` + where + `) RETURNING id::text, machine_id`
	rows, err := w.Query(q, append(args, reason, by)...)
	if err != nil {
		return nil, err
	}
	var out []Ended
	for rows.Next() {
		var e Ended
		if err := rows.Scan(&e.GrantID, &e.MachineID); err != nil {
			rows.Close()
			return nil, err
		}
		e.Reason = reason
		out = append(out, e)
	}
	rows.Close()
	for _, e := range out {
		if _, err := w.Exec(`UPDATE gui_sessions SET state='ended', ended_at=now(), end_reason='grant_ended' WHERE state<>'ended' AND $1::uuid = ANY(grant_ids)`, e.GrantID); err != nil {
			return nil, err
		}
		if _, err := w.Exec(`UPDATE proc_leases SET state='killed', ended_at=now() WHERE state='running' AND grant_id=$1::uuid`, e.GrantID); err != nil {
			return nil, err
		}
		action := map[bool]string{true: "grant.revoked", false: "grant.expired"}[reason == "revoked"]
		if err := w.Event(action, nil, nil, map[string]any{"grant": e.GrantID, "machine": e.MachineID, "reason": reason}); err != nil {
			return nil, err
		}
	}
	return out, nil
}
