package hubtest

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

// TestRunRecords: a run is opened under the caller's lease, its transcript arrives in ordered, retry-safe chunks,
// finishing hashes it into the event chain, and a finished run (and every chunk) is frozen.
func TestRunRecords(t *testing.T) {
	e := setup(t)
	id := e.newTask("jesse", "recorded work", nil)
	p := "/tasks/" + itoa(id)
	if r := e.call("codex", "POST", p+"/runs", map[string]any{"fence": 1, "adapter": "claude-code"}); r.code() != "stale_fence" && r.code() != "not_holder" {
		t.Fatalf("run without a lease: %v", r.Body)
	}
	f := num(e.call("codex", "POST", p+"/claim", map[string]any{}).Body["fence"])
	if r := e.call("codex", "POST", p+"/runs", map[string]any{"fence": f, "adapter": "Claude Code"}); r.code() != "invalid" {
		t.Errorf("bad adapter: %v", r.Body)
	}
	if r := e.call("cursor", "POST", p+"/runs", map[string]any{"fence": f, "adapter": "x"}); r.code() != "not_holder" {
		t.Errorf("non-holder opened a run: %v", r.Body)
	}
	r := e.call("codex", "POST", p+"/runs", map[string]any{"fence": f, "adapter": "claude-code", "seat": "s1", "model": "m"})
	if r.Status != 200 || r.Body["status"] != "running" {
		t.Fatalf("start run: %d %v", r.Status, r.Body)
	}
	run := "/runs/" + itoa(num(r.Body["id"]))
	if r := e.call("cursor", "POST", run+"/transcript", map[string]any{"seq": 0, "body": "x"}); r.code() != "policy_denied" {
		t.Errorf("someone else wrote the transcript: %v", r.Body)
	}
	for _, c := range []map[string]any{{"seq": 0, "body": "line one\n"}, {"seq": 1, "body": "line two\n"}} {
		if r := e.call("codex", "POST", run+"/transcript", c); r.Status != 200 {
			t.Fatalf("chunk: %v", r.Body)
		}
	}
	if r := e.call("codex", "POST", run+"/transcript", map[string]any{"seq": 1, "body": "line two\n"}); r.Body["duplicate"] != true {
		t.Errorf("retry of a chunk: %v", r.Body)
	}
	if r := e.call("codex", "POST", run+"/transcript", map[string]any{"seq": 1, "body": "other\n"}); r.code() != "chunk_conflict" {
		t.Errorf("rewrite of a chunk: %v", r.Body)
	}
	if r := e.call("codex", "POST", run+"/finish", map[string]any{"status": "done"}); r.code() != "invalid" {
		t.Errorf("bad status: %v", r.Body)
	}
	fin := e.call("codex", "POST", run+"/finish", map[string]any{"status": "completed", "turns": 3, "head_sha": sha('a')})
	want := sha256.Sum256([]byte("line one\nline two\n"))
	got := fin.Body["transcript_sha256"]
	if fin.Status != 200 || fin.Body["status"] != "completed" || num(fin.Body["transcript_bytes"]) != 18 || got != hex.EncodeToString(want[:]) {
		t.Fatalf("finish: %v", fin.Body)
	}
	if r := e.call("codex", "POST", run+"/transcript", map[string]any{"seq": 2, "body": "late\n"}); r.code() != "run_finished" {
		t.Errorf("wrote to a finished run: %v", r.Body)
	}
	read := e.call("jesse", "GET", run+"?after_seq=0", nil).Body
	if cs := read["chunks"].([]any); len(cs) != 1 || cs[0].(map[string]any)["body"] != "line two\n" {
		t.Errorf("paged read: %v", read)
	}
	ctx := context.Background()
	if _, err := e.pool.Exec(ctx, `UPDATE runs SET reason='edited' WHERE id=$1`, num(r.Body["id"])); err == nil {
		t.Error("a finished run must be frozen")
	}
	if _, err := e.pool.Exec(ctx, `DELETE FROM run_chunks`); err == nil {
		t.Error("run_chunks must be append-only")
	}
	var evSum string
	if err := e.pool.QueryRow(ctx, `SELECT data->>'transcript_sha256' FROM events WHERE action='run.finished' ORDER BY seq DESC LIMIT 1`).Scan(&evSum); err != nil || evSum != got {
		t.Errorf("run.finished carries %q, run has %v (%v)", evSum, got, err)
	}

	// A run left open when its lease ends is closed as lost by the next run on the task; a gap marks truncation.
	r2 := e.call("codex", "POST", p+"/runs", map[string]any{"fence": f, "adapter": "claude-code"})
	e.call("codex", "POST", "/runs/"+itoa(num(r2.Body["id"]))+"/transcript", map[string]any{"seq": 1, "body": "gap before me\n"})
	r3 := e.call("codex", "POST", p+"/runs", map[string]any{"fence": f, "adapter": "claude-code"})
	runs := e.call("jesse", "GET", p, nil).Body["runs"].([]any)
	if len(runs) != 3 || runs[1].(map[string]any)["status"] != "lost" || runs[2].(map[string]any)["status"] != "running" {
		t.Fatalf("runs after a new start: %v", runs)
	}
	e.call("codex", "POST", "/runs/"+itoa(num(r3.Body["id"]))+"/transcript", map[string]any{"seq": 1, "body": "gap\n"})
	if fin := e.call("codex", "POST", "/runs/"+itoa(num(r3.Body["id"]))+"/finish", map[string]any{"status": "failed"}); fin.Body["truncated"] != true {
		t.Errorf("a transcript with a missing chunk must be marked truncated: %v", fin.Body)
	}

	cs := e.mcpSession("jesse")
	if out, isErr := callTool(t, cs, "get_task", map[string]any{"task_id": id}); isErr || len(out["runs"].([]any)) != 3 {
		t.Errorf("get_task runs: %v", out)
	}
	if v := e.call("codex", "GET", "/events/verify", nil); v.Body["ok"] != true {
		t.Errorf("chain: %v", v.Body)
	}
}
