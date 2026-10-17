package store

import (
	"context"
	"encoding/json"
	"strings"

	"agenthub/internal/policy"
)

// NewTask is the create input.
type NewTask struct {
	Project     string   `json:"project"`
	Title       string   `json:"title"`
	Description string   `json:"description,omitempty"`
	Priority    *int     `json:"priority,omitempty"`
	Labels      []string `json:"labels,omitempty"`
	Assignee    string   `json:"assignee,omitempty"`
	DependsOn   []int64  `json:"depends_on,omitempty"`
}

// CreateTask adds a task in the project's workflow initial state.
func (s *Store) CreateTask(ctx context.Context, p *Principal, reqID string, in NewTask) (any, error) {
	return s.write(ctx, p, reqID, "create_task", in, func(w *W) (any, error) {
		in.Title, in.Labels = strings.TrimSpace(in.Title), cleanLabels(in.Labels)
		if in.Title == "" || len(in.Title) > 300 {
			return nil, errInvalid("title is required (max 300 chars)")
		}
		wf, err := loadWorkflowFor(ctx, w.tx, in.Project)
		if err != nil {
			return nil, err
		}
		if err := w.can("task.create", policy.Resource{Project: in.Project, Labels: in.Labels}); err != nil {
			return nil, err
		}
		st, _ := loadSettings(ctx, w.tx)
		prio := st.Int("default_priority", 2)
		if in.Priority != nil {
			prio = *in.Priority
		}
		var asg *string
		if in.Assignee != "" {
			asg = &in.Assignee
		}
		var id int64
		err = w.row(`INSERT INTO tasks(project,title,description,state,priority,labels,assignee,created_by)
			VALUES($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id`, in.Project, in.Title, in.Description, wf.InitialState,
			prio, in.Labels, asg, p.ID).Scan(&id)
		if err != nil {
			return nil, err
		}
		for _, d := range in.DependsOn {
			if _, err := w.exec(`INSERT INTO task_deps(task_id,depends_on) VALUES($1,$2)`, id, d); err != nil {
				return nil, errInvalid("dependency %d: %v", d, err)
			}
		}
		if err := w.Event("task.created", &id, nil, map[string]any{"project": in.Project, "title": in.Title,
			"state": wf.InitialState, "depends_on": in.DependsOn}); err != nil {
			return nil, err
		}
		return getTask(ctx, w.tx, id, false)
	})
}

// TaskPatch is an optimistic-concurrency update; Fence is required while you hold the task.
type TaskPatch struct {
	Version     int64           `json:"version"`
	Fence       *int64          `json:"fence,omitempty"`
	Title       *string         `json:"title,omitempty"`
	Description *string         `json:"description,omitempty"`
	Priority    *int            `json:"priority,omitempty"`
	Labels      *[]string       `json:"labels,omitempty"`
	Assignee    *string         `json:"assignee,omitempty"`
	State       *string         `json:"state,omitempty"`
	Progress    json.RawMessage `json:"progress,omitempty"`
}

// UpdateTask applies a patch after version, fence, workflow and policy checks.
func (s *Store) UpdateTask(ctx context.Context, p *Principal, reqID string, id int64, in TaskPatch) (any, error) {
	return s.write(ctx, p, reqID, "update_task", []any{id, in}, func(w *W) (any, error) {
		t, err := getTask(ctx, w.tx, id, true)
		if err != nil {
			return nil, err
		}
		if in.Fence != nil && *in.Fence != t.Fence {
			e := fail(409, "stale_fence", "fence %d is not current; your lease was lost", *in.Fence)
			e.Current = map[string]any{"fence": t.Fence, "holder": t.Holder}
			return nil, e
		}
		if in.Version != t.Version {
			return nil, errVersion(t)
		}
		live := t.Holder != nil && !t.LeaseExpired
		mine := live && *t.Holder == p.ID
		if mine && in.Fence == nil {
			return nil, fail(409, "stale_fence", "fence is required while you hold the task")
		}
		if live && !mine {
			if err := w.can("task.edit_held", resOf(t)); err != nil {
				return nil, err
			}
		}
		if err := w.can("task.edit", resOf(t)); err != nil {
			return nil, err
		}
		changed := map[string]any{}
		if in.State != nil && *in.State != t.State {
			if err := w.checkTransition(t, *in.State, live); err != nil {
				return nil, err
			}
			changed["state"] = []string{t.State, *in.State}
		}
		if in.Title != nil && (strings.TrimSpace(*in.Title) == "" || len(*in.Title) > 300) {
			return nil, errInvalid("title must be 1-300 chars")
		}
		var labels []string
		if in.Labels != nil {
			labels = cleanLabels(*in.Labels)
			changed["labels"] = labels
		}
		for k, v := range map[string]any{"title": in.Title, "description": in.Description, "priority": in.Priority, "assignee": in.Assignee} {
			if !isNilPtr(v) {
				changed[k] = v
			}
		}
		if len(in.Progress) > 0 {
			changed["progress"] = true
		}
		var asg any = nil
		if in.Assignee != nil && *in.Assignee != "" {
			asg = *in.Assignee
		}
		_, err = w.exec(`UPDATE tasks SET title=coalesce($2,title), description=coalesce($3,description), priority=coalesce($4,priority),
			labels=coalesce($5,labels), state=coalesce($6,state), progress=coalesce($7,progress),
			assignee=CASE WHEN $8 THEN $9 ELSE assignee END, version=version+1, updated_at=clock_timestamp() WHERE id=$1`,
			id, in.Title, in.Description, in.Priority, labelsArg(in.Labels, labels), in.State, nullJSON(in.Progress), in.Assignee != nil, asg)
		if err != nil {
			return nil, err
		}
		if err := w.Event("task.updated", &t.ID, in.Fence, map[string]any{"changed": changed, "version": t.Version + 1}); err != nil {
			return nil, err
		}
		return getTask(ctx, w.tx, id, false)
	})
}
