package store

import (
	"context"
	"encoding/json"
	"strings"
)

// Heartbeat renews the caller's lease (same TTL) and optionally records progress.
func (s *Store) Heartbeat(ctx context.Context, p *Principal, reqID string, taskID, fence int64, progress json.RawMessage) (any, error) {
	return s.write(ctx, p, reqID, "heartbeat", []any{taskID, fence, progress}, func(w *W) (any, error) {
		t, _, st, err := w.held(taskID, fence)
		if err != nil {
			return nil, err
		}
		if len(progress) == 0 {
			progress = t.Progress
		}
		_, err = w.exec(`UPDATE tasks SET lease_expires_at=clock_timestamp()+make_interval(secs=>lease_ttl_seconds),
			heartbeat_at=clock_timestamp(), progress=$2 WHERE id=$1`, taskID, progress)
		if err != nil {
			return nil, err
		}
		n, err := getTask(ctx, w.tx, taskID, false)
		if err != nil {
			return nil, err
		}
		if err := w.Event("task.heartbeat", &n.ID, &n.Fence, map[string]any{"lease_expires_at": n.LeaseExpiresAt}); err != nil {
			return nil, err
		}
		return Lease{Task: n, Fence: n.Fence, LeaseID: *n.LeaseID, LeaseExpiresAt: *n.LeaseExpiresAt,
			HeartbeatEvery: st.Int("heartbeat_hint_seconds", 300), Claimed: true}, nil
	})
}

// Release gives the task back to its lane's release state.
func (s *Store) Release(ctx context.Context, p *Principal, reqID string, taskID, fence int64, reason string) (any, error) {
	return s.write(ctx, p, reqID, "release", []any{taskID, fence, reason}, func(w *W) (any, error) {
		t, wf, _, err := w.held(taskID, fence)
		if err != nil {
			return nil, err
		}
		lane, _ := wf.laneByActive(t.State)
		to := firstNonEmpty(lane.ReleaseTo, wf.InitialState)
		return w.clearLease(t, to, "task.released", map[string]any{"reason": reason})
	})
}

// Complete finishes the active lane: summary note, last_worker = caller, state = lane.complete_to.
func (s *Store) Complete(ctx context.Context, p *Principal, reqID string, taskID, fence int64, summary string) (any, error) {
	return s.write(ctx, p, reqID, "complete", []any{taskID, fence, summary}, func(w *W) (any, error) {
		t, wf, _, err := w.held(taskID, fence)
		if err != nil {
			return nil, err
		}
		lane, ok := wf.laneByActive(t.State)
		if !ok {
			return nil, fail(409, "not_completable", "state %q is not an active lane state", t.State)
		}
		if err := w.can("task.complete:"+lane.Active, resOf(t)); err != nil {
			return nil, err
		}
		if strings.TrimSpace(summary) != "" {
			if err := w.note(t, "summary", summary, nil, &fence); err != nil {
				return nil, err
			}
		}
		if _, err := w.exec(`UPDATE tasks SET last_worker=$2 WHERE id=$1`, t.ID, p.ID); err != nil {
			return nil, err
		}
		return w.clearLease(t, lane.CompleteTo, "task.completed", map[string]any{"summary": summary})
	})
}

// Handoff = handoff note + release + optional assignee, in one transaction.
func (s *Store) Handoff(ctx context.Context, p *Principal, reqID string, taskID, fence int64, note, to string) (any, error) {
	return s.write(ctx, p, reqID, "handoff", []any{taskID, fence, note, to}, func(w *W) (any, error) {
		t, wf, _, err := w.held(taskID, fence)
		if err != nil {
			return nil, err
		}
		if len(strings.TrimSpace(note)) < 3 {
			return nil, errInvalid("a handoff needs a note (what is done, what is next)")
		}
		if err := w.can("task.handoff", resOf(t)); err != nil {
			return nil, err
		}
		var toP *string
		if to != "" {
			var ok bool
			if err := w.row(`SELECT enabled FROM principals WHERE id=$1`, to).Scan(&ok); err != nil || !ok {
				return nil, errInvalid("handoff target %q is not an enabled principal", to)
			}
			toP = &to
		}
		if err := w.note(t, "handoff", note, toP, &fence); err != nil {
			return nil, err
		}
		if _, err := w.exec(`UPDATE tasks SET assignee=$2, last_worker=$3 WHERE id=$1`, t.ID, toP, p.ID); err != nil {
			return nil, err
		}
		lane, _ := wf.laneByActive(t.State)
		return w.clearLease(t, firstNonEmpty(lane.ReleaseTo, wf.InitialState), "task.handoff", map[string]any{"to": toP})
	})
}

// ForceRelease breaks someone else's lease. Code rule: humans only; policy must also allow it.
func (s *Store) ForceRelease(ctx context.Context, p *Principal, reqID string, taskID int64, reason string) (any, error) {
	return s.write(ctx, p, reqID, "force_release", []any{taskID, reason}, func(w *W) (any, error) {
		if !p.IsHuman {
			return nil, fail(403, "humans_only", "force release is limited to human principals")
		}
		if len(strings.TrimSpace(reason)) < 3 {
			return nil, errInvalid("a reason (3+ characters) is required")
		}
		t, err := getTask(ctx, w.tx, taskID, true)
		if err != nil {
			return nil, err
		}
		if t.Holder == nil {
			return nil, fail(409, "not_held", "task %d has no lease", taskID)
		}
		if err := w.can("task.force_release", resOf(t)); err != nil {
			return nil, err
		}
		wf, err := loadWorkflowFor(ctx, w.tx, t.Project)
		if err != nil {
			return nil, err
		}
		if err := w.note(t, "note", "Force-released from "+*t.Holder+": "+reason, nil, nil); err != nil {
			return nil, err
		}
		lane, _ := wf.laneByActive(t.State)
		return w.clearLease(t, firstNonEmpty(lane.ReleaseTo, wf.InitialState), "task.force_released",
			map[string]any{"previous_holder": t.Holder, "reason": reason})
	})
}

func firstNonEmpty(a ...string) string {
	for _, s := range a {
		if s != "" {
			return s
		}
	}
	return ""
}
