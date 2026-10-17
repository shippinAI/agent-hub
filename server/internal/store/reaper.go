package store

import (
	"context"
	"log"
	"time"
)

// System is the actor recorded for reaper actions.
var System = &Principal{ID: "system", DisplayName: "hub reaper", Kind: "system", Enabled: true}

// ReapOnce expires every lapsed lease and prunes old idempotency keys. Returns how many leases expired.
func (s *Store) ReapOnce(ctx context.Context) (int, error) {
	out, err := s.write(ctx, System, "", "reap", nil, func(w *W) (any, error) {
		st, err := loadSettings(ctx, w.tx)
		if err != nil {
			return 0, err
		}
		rows, err := w.tx.Query(ctx, `SELECT id FROM tasks WHERE holder IS NOT NULL AND lease_expires_at < clock_timestamp()
			ORDER BY id FOR UPDATE SKIP LOCKED`)
		if err != nil {
			return 0, err
		}
		var ids []int64
		for rows.Next() {
			var id int64
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return 0, err
			}
			ids = append(ids, id)
		}
		rows.Close()
		for _, id := range ids {
			t, err := getTask(ctx, w.tx, id, false)
			if err != nil {
				return 0, err
			}
			wf, err := loadWorkflowFor(ctx, w.tx, t.Project)
			if err != nil {
				return 0, err
			}
			if _, err := w.expire(t, wf, st); err != nil {
				return 0, err
			}
		}
		_, err = w.exec(`DELETE FROM idempotency_keys WHERE created_at < clock_timestamp() - make_interval(hours=>$1)`,
			st.Int("idempotency_ttl_hours", 24))
		return len(ids), err
	})
	n, _ := out.(int)
	return n, err
}

// RunReaper loops until ctx ends; the interval is re-read from settings every pass.
func (s *Store) RunReaper(ctx context.Context) {
	for {
		if n, err := s.ReapOnce(ctx); err != nil {
			log.Printf("reaper: %v", err)
		} else if n > 0 {
			log.Printf("reaper: expired %d lease(s)", n)
		}
		st, _ := loadSettings(ctx, s.DB)
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Duration(st.Int("reaper_interval_seconds", 5)) * time.Second):
		}
	}
}
