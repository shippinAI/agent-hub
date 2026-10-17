package store

import "fmt"

func (w *W) checkWIP(st Settings) error {
	limit := st.Int("wip_limit_default", 1)
	if w.P.WIPLimit != nil {
		limit = *w.P.WIPLimit
	}
	var n int
	if err := w.row(`SELECT count(*) FROM tasks WHERE holder=$1 AND lease_expires_at > clock_timestamp()`, w.P.ID).Scan(&n); err != nil {
		return err
	}
	if limit > 0 && n >= limit {
		return fail(409, "wip_limit", "you already hold %d task(s); WIP limit is %d", n, limit)
	}
	return nil
}

func (w *W) ttl(st Settings, asked int) int {
	ttl := st.Int("lease_ttl_seconds", 900)
	if asked > 0 {
		ttl = asked
	}
	if lo := st.Int("lease_min_ttl_seconds", 30); ttl < lo {
		ttl = lo
	}
	if hi := st.Int("lease_max_ttl_seconds", 3600); ttl > hi {
		ttl = hi
	}
	return ttl
}

func (w *W) takeLease(t Task, lane Lane, st Settings, asked int) (Lease, error) {
	ttl := w.ttl(st, asked)
	_, err := w.exec(`UPDATE tasks SET holder=$2, lease_id=gen_random_uuid(), lease_ttl_seconds=$3::int,
		lease_expires_at=clock_timestamp()+make_interval(secs=>$3::int), heartbeat_at=clock_timestamp(),
		fence=nextval('fence_seq'), state=$4, version=version+1, updated_at=clock_timestamp() WHERE id=$1`,
		t.ID, w.P.ID, ttl, lane.Active)
	if err != nil {
		return Lease{}, err
	}
	n, err := getTask(w.ctx, w.tx, t.ID, false)
	if err != nil {
		return Lease{}, err
	}
	err = w.Event("task.claimed", &n.ID, &n.Fence, map[string]any{"from": t.State, "to": n.State,
		"ttl_seconds": ttl, "lease_expires_at": n.LeaseExpiresAt})
	return Lease{Task: n, Fence: n.Fence, LeaseID: *n.LeaseID, LeaseExpiresAt: *n.LeaseExpiresAt,
		HeartbeatEvery: beatEvery(st, ttl), Claimed: true}, err
}

// expire ends a lapsed lease: back to the lane's release state, or attention after too many expiries.
func (w *W) expire(t Task, wf Workflow, st Settings) (Task, error) {
	to := wf.InitialState
	if lane, ok := wf.laneByActive(t.State); ok {
		to = lane.ReleaseTo
	}
	count := t.ExpiryCount + 1
	if max := st.Int("max_lease_expiries", 3); max > 0 && count >= max {
		to = wf.AttentionState
	}
	if _, err := w.exec(`UPDATE tasks SET expiry_count=$2 WHERE id=$1`, t.ID, count); err != nil {
		return t, err
	}
	return w.clearLease(t, to, "lease.expired", map[string]any{"previous_holder": t.Holder, "expiry_count": count})
}

// clearLease drops the claim, moves the task and logs the event; the fence is never reused.
func (w *W) clearLease(t Task, to, action string, data map[string]any) (Task, error) {
	_, err := w.exec(`UPDATE tasks SET holder=NULL, lease_id=NULL, lease_ttl_seconds=NULL, lease_expires_at=NULL,
		state=$2, version=version+1, updated_at=clock_timestamp() WHERE id=$1`, t.ID, to)
	if err != nil {
		return t, err
	}
	data["from"], data["to"] = t.State, to
	if err := w.Event(action, &t.ID, &t.Fence, data); err != nil {
		return t, err
	}
	return getTask(w.ctx, w.tx, t.ID, false)
}

// held locks a task and checks the caller still owns its lease under this fence.
func (w *W) held(taskID, fence int64) (Task, Workflow, Settings, error) {
	t, err := getTask(w.ctx, w.tx, taskID, true)
	if err != nil {
		return t, Workflow{}, nil, err
	}
	switch {
	case fence != t.Fence:
		e := fail(409, "stale_fence", "fence %d is not current; your lease was lost", fence)
		e.Current = map[string]any{"fence": t.Fence, "holder": t.Holder}
		return t, Workflow{}, nil, e
	case t.Holder == nil || *t.Holder != w.P.ID:
		return t, Workflow{}, nil, fail(409, "not_holder", "you do not hold task %d", taskID)
	case t.LeaseExpired:
		return t, Workflow{}, nil, fail(409, "lease_expired", "lease expired at %s; claim again", t.LeaseExpiresAt)
	}
	wf, err := loadWorkflowFor(w.ctx, w.tx, t.Project)
	if err != nil {
		return t, wf, nil, err
	}
	st, err := loadSettings(w.ctx, w.tx)
	return t, wf, st, err
}

func needFence(f *int64) error {
	if f == nil {
		return fmt.Errorf("%w", fail(409, "stale_fence", "fence is required for a held task"))
	}
	return nil
}
