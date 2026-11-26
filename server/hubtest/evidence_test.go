package hubtest

import (
	"context"
	"testing"
)

// requireEvidence makes the default workflow's build lane demand passing evidence of the given kinds (data, not code).
func (e *env) requireEvidence(kinds ...string) {
	cfg := e.call("jesse", "GET", "/config", nil)
	var wf map[string]any
	for _, w := range cfg.Body["workflows"].([]any) {
		if m := w.(map[string]any); m["id"] == "default" {
			wf = m
		}
	}
	lanes := wf["lanes"].([]any)
	lanes[0].(map[string]any)["requires_evidence"] = kinds
	if r := e.call("jesse", "PUT", "/workflows/default", wf); r.Status != 200 {
		e.t.Fatalf("put workflow: %d %v", r.Status, r.Body)
	}
}

func current(r resp) map[string]any {
	er, _ := r.Body["error"].(map[string]any)
	c, _ := er["current"].(map[string]any)
	return c
}

func strs(v any) []string {
	out := []string{}
	for _, x := range v.([]any) {
		out = append(out, x.(string))
	}
	return out
}

// TestEvidenceGate: a lane that requires evidence refuses complete until passing proof exists for the
// current lease; a later fail supersedes a pass; proof from an earlier lease does not count; rows are
// append-only and sit in the hash chain; acceptance criteria round-trip.
func TestEvidenceGate(t *testing.T) {
	e := setup(t)
	if r := e.call("jesse", "PUT", "/workflows/default", map[string]any{"id": "default"}); r.Status == 200 {
		t.Fatalf("empty workflow accepted: %v", r.Body)
	}
	e.requireEvidence("test_run")

	id := e.newTask("jesse", "gated task", map[string]any{"acceptance": []string{"tests pass", " tests pass ", "", "docs updated"}})
	p := "/tasks/" + itoa(id)
	if got := strs(taskOf(e.call("codex", "GET", p, nil))["acceptance"]); len(got) != 2 || got[0] != "tests pass" || got[1] != "docs updated" {
		t.Fatalf("acceptance not cleaned: %v", got)
	}

	l1 := e.call("codex", "POST", p+"/claim", map[string]any{})
	f1 := num(l1.Body["fence"])
	if r := e.call("codex", "POST", p+"/complete", map[string]any{"fence": f1, "summary": "done?"}); r.code() != "evidence_missing" ||
		len(strs(current(r)["missing"])) != 1 {
		t.Fatalf("complete without evidence: %d %v", r.Status, r.Body)
	}
	if r := e.call("codex", "POST", p+"/evidence", map[string]any{"fence": f1, "kind": "Not A Kind", "status": "pass"}); r.code() != "invalid" {
		t.Errorf("bad kind: %v", r.Body)
	}
	if r := e.call("codex", "POST", p+"/evidence", map[string]any{"fence": f1, "kind": "test_run", "status": "maybe"}); r.code() != "invalid" {
		t.Errorf("bad status: %v", r.Body)
	}
	if r := e.call("codex", "POST", p+"/evidence", map[string]any{"fence": f1 + 1000, "kind": "test_run", "status": "pass"}); r.code() != "stale_fence" {
		t.Errorf("stale fence evidence: %v", r.Body)
	}
	if r := e.call("codex", "POST", p+"/evidence", map[string]any{"fence": f1, "kind": "test_run", "status": "pass",
		"summary": "go test ./... ok", "sha256": "ABCDEF" + "0123456789abcdef0123456789abcdef0123456789abcdef0123456789"}); r.Status != 200 || num(r.Body["fence"]) != f1 {
		t.Fatalf("add pass: %d %v", r.Status, r.Body)
	}
	if r := e.call("codex", "POST", p+"/evidence", map[string]any{"fence": f1, "kind": "test_run", "status": "fail", "summary": "flaky test failed"}); r.Status != 200 {
		t.Fatalf("add fail: %v", r.Body)
	}
	if r := e.call("codex", "POST", p+"/complete", map[string]any{"fence": f1, "summary": "done?"}); r.code() != "evidence_missing" ||
		len(strs(current(r)["failing"])) != 1 {
		t.Fatalf("a later fail must supersede the pass: %v", r.Body)
	}

	// New lease: the old lease's proof no longer counts, and the old fence cannot add more.
	e.call("codex", "POST", p+"/release", map[string]any{"fence": f1, "reason": "retry"})
	l2 := e.call("codex", "POST", p+"/claim", map[string]any{})
	f2 := num(l2.Body["fence"])
	if f2 <= f1 {
		t.Fatalf("fence did not advance: %d -> %d", f1, f2)
	}
	if r := e.call("codex", "POST", p+"/evidence", map[string]any{"fence": f1, "kind": "test_run", "status": "pass"}); r.code() != "stale_fence" {
		t.Errorf("old lease adds evidence: %v", r.Body)
	}
	if r := e.call("codex", "POST", p+"/complete", map[string]any{"fence": f2, "summary": "done?"}); r.code() != "evidence_missing" {
		t.Fatalf("old lease evidence counted: %v", r.Body)
	}
	// A non-holder (CI) may attach without a fence; it is stamped with the current lease.
	if r := e.call("clyffy", "POST", p+"/evidence", map[string]any{"kind": "test_run", "status": "pass", "ref": "https://ci.example/run/1"}); r.Status != 200 ||
		num(r.Body["fence"]) != f2 {
		t.Fatalf("CI evidence: %d %v", r.Status, r.Body)
	}
	if r := e.call("codex", "POST", p+"/complete", map[string]any{"fence": f2, "summary": "green"}); r.Status != 200 || r.Body["state"] != "review" {
		t.Fatalf("complete with evidence: %d %v", r.Status, r.Body)
	}

	// Over MCP too: the review lane has no requirement, and evidence is visible on get_task.
	cs := e.mcpSession("clyffy")
	rv, _ := callTool(t, cs, "claim", map[string]any{"task_id": id})
	if out, isErr := callTool(t, cs, "add_evidence", map[string]any{"task_id": id, "fence": rv["fence"], "kind": "review", "status": "pass", "summary": "LGTM"}); isErr {
		t.Fatalf("mcp add_evidence: %v", out)
	}
	got, _ := callTool(t, cs, "get_task", map[string]any{"task_id": id})
	if ev, _ := got["evidence"].([]any); len(ev) != 4 {
		t.Fatalf("evidence on get_task: %v", got["evidence"])
	}

	ctx := context.Background()
	if _, err := e.pool.Exec(ctx, `UPDATE task_evidence SET status='pass' WHERE status='fail'`); err == nil {
		t.Error("UPDATE on task_evidence must be refused")
	}
	if _, err := e.pool.Exec(ctx, `DELETE FROM task_evidence`); err == nil {
		t.Error("DELETE on task_evidence must be refused")
	}
	var n int
	if err := e.pool.QueryRow(ctx, `SELECT count(*) FROM events WHERE action='evidence.added' AND task_id=$1`, id).Scan(&n); err != nil || n != 4 {
		t.Errorf("evidence events: %d %v", n, err)
	}
	if v := e.call("codex", "GET", "/events/verify", nil); v.Body["ok"] != true {
		t.Errorf("chain: %v", v.Body)
	}

	task := taskOf(e.call("jesse", "GET", p, nil))
	if r := e.call("jesse", "PATCH", p, map[string]any{"acceptance": []string{"only this"}, "version": task["version"]}); r.Status != 200 ||
		len(strs(r.Body["acceptance"])) != 1 {
		t.Errorf("patch acceptance: %d %v", r.Status, r.Body)
	}
}
