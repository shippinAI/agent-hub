package store

import (
	"context"
	"strings"

	"github.com/jackc/pgx/v5"
)

const taskCols = `t.id,t.project,t.title,t.description,t.state,t.priority,t.labels,t.assignee,t.holder,t.lease_id::text,
	t.lease_ttl_seconds,t.lease_expires_at,t.heartbeat_at,t.fence,t.version,t.expiry_count,t.last_worker,t.progress,
	t.created_by,t.created_at,t.updated_at,hub_open_deps(t.id),(t.holder IS NULL AND hub_is_queue(t.project,t.state)),
	coalesce(t.lease_expires_at < clock_timestamp(), false)`

func scanTask(r pgx.Row) (Task, error) {
	var t Task
	var queue bool
	err := r.Scan(&t.ID, &t.Project, &t.Title, &t.Description, &t.State, &t.Priority, &t.Labels, &t.Assignee, &t.Holder,
		&t.LeaseID, &t.LeaseTTL, &t.LeaseExpiresAt, &t.HeartbeatAt, &t.Fence, &t.Version, &t.ExpiryCount, &t.LastWorker,
		&t.Progress, &t.CreatedBy, &t.CreatedAt, &t.UpdatedAt, &t.BlockedBy, &queue, &t.LeaseExpired)
	t.Ready = queue && len(t.BlockedBy) == 0
	return t, err
}

func getTask(ctx context.Context, q querier, id int64, lock bool) (Task, error) {
	sql := `SELECT ` + taskCols + ` FROM tasks t WHERE t.id=$1`
	if lock {
		sql += ` FOR UPDATE OF t`
	}
	t, err := scanTask(q.QueryRow(ctx, sql, id))
	if err == pgx.ErrNoRows {
		return t, errNotFound("task")
	}
	return t, err
}

// Task reads one task.
func (s *Store) Task(ctx context.Context, id int64) (Task, error) { return getTask(ctx, s.DB, id, false) }

// TaskFilter selects tasks for lists, the board and search.
type TaskFilter struct {
	Project, State, Holder, Label, Query string
	ReadyOnly                            bool
	Limit                                int
}

// Tasks lists tasks ordered by priority then age.
func (s *Store) Tasks(ctx context.Context, f TaskFilter) ([]Task, error) {
	if f.Limit <= 0 || f.Limit > 1000 {
		f.Limit = 500
	}
	where := []string{"true"}
	args := []any{}
	add := func(cond string, v any) {
		args = append(args, v)
		where = append(where, strings.ReplaceAll(cond, "?", "$"+itoa(len(args))))
	}
	if f.Project != "" {
		add("t.project=?", f.Project)
	}
	if f.State != "" {
		add("t.state=?", f.State)
	}
	if f.Holder != "" {
		add("t.holder=?", f.Holder)
	}
	if f.Label != "" {
		add("?=ANY(t.labels)", f.Label)
	}
	if q := strings.TrimSpace(f.Query); q != "" {
		add(`(to_tsvector('simple', t.title||' '||t.description) @@ websearch_to_tsquery('simple', ?)
			OR t.title ILIKE '%'||?||'%' OR t.id::text=? OR EXISTS (SELECT 1 FROM notes n WHERE n.task_id=t.id AND n.body ILIKE '%'||?||'%'))`, q)
	}
	if f.ReadyOnly {
		where = append(where, "t.holder IS NULL AND hub_is_queue(t.project,t.state) AND cardinality(hub_open_deps(t.id))=0")
	}
	args = append(args, f.Limit)
	rows, err := s.DB.Query(ctx, `SELECT `+taskCols+` FROM tasks t WHERE `+strings.Join(where, " AND ")+
		` ORDER BY t.priority, t.created_at, t.id LIMIT $`+itoa(len(args)), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Task{}
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// Notes lists a task's notes oldest first.
func (s *Store) Notes(ctx context.Context, taskID int64) ([]Note, error) {
	rows, err := s.DB.Query(ctx, `SELECT id,task_id,author,kind,body,to_principal,fence,created_at FROM notes WHERE task_id=$1 ORDER BY id`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Note{}
	for rows.Next() {
		var n Note
		if err := rows.Scan(&n.ID, &n.TaskID, &n.Author, &n.Kind, &n.Body, &n.ToPrincipal, &n.Fence, &n.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// Deps lists what a task depends on.
func (s *Store) Deps(ctx context.Context, taskID int64) ([]int64, error) {
	var out []int64
	err := s.DB.QueryRow(ctx, `SELECT coalesce(array_agg(depends_on ORDER BY depends_on),'{}') FROM task_deps WHERE task_id=$1`, taskID).Scan(&out)
	return out, err
}
