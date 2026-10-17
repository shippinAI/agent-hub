package store

import (
	"context"
	"reflect"
	"slices"
	"strings"
)

func (w *W) note(t Task, kind, body string, to *string, fence *int64) error {
	var id int64
	if err := w.row(`INSERT INTO notes(task_id,author,kind,body,to_principal,fence) VALUES($1,$2,$3,$4,$5,$6) RETURNING id`,
		t.ID, w.P.ID, kind, body, to, fence).Scan(&id); err != nil {
		return err
	}
	return w.Event("note.added", &t.ID, fence, map[string]any{"note_id": id, "kind": kind, "to": to, "chars": len(body)})
}

// NoteInput is add_note's input. A supplied fence must be current.
type NoteInput struct {
	Kind  string `json:"kind"`
	Body  string `json:"body"`
	To    string `json:"to,omitempty"`
	Fence *int64 `json:"fence,omitempty"`
}

// AddNote appends a note; kinds come from the note_kinds setting.
func (s *Store) AddNote(ctx context.Context, p *Principal, reqID string, taskID int64, in NoteInput) (any, error) {
	return s.write(ctx, p, reqID, "add_note", []any{taskID, in}, func(w *W) (any, error) {
		t, err := getTask(ctx, w.tx, taskID, true)
		if err != nil {
			return nil, err
		}
		if in.Fence != nil && *in.Fence != t.Fence {
			return nil, fail(409, "stale_fence", "fence %d is not current", *in.Fence)
		}
		st, _ := loadSettings(ctx, w.tx)
		if in.Kind == "" {
			in.Kind = "note"
		}
		if kinds := st.Strings("note_kinds"); len(kinds) > 0 && !slices.Contains(kinds, in.Kind) {
			return nil, errInvalid("note kind must be one of %v", kinds)
		}
		if b := strings.TrimSpace(in.Body); b == "" || len(b) > 20000 {
			return nil, errInvalid("note body must be 1-20000 chars")
		}
		if err := w.can("note.add:"+in.Kind, resOf(t)); err != nil {
			return nil, err
		}
		var to *string
		if in.To != "" {
			to = &in.To
		}
		if err := w.note(t, in.Kind, in.Body, to, in.Fence); err != nil {
			return nil, err
		}
		var n Note
		err = w.row(`SELECT id,task_id,author,kind,body,to_principal,fence,created_at FROM notes WHERE task_id=$1 ORDER BY id DESC LIMIT 1`,
			taskID).Scan(&n.ID, &n.TaskID, &n.Author, &n.Kind, &n.Body, &n.ToPrincipal, &n.Fence, &n.CreatedAt)
		return n, err
	})
}

// AddDep records task -> depends_on, refusing cycles.
func (s *Store) AddDep(ctx context.Context, p *Principal, reqID string, taskID, dependsOn int64) (any, error) {
	return s.write(ctx, p, reqID, "add_dep", []any{taskID, dependsOn}, func(w *W) (any, error) {
		t, err := getTask(ctx, w.tx, taskID, true)
		if err != nil {
			return nil, err
		}
		if err := w.can("task.edit", resOf(t)); err != nil {
			return nil, err
		}
		var cycle bool
		err = w.row(`WITH RECURSIVE up(id) AS (SELECT $2::bigint UNION SELECT d.depends_on FROM task_deps d JOIN up ON d.task_id=up.id)
			SELECT EXISTS(SELECT 1 FROM up WHERE id=$1)`, taskID, dependsOn).Scan(&cycle)
		if err != nil {
			return nil, err
		}
		if cycle {
			return nil, errInvalid("dependency would create a cycle")
		}
		if _, err := w.exec(`INSERT INTO task_deps VALUES($1,$2) ON CONFLICT DO NOTHING`, taskID, dependsOn); err != nil {
			return nil, errInvalid("dependency: %v", err)
		}
		if err := w.Event("dep.added", &taskID, nil, map[string]any{"depends_on": dependsOn}); err != nil {
			return nil, err
		}
		return getTask(ctx, w.tx, taskID, false)
	})
}

func isNilPtr(v any) bool {
	rv := reflect.ValueOf(v)
	return v == nil || (rv.Kind() == reflect.Pointer && rv.IsNil())
}

func labelsArg(set *[]string, clean []string) any {
	if set == nil {
		return nil
	}
	return clean
}

func nullJSON(b []byte) any {
	if len(b) == 0 {
		return nil
	}
	return b
}
