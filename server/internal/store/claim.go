package store

import (
	"context"
	"slices"

	"agenthub/internal/policy"
)

func resOf(t Task) policy.Resource { return policy.Resource{Project: t.Project, Labels: t.Labels} }

// Claim takes a lease on one task. Re-claiming a task you already hold returns the current lease.
func (s *Store) Claim(ctx context.Context, p *Principal, reqID string, taskID int64, ttl int) (any, error) {
	return s.write(ctx, p, reqID, "claim", []any{taskID, ttl}, func(w *W) (any, error) {
		t, err := getTask(ctx, w.tx, taskID, true)
		if err != nil {
			return nil, err
		}
		wf, err := loadWorkflowFor(ctx, w.tx, t.Project)
		if err != nil {
			return nil, err
		}
		st, err := loadSettings(ctx, w.tx)
		if err != nil {
			return nil, err
		}
		if t.Holder != nil && !t.LeaseExpired {
			if *t.Holder == p.ID {
				return Lease{Task: t, Fence: t.Fence, LeaseID: *t.LeaseID, LeaseExpiresAt: *t.LeaseExpiresAt,
					HeartbeatEvery: beatEvery(st, derefInt(t.LeaseTTL)), AlreadyHeld: true, Claimed: true}, nil
			}
			e := fail(409, "held_by_other", "task is leased by %s", *t.Holder)
			e.Current = map[string]any{"holder": t.Holder, "lease_expires_at": t.LeaseExpiresAt}
			return nil, e
		}
		if t.Holder != nil {
			if t, err = w.expire(t, wf, st); err != nil {
				return nil, err
			}
		}
		lane, ok := wf.laneByQueue(t.State)
		if !ok {
			return nil, fail(409, "not_claimable", "state %q is not a claimable queue", t.State)
		}
		if len(t.BlockedBy) > 0 {
			e := fail(409, "blocked", "waiting on dependencies")
			e.Current = t.BlockedBy
			return nil, e
		}
		if err := w.can("task.claim:"+lane.Queue, resOf(t)); err != nil {
			return nil, err
		}
		if lane.DistinctFromPrevious && t.LastWorker != nil && *t.LastWorker == p.ID {
			return nil, fail(409, "distinct_required", "the %s lane needs a different principal than the previous worker", lane.Queue)
		}
		if err := w.checkWIP(st); err != nil {
			return nil, err
		}
		return w.takeLease(t, lane, st, ttl)
	})
}

// NextFilter narrows claim_next.
type NextFilter struct {
	Project    string   `json:"project,omitempty"`
	Labels     []string `json:"labels,omitempty"`
	States     []string `json:"states,omitempty"`
	TTLSeconds int      `json:"ttl_seconds,omitempty"`
}

// ClaimNext atomically claims the best ready task the caller is allowed to take (SKIP LOCKED queue).
func (s *Store) ClaimNext(ctx context.Context, p *Principal, reqID string, f NextFilter) (any, error) {
	return s.write(ctx, p, reqID, "claim_next", f, func(w *W) (any, error) {
		st, err := loadSettings(ctx, w.tx)
		if err != nil {
			return nil, err
		}
		if err := w.checkWIP(st); err != nil {
			return nil, err
		}
		rows, err := w.tx.Query(ctx, `SELECT t.id FROM tasks t WHERE
			((t.holder IS NULL AND hub_is_queue(t.project,t.state)) OR (t.holder IS NOT NULL AND t.lease_expires_at < clock_timestamp()))
			AND cardinality(hub_open_deps(t.id))=0 AND (t.assignee IS NULL OR t.assignee=$1)
			AND ($2='' OR t.project=$2) AND t.labels @> $3 AND (cardinality($4::text[])=0 OR t.state=ANY($4))
			ORDER BY (t.assignee IS NOT NULL) DESC, t.priority, t.created_at, t.id LIMIT 25 FOR UPDATE SKIP LOCKED`,
			p.ID, f.Project, cleanLabels(f.Labels), nonNil(f.States))
		if err != nil {
			return nil, err
		}
		var ids []int64
		for rows.Next() {
			var id int64
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return nil, err
			}
			ids = append(ids, id)
		}
		rows.Close()
		rules, err := loadRules(ctx, w.tx)
		if err != nil {
			return nil, err
		}
		for _, id := range ids {
			t, err := getTask(ctx, w.tx, id, false)
			if err != nil {
				return nil, err
			}
			wf, err := loadWorkflowFor(ctx, w.tx, t.Project)
			if err != nil {
				return nil, err
			}
			if t.Holder != nil {
				if t, err = w.expire(t, wf, st); err != nil {
					return nil, err
				}
			}
			lane, ok := wf.laneByQueue(t.State)
			if !ok || len(f.States) > 0 && !slices.Contains(f.States, t.State) {
				continue
			}
			if lane.DistinctFromPrevious && t.LastWorker != nil && *t.LastWorker == p.ID {
				continue
			}
			if !policy.Decide(rules, subjectOf(p), "task.claim:"+lane.Queue, resOf(t)).Allowed {
				continue
			}
			return w.takeLease(t, lane, st, f.TTLSeconds)
		}
		return map[string]any{"claimed": false, "message": "no ready task you are allowed to claim"}, nil
	})
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
