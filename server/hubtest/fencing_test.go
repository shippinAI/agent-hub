package hubtest

import (
	"context"
	"testing"
	"time"
)

// An expired lease is reclaimed by another agent and the old holder's writes are rejected by fencing.
func TestExpiredLeaseReclaimedAndStaleFenceRejected(t *testing.T) {
	e := setup(t)
	e.setSetting("lease_min_ttl_seconds", 1)
	id := e.newTask("jesse", "fenced task", nil)
	a := e.call("claude-code", "POST", "/claim-next", map[string]any{"ttl_seconds": 1})
	oldFence, ver := num(a.Body["fence"]), num(taskOf(a)["version"])
	if a.Body["claimed"] != true || num(taskOf(a)["id"]) != id {
		t.Fatalf("claude-code claim: %v", a.Body)
	}
	if r := e.call("codex", "POST", "/claim-next", map[string]any{}); r.Body["claimed"] != false {
		t.Fatalf("codex should get nothing while the lease is live: %v", r.Body)
	}
	time.Sleep(1300 * time.Millisecond) // claude-code "pauses" past its lease
	b := e.call("codex", "POST", "/claim-next", map[string]any{"ttl_seconds": 60})
	newFence := num(b.Body["fence"])
	if b.Body["claimed"] != true || num(taskOf(b)["id"]) != id || newFence <= oldFence {
		t.Fatalf("codex reclaim: %v", b.Body)
	}
	checks := []struct{ name, method, path string; body map[string]any }{
		{"heartbeat", "POST", "/tasks/{id}/heartbeat", map[string]any{"fence": oldFence}},
		{"complete", "POST", "/tasks/{id}/complete", map[string]any{"fence": oldFence, "summary": "stale write"}},
		{"update", "PATCH", "/tasks/{id}", map[string]any{"fence": oldFence, "version": ver, "title": "hijacked"}},
		{"note", "POST", "/tasks/{id}/notes", map[string]any{"fence": oldFence, "body": "late note"}},
		{"handoff", "POST", "/tasks/{id}/handoff", map[string]any{"fence": oldFence, "note": "handing off"}},
	}
	for _, c := range checks {
		r := e.call("claude-code", c.method, replaceID(c.path, id), c.body)
		if r.Status != 409 || r.code() != "stale_fence" {
			t.Errorf("old holder %s: got %d %s, want 409 stale_fence", c.name, r.Status, r.code())
		}
	}
	if r := e.call("codex", "POST", replaceID("/tasks/{id}/heartbeat", id), map[string]any{"fence": newFence}); r.Status != 200 {
		t.Errorf("new holder heartbeat: %d %v", r.Status, r.Body)
	}
	got := e.call("codex", "GET", replaceID("/tasks/{id}", id), nil)
	if tk := taskOf(got); tk["title"] != "fenced task" || tk["holder"] != "codex" {
		t.Errorf("task changed by stale writer: %v", tk)
	}
	var expired int
	_ = e.pool.QueryRow(context.Background(), `SELECT count(*) FROM events WHERE action='lease.expired' AND task_id=$1`, id).Scan(&expired)
	if expired != 1 {
		t.Errorf("lease.expired events = %d", expired)
	}
}

// The reaper returns lapsed tasks to their lane and parks repeat offenders in the attention state.
func TestReaperExpiresAndEscalates(t *testing.T) {
	e := setup(t)
	e.setSetting("lease_min_ttl_seconds", 1)
	e.setSetting("max_lease_expiries", 2)
	id := e.newTask("jesse", "flaky task", nil)
	for round := 1; round <= 2; round++ {
		if r := e.call("cursor", "POST", replaceID("/tasks/{id}/claim", id), map[string]any{"ttl_seconds": 1}); r.Status != 200 {
			t.Fatalf("round %d claim: %v", round, r.Body)
		}
		time.Sleep(1200 * time.Millisecond)
		if n, err := e.st.ReapOnce(context.Background()); err != nil || n != 1 {
			t.Fatalf("round %d reap: n=%d err=%v", round, n, err)
		}
		tk := taskOf(e.call("cursor", "GET", replaceID("/tasks/{id}", id), nil))
		want := map[int]string{1: "ready", 2: "needs_attention"}[round]
		if tk["state"] != want || tk["holder"] != nil || num(tk["expiry_count"]) != int64(round) {
			t.Errorf("after expiry %d: state=%v holder=%v count=%v, want %s", round, tk["state"], tk["holder"], tk["expiry_count"], want)
		}
	}
}

func replaceID(p string, id int64) string {
	return replaceAll(p, "{id}", itoa(id))
}
