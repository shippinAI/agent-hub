package access

import (
	"context"
	"time"

	"agenthub/internal/store"
)

// ReapOnce ends grants whose time, uses, task lease or guide run is over, expires stale requests, and
// expires proc/GUI leases. Ended grants are pushed to the bridge after commit.
func (s *Service) ReapOnce(ctx context.Context) (int, error) {
	var ended []Ended
	_, err := s.St.Write(ctx, store.System, "", "access_reap", nil, func(w *store.W) (any, error) {
		ended = nil
		for reason, where := range map[string]string{
			"expired":       `expires_at IS NOT NULL AND expires_at <= now()`,
			"used_up":       `uses_max IS NOT NULL AND uses >= uses_max`,
			"task_ended":    `task_id IS NOT NULL AND task_lease_id IS DISTINCT FROM (SELECT lease_id FROM tasks t WHERE t.id=grants.task_id)`,
			"guide_ended":   `guide_run_id IS NOT NULL AND task_id IS NULL AND EXISTS (SELECT 1 FROM guide_runs r WHERE r.id=grants.guide_run_id AND r.status IN ('done','failed','abandoned'))`,
			"principal_off": `EXISTS (SELECT 1 FROM principals p WHERE p.id=grants.principal_id AND NOT p.enabled)`,
		} {
			e, err := endGrants(w, reason, "", where)
			if err != nil {
				return nil, err
			}
			ended = append(ended, e...)
		}
		rows, err := w.Query(`UPDATE access_requests SET status='expired' WHERE status='pending' AND expires_at <= now() RETURNING id::text`)
		if err != nil {
			return nil, err
		}
		var exp []string
		for rows.Next() {
			var id string
			_ = rows.Scan(&id)
			exp = append(exp, id)
		}
		rows.Close()
		for _, id := range exp {
			if err := w.Event("access.expired", nil, nil, map[string]any{"request": id}); err != nil {
				return nil, err
			}
		}
		if _, err := w.Exec(`UPDATE proc_leases SET state='lost', ended_at=now() WHERE state='running' AND lease_expires_at <= now()`); err != nil {
			return nil, err
		}
		_, err = w.Exec(`UPDATE gui_sessions SET state='ended', ended_at=now(), end_reason='lease_expired' WHERE state<>'ended' AND lease_expires_at <= now()`)
		return nil, err
	})
	if err == nil {
		s.notify(ctx, ended)
	}
	return len(ended), err
}

// RunReaper reaps every interval until ctx ends.
func (s *Service) RunReaper(ctx context.Context, every time.Duration) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			_, _ = s.ReapOnce(ctx)
		}
	}
}
