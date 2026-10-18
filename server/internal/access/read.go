package access

import (
	"context"
	"time"

	"agenthub/internal/store"
)

// Requests lists access requests as JSON rows (status "" = all), newest first, with requester details.
func (s *Service) Requests(ctx context.Context, status, machine string) ([]map[string]any, error) {
	return s.rows(ctx, `SELECT to_jsonb(r) || jsonb_build_object('machine_label', m.label, 'requester_name', p.display_name,
		'task_title', (SELECT title FROM tasks t WHERE t.id=r.task_id)) FROM access_requests r JOIN machines m ON m.id=r.machine_id
		JOIN principals p ON p.id=r.requester WHERE ($1='' OR r.status=$1) AND ($2='' OR r.machine_id=$2) ORDER BY r.created_at DESC LIMIT 200`, status, machine)
}

// Grants lists grants (live only when live), optionally for one machine or principal.
func (s *Service) Grants(ctx context.Context, live bool, machine, principal string) ([]map[string]any, error) {
	return s.rows(ctx, `SELECT to_jsonb(g) || jsonb_build_object('machine_label', m.label, 'live', g.ended_at IS NULL,
		'high_risk', g.sandbox_level='unsandboxed' OR g.full_access OR g.capability LIKE 'gui.%' OR g.risk_snapshot @> '[{"severity":"high"}]')
		FROM grants g JOIN machines m ON m.id=g.machine_id
		WHERE (NOT $1 OR g.ended_at IS NULL) AND ($2='' OR g.machine_id=$2) AND ($3='' OR g.principal_id=$3) ORDER BY g.approved_at DESC LIMIT 500`,
		live, machine, principal)
}

func (s *Service) rows(ctx context.Context, sql string, args ...any) ([]map[string]any, error) {
	rows, err := s.St.DB.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var m map[string]any
		if err := rows.Scan(&m); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// Status is a request's (or group's) decision as the agent sees it.
type Status struct {
	RequestID string         `json:"request_id"`
	Status    string         `json:"status"`
	GrantID   *string        `json:"grant_id"`
	Granted   map[string]any `json:"granted,omitempty"`
	Note      *string        `json:"note"`
}

// Lookup returns the decision for a request id or every request of a group id.
func (s *Service) Lookup(ctx context.Context, id string) ([]Status, error) {
	rows, err := s.St.DB.Query(ctx, `SELECT r.id::text, CASE WHEN r.status='pending' AND r.expires_at<=now() THEN 'expired' ELSE r.status END,
		r.grant_id::text, r.decision_note, CASE WHEN g.id IS NULL THEN NULL ELSE jsonb_build_object('capability',g.capability,'selector',g.selector,
		'sandbox_level',g.sandbox_level,'duration',g.duration,'uses_max',g.uses_max,'expires_at',g.expires_at) END
		FROM access_requests r LEFT JOIN grants g ON g.id=r.grant_id WHERE r.id::text=$1 OR r.group_id::text=$1 ORDER BY r.created_at`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Status{}
	for rows.Next() {
		var st Status
		if err := rows.Scan(&st.RequestID, &st.Status, &st.GrantID, &st.Note, &st.Granted); err != nil {
			return nil, err
		}
		out = append(out, st)
	}
	if len(out) == 0 {
		return nil, store.ErrNotFound("access request " + id)
	}
	return out, rows.Err()
}

// Summary folds a group into one status: pending while any item is pending, approved if any was granted.
func Summary(sts []Status) string {
	granted, pending := false, false
	for _, s := range sts {
		pending = pending || s.Status == "pending"
		granted = granted || s.Status == "approved" || s.Status == "narrowed"
	}
	switch {
	case pending:
		return "pending"
	case granted:
		return "approved"
	}
	return sts[0].Status
}

// Wait long-polls a request or group until it is decided or timeout (capped at 25 s) passes.
func (s *Service) Wait(ctx context.Context, id string, timeout time.Duration) ([]Status, error) {
	timeout = min(max(timeout, time.Second), 25*time.Second)
	deadline := time.Now().Add(timeout)
	for {
		sts, err := s.Lookup(ctx, id)
		if err != nil || Summary(sts) != "pending" || time.Now().After(deadline) {
			return sts, err
		}
		select {
		case <-ctx.Done():
			return sts, nil
		case <-time.After(400 * time.Millisecond):
		}
	}
}
