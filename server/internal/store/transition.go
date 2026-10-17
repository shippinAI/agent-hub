package store

import "slices"

func (w *W) checkTransition(t Task, to string, live bool) error {
	wf, err := loadWorkflowFor(w.ctx, w.tx, t.Project)
	if err != nil {
		return err
	}
	ts, ok := wf.state(to)
	if !ok {
		return errInvalid("unknown state %q", to)
	}
	if live || ts.Kind == "active" {
		return fail(409, "use_lease_ops", "held/active states change only through claim, complete, release or handoff")
	}
	if !wf.allows(t.State, to) {
		return fail(409, "transition_not_allowed", "workflow %s has no transition %s->%s", wf.ID, t.State, to)
	}
	if slices.ContainsFunc(wf.Lanes, func(l Lane) bool { return l.Queue == t.State && l.DistinctFromPrevious }) &&
		t.LastWorker != nil && *t.LastWorker == w.P.ID {
		return fail(409, "distinct_required", "you did the previous step; someone else must move it out of %s", t.State)
	}
	return w.can("task.transition:"+t.State+"->"+to, resOf(t))
}
