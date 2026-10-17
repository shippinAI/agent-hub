package hubtest

import (
	"context"
	"sync"
	"testing"
)

func TestIdempotencyKeys(t *testing.T) {
	e := setup(t)
	body := map[string]any{"project": "general", "title": "only once"}
	a := e.call("codex", "POST", "/tasks", body, "Idempotency-Key: k-1")
	b := e.call("codex", "POST", "/tasks", body, "Idempotency-Key: k-1")
	if a.Status != 200 || b.Status != 200 || num(a.Body["id"]) != num(b.Body["id"]) {
		t.Fatalf("retry should return the same task: %v / %v", a.Body, b.Body)
	}
	if b.Header.Get("Idempotent-Replayed") != "true" || a.Header.Get("Idempotent-Replayed") != "" {
		t.Errorf("replay header: first=%q second=%q", a.Header.Get("Idempotent-Replayed"), b.Header.Get("Idempotent-Replayed"))
	}
	c := e.call("codex", "POST", "/tasks", map[string]any{"project": "general", "title": "different"}, "Idempotency-Key: k-1")
	if c.Status != 422 || c.code() != "idempotency_mismatch" {
		t.Errorf("same key, different body: %d %s", c.Status, c.code())
	}
	// keys are per principal: another agent may use the same key string
	if d := e.call("cursor", "POST", "/tasks", body, "Idempotency-Key: k-1"); d.Status != 200 || num(d.Body["id"]) == num(a.Body["id"]) {
		t.Errorf("other principal, same key: %v", d.Body)
	}
	// concurrent retries of one complete: exactly one applies
	id := e.newTask("jesse", "complete me", nil)
	l := e.call("claude-code", "POST", "/tasks/"+itoa(id)+"/claim", map[string]any{})
	fence := num(l.Body["fence"])
	var wg sync.WaitGroup
	codes := make([]int, 8)
	for i := range codes {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			codes[i] = e.call("claude-code", "POST", "/tasks/"+itoa(id)+"/complete",
				map[string]any{"fence": fence, "summary": "done"}, "Idempotency-Key: done-1").Status
		}(i)
	}
	wg.Wait()
	for i, c := range codes {
		if c != 200 {
			t.Errorf("retry %d got %d", i, c)
		}
	}
	var completes, notes int
	_ = e.pool.QueryRow(context.Background(), `SELECT count(*) FROM events WHERE action='task.completed' AND task_id=$1`, id).Scan(&completes)
	_ = e.pool.QueryRow(context.Background(), `SELECT count(*) FROM notes WHERE task_id=$1 AND kind='summary'`, id).Scan(&notes)
	if completes != 1 || notes != 1 {
		t.Errorf("complete applied %d times, %d summary notes; want 1/1", completes, notes)
	}
}

func TestOptimisticConcurrency(t *testing.T) {
	e := setup(t)
	id := e.newTask("jesse", "edit me", nil)
	p := "/tasks/" + itoa(id)
	if r := e.call("codex", "PATCH", p, map[string]any{"title": "v2"}, `If-Match: "1"`); r.Status != 200 || num(r.Body["version"]) != 2 {
		t.Fatalf("update with right version: %v", r.Body)
	}
	r := e.call("cursor", "PATCH", p, map[string]any{"version": 1, "title": "lost update"})
	if r.Status != 409 || r.code() != "version_conflict" {
		t.Fatalf("stale version: %d %s", r.Status, r.code())
	}
	cur := r.Body["error"].(map[string]any)["current"].(map[string]any)
	if cur["title"] != "v2" || num(cur["version"]) != 2 {
		t.Errorf("conflict should return the current row: %v", cur)
	}
	// holder must send a fence; others need task.edit_held (humans only by preset)
	l := e.call("codex", "POST", p+"/claim", map[string]any{})
	if r := e.call("codex", "PATCH", p, map[string]any{"version": num(taskOf(l)["version"]), "title": "x"}); r.code() != "stale_fence" {
		t.Errorf("holder without fence: %s", r.code())
	}
	if r := e.call("cursor", "PATCH", p, map[string]any{"version": num(taskOf(l)["version"]), "title": "x"}); r.code() != "policy_denied" {
		t.Errorf("non-holder agent editing a held task: %s", r.code())
	}
	if r := e.call("jesse", "PATCH", p, map[string]any{"version": num(taskOf(l)["version"]), "description": "human note"}); r.Status != 200 {
		t.Errorf("human edit_held: %v", r.Body)
	}
}
