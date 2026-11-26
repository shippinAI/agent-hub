package conformance

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"slices"
	"testing"
)

// Run runs every case against the target. Cases are independent and each uses a project of its own.
func Run(t *testing.T, tg Target) {
	for _, c := range []struct {
		name string
		fn   func(*testing.T, *hub)
	}{
		{"errors", errorsCase},
		{"lease", leaseCase},
		{"fencing", fencingCase},
		{"claim_next", claimNextCase},
		{"idempotency", idempotencyCase},
		{"versions", versionsCase},
		{"review_lane", reviewLaneCase},
		{"evidence", evidenceCase},
		{"runs", runsCase},
		{"usage", usageCase},
		{"events", eventsCase},
		{"mcp", mcpCase},
	} {
		t.Run(c.name, func(t *testing.T) { c.fn(t, newHub(t, tg)) })
	}
}

// errorsCase: errors are {error:{code,message}} with the documented HTTP status.
func errorsCase(t *testing.T, h *hub) {
	if r := h.call("", "GET", "/tasks", nil); r.Status != 401 || r.code() != "unauthenticated" {
		t.Errorf("no token: %d %v", r.Status, r.Body)
	}
	if r := h.call(h.tg.AgentA, "GET", "/tasks/999999999", nil); r.Status != 404 || r.code() != "not_found" {
		t.Errorf("missing task: %d %v", r.Status, r.Body)
	}
	if r := h.call(h.tg.AgentA, "POST", "/tasks", map[string]any{"project": h.project, "title": ""}); r.Status != 400 || r.code() != "invalid" {
		t.Errorf("empty title: %d %v", r.Status, r.Body)
	}
	if r := h.call(h.tg.AgentA, "GET", "/me", nil); r.Status != 200 {
		t.Errorf("me: %d %v", r.Status, r.Body)
	}
}

// leaseCase: one holder at a time; re-claim by the holder returns the same lease; heartbeat, release.
func leaseCase(t *testing.T, h *hub) {
	id := h.task("lease", nil)
	p := fmt.Sprintf("/tasks/%d", id)
	f := h.claim(h.tg.AgentA, id)
	if r := h.call(h.tg.AgentA, "POST", p+"/claim", map[string]any{}); r.Body["already_held"] != true || num(r.Body["fence"]) != f {
		t.Errorf("re-claim by holder: %v", r.Body)
	}
	if r := h.call(h.tg.AgentB, "POST", p+"/claim", map[string]any{}); r.Status != 409 || r.code() != "held_by_other" {
		t.Errorf("second claimer: %d %v", r.Status, r.Body)
	}
	if r := h.call(h.tg.AgentA, "POST", p+"/heartbeat", map[string]any{"fence": f, "progress": map[string]any{"pct": 50}}); r.Status != 200 {
		t.Errorf("heartbeat: %d %v", r.Status, r.Body)
	}
	if r := h.call(h.tg.AgentB, "POST", p+"/heartbeat", map[string]any{"fence": f}); r.Status != 409 {
		t.Errorf("heartbeat by non-holder: %d %v", r.Status, r.Body)
	}
	if r := h.call(h.tg.AgentA, "POST", p+"/release", map[string]any{"fence": f, "reason": "done for now"}); r.Status != 200 {
		t.Errorf("release: %d %v", r.Status, r.Body)
	}
	if task := h.get(id); task["holder"] != nil || task["state"] != "ready" {
		t.Errorf("after release: %v", task)
	}
	if f2 := h.claim(h.tg.AgentB, id); f2 <= f {
		t.Errorf("fence must grow with each lease: %d then %d", f, f2)
	}
}

// fencingCase: a lease taken away (force-release by a human) leaves the old fence dead for every write.
func fencingCase(t *testing.T, h *hub) {
	id := h.task("fencing", nil)
	p := fmt.Sprintf("/tasks/%d", id)
	f := h.claim(h.tg.AgentA, id)
	if r := h.call(h.tg.AgentA, "POST", p+"/force-release", map[string]any{"reason": "x"}); r.Status != 403 {
		t.Errorf("an agent may not force-release: %d %v", r.Status, r.Body)
	}
	if r := h.call(h.tg.Human, "POST", p+"/force-release", map[string]any{"reason": "stuck"}); r.Status != 200 {
		t.Fatalf("force-release: %d %v", r.Status, r.Body)
	}
	h.claim(h.tg.AgentB, id)
	for _, op := range []string{"heartbeat", "complete", "release"} {
		if r := h.call(h.tg.AgentA, "POST", p+"/"+op, map[string]any{"fence": f}); r.Status != 409 ||
			!slices.Contains([]string{"stale_fence", "not_holder"}, r.code()) {
			t.Errorf("%s with a dead fence: %d %v", op, r.Status, r.Body)
		}
	}
}

// claimNextCase: claim_next takes the best ready task (assigned first, then priority), skips held and blocked ones.
func claimNextCase(t *testing.T, h *hub) {
	low := h.task("low", map[string]any{"priority": 3})
	high := h.task("high", map[string]any{"priority": 0})
	blocked := h.task("blocked", map[string]any{"priority": 0})
	if r := h.call(h.tg.Human, "POST", fmt.Sprintf("/tasks/%d/deps", blocked), map[string]any{"depends_on": low}); r.Status != 200 {
		t.Fatalf("dep: %d %v", r.Status, r.Body)
	}
	mine := h.task("assigned", map[string]any{"priority": 4, "assignee": principal(t, h, h.tg.AgentB)})
	next := func(tok string) int64 {
		r := h.call(tok, "POST", "/claim-next", map[string]any{"project": h.project})
		if r.Body["claimed"] != true {
			return 0
		}
		task, _ := r.Body["task"].(map[string]any)
		return num(task["id"])
	}
	if got := next(h.tg.AgentB); got != mine {
		t.Errorf("assigned work first: got #%d want #%d", got, mine)
	}
	if got := next(h.tg.AgentA); got != high {
		t.Errorf("then priority: got #%d want #%d", got, high)
	}
	if got := next(h.tg.AgentA); got != low {
		t.Errorf("blocked task skipped: got #%d want #%d", got, low)
	}
	if got := next(h.tg.AgentB); got != 0 {
		t.Errorf("nothing left (the rest is blocked or held): got #%d", got)
	}
}

func principal(t *testing.T, h *hub, tok string) string {
	r := h.call(tok, "GET", "/me", nil)
	if p, ok := r.Body["principal"].(map[string]any); ok {
		return p["id"].(string)
	}
	id, _ := r.Body["id"].(string)
	if id == "" {
		t.Fatalf("me: %v", r.Body)
	}
	return id
}

// idempotencyCase: the same key and body replays the stored answer; a different body under the key is refused.
func idempotencyCase(t *testing.T, h *hub) {
	key := "Idempotency-Key: conf-" + h.project
	body := map[string]any{"project": h.project, "title": "once"}
	a := h.call(h.tg.AgentA, "POST", "/tasks", body, key)
	b := h.call(h.tg.AgentA, "POST", "/tasks", body, key)
	if a.Status != 200 || b.Status != 200 || num(a.Body["id"]) != num(b.Body["id"]) || b.Header.Get("Idempotent-Replayed") != "true" {
		t.Errorf("replay: %v / %v (%q)", a.Body, b.Body, b.Header.Get("Idempotent-Replayed"))
	}
	body["title"] = "twice"
	if r := h.call(h.tg.AgentA, "POST", "/tasks", body, key); r.Status != 422 || r.code() != "idempotency_mismatch" {
		t.Errorf("same key, other body: %d %v", r.Status, r.Body)
	}
	if r := h.call(h.tg.AgentB, "POST", "/tasks", body, key); r.Status != 200 || num(r.Body["id"]) == num(a.Body["id"]) {
		t.Errorf("keys are per principal: %d %v", r.Status, r.Body)
	}
}

// versionsCase: updates need the current version (body or If-Match); a stale one gets version_conflict.
func versionsCase(t *testing.T, h *hub) {
	id := h.task("versions", nil)
	p := fmt.Sprintf("/tasks/%d", id)
	v := num(h.get(id)["version"])
	if r := h.call(h.tg.Human, "PATCH", p, map[string]any{"title": "v2", "version": v}); r.Status != 200 || num(r.Body["version"]) != v+1 {
		t.Fatalf("update: %d %v", r.Status, r.Body)
	}
	if r := h.call(h.tg.Human, "PATCH", p, map[string]any{"title": "v3", "version": v}); r.Status != 409 || r.code() != "version_conflict" {
		t.Errorf("stale version: %d %v", r.Status, r.Body)
	}
	if r := h.call(h.tg.Human, "PATCH", p, map[string]any{"title": "v3"}, fmt.Sprintf(`If-Match: "%d"`, v+1)); r.Status != 200 {
		t.Errorf("If-Match: %d %v", r.Status, r.Body)
	}
}

// reviewLaneCase: completing the build lane moves the task to review; the review lane needs someone else.
func reviewLaneCase(t *testing.T, h *hub) {
	id := h.task("review", nil)
	p := fmt.Sprintf("/tasks/%d", id)
	f := h.claim(h.tg.Human, id)
	if r := h.call(h.tg.Human, "POST", p+"/complete", map[string]any{"fence": f, "summary": "built"}); r.Status != 200 {
		t.Fatalf("complete: %d %v", r.Status, r.Body)
	}
	if task := h.get(id); task["state"] != "review" || task["holder"] != nil {
		t.Fatalf("after build: %v", task)
	}
	if r := h.call(h.tg.Human, "POST", p+"/claim", map[string]any{}); r.Status != 409 || r.code() != "distinct_required" {
		t.Errorf("the builder reviewing its own work: %d %v", r.Status, r.Body)
	}
}

// evidenceCase: evidence is append-only and stamped with the fence it was given under; a stale fence is refused,
// and evidence from an earlier lease stays on record under its old fence.
func evidenceCase(t *testing.T, h *hub) {
	id := h.task("evidence", map[string]any{"acceptance": []string{"tests pass"}})
	p := fmt.Sprintf("/tasks/%d", id)
	f := h.claim(h.tg.AgentA, id)
	if r := h.call(h.tg.AgentA, "POST", p+"/evidence", map[string]any{"fence": f, "kind": "test.unit", "status": "fail", "summary": "1 failing"}); r.Status != 200 || num(r.Body["fence"]) != f {
		t.Fatalf("evidence: %d %v", r.Status, r.Body)
	}
	if r := h.call(h.tg.AgentA, "POST", p+"/evidence", map[string]any{"fence": f, "kind": "test.unit", "status": "maybe"}); r.Status != 400 {
		t.Errorf("status must be pass or fail: %d %v", r.Status, r.Body)
	}
	h.call(h.tg.AgentA, "POST", p+"/release", map[string]any{"fence": f, "reason": "next"})
	f2 := h.claim(h.tg.AgentB, id)
	if r := h.call(h.tg.AgentB, "POST", p+"/evidence", map[string]any{"fence": f, "kind": "test.unit", "status": "pass"}); r.Status != 409 {
		t.Errorf("evidence under a stale fence: %d %v", r.Status, r.Body)
	}
	h.call(h.tg.AgentB, "POST", p+"/evidence", map[string]any{"fence": f2, "kind": "test.unit", "status": "pass"})
	ev, _ := h.call(h.tg.Human, "GET", p, nil).Body["evidence"].([]any)
	if len(ev) != 2 || num(ev[0].(map[string]any)["fence"]) == num(ev[1].(map[string]any)["fence"]) {
		t.Errorf("both rows kept, each under its own lease: %v", ev)
	}
}

// runsCase: a run is opened under the lease, its transcript is append-only and retry-safe, finishing hashes it.
func runsCase(t *testing.T, h *hub) {
	id := h.task("runs", nil)
	f := h.claim(h.tg.AgentA, id)
	r := h.call(h.tg.AgentA, "POST", fmt.Sprintf("/tasks/%d/runs", id), map[string]any{"fence": f, "adapter": "conformance"})
	if r.Status != 200 || r.Body["status"] != "running" {
		t.Fatalf("start run: %d %v", r.Status, r.Body)
	}
	run := fmt.Sprintf("/runs/%d", num(r.Body["id"]))
	chunks := []string{"{\"a\":1}\n", "{\"b\":2}\n"}
	for i, c := range chunks {
		if r := h.call(h.tg.AgentA, "POST", run+"/transcript", map[string]any{"seq": i, "body": c}); r.Status != 200 {
			t.Fatalf("chunk %d: %d %v", i, r.Status, r.Body)
		}
	}
	if r := h.call(h.tg.AgentA, "POST", run+"/transcript", map[string]any{"seq": 0, "body": chunks[0]}); r.Body["duplicate"] != true {
		t.Errorf("resent chunk: %v", r.Body)
	}
	if r := h.call(h.tg.AgentA, "POST", run+"/transcript", map[string]any{"seq": 0, "body": "other"}); r.Status != 409 || r.code() != "chunk_conflict" {
		t.Errorf("rewritten chunk: %d %v", r.Status, r.Body)
	}
	if r := h.call(h.tg.AgentB, "POST", run+"/transcript", map[string]any{"seq": 2, "body": "x"}); r.Status != 403 {
		t.Errorf("another principal writing the transcript: %d %v", r.Status, r.Body)
	}
	sum := sha256.Sum256([]byte(chunks[0] + chunks[1]))
	fin := h.call(h.tg.AgentA, "POST", run+"/finish", map[string]any{"status": "completed", "turns": 2})
	if fin.Status != 200 || fin.Body["transcript_sha256"] != hex.EncodeToString(sum[:]) || fin.Body["truncated"] != false {
		t.Errorf("finish: %d %v", fin.Status, fin.Body)
	}
	if r := h.call(h.tg.AgentA, "POST", run+"/transcript", map[string]any{"seq": 2, "body": "late"}); r.Status != 409 || r.code() != "run_finished" {
		t.Errorf("writing to a finished run: %d %v", r.Status, r.Body)
	}
	got := h.call(h.tg.Human, "GET", run, nil).Body
	if cs, _ := got["chunks"].([]any); len(cs) != 2 {
		t.Errorf("read transcript: %v", got)
	}
}

// usageCase: usage is recorded per caller, deduplicated by external_ref, and summed per task.
func usageCase(t *testing.T, h *hub) {
	id := h.task("usage", nil)
	f := h.claim(h.tg.AgentA, id)
	in := map[string]any{"task_id": id, "fence": f, "provider_id": "anthropic-api", "model": "m", "billing_kind": "api_key", "source": "stream",
		"input_tokens": 100, "cache_read_tokens": 40, "output_tokens": 10, "cost_micros": 2500, "cost_basis": "client_estimate",
		"external_ref": "conf/" + h.project}
	a := h.call(h.tg.AgentA, "POST", "/usage", in)
	b := h.call(h.tg.AgentA, "POST", "/usage", in)
	bu, _ := b.Body["usage"].(map[string]any)
	if a.Status != 200 || bu["duplicate"] != true {
		t.Errorf("dedupe: %v / %v", a.Body, b.Body)
	}
	in["cache_read_tokens"] = 500
	if r := h.call(h.tg.AgentA, "POST", "/usage", in); r.Status != 400 {
		t.Errorf("cache reads are part of input, so they cannot exceed it: %d %v", r.Status, r.Body)
	}
	u, _ := h.call(h.tg.Human, "GET", fmt.Sprintf("/tasks/%d", id), nil).Body["usage"].(map[string]any)
	if num(u["estimate_micros"]) != 2500 || num(u["input_tokens"]) != 100 {
		t.Errorf("task usage: %v", u)
	}
}

// eventsCase: every write is in the hash chain; the chain verifies.
func eventsCase(t *testing.T, h *hub) {
	id := h.task("events", nil)
	f := h.claim(h.tg.AgentA, id)
	h.call(h.tg.AgentA, "POST", fmt.Sprintf("/tasks/%d/notes", id), map[string]any{"kind": "note", "body": "hello", "fence": f})
	evs, _ := h.call(h.tg.Human, "GET", fmt.Sprintf("/events?task_id=%d", id), nil).Body["events"].([]any)
	var actions []string
	for _, e := range evs {
		actions = append(actions, e.(map[string]any)["action"].(string))
	}
	for _, want := range []string{"task.created", "task.claimed", "note.added"} {
		if !slices.Contains(actions, want) {
			t.Errorf("event %s missing from %v", want, actions)
		}
	}
	if r := h.call(h.tg.Human, "GET", "/events/verify", nil); r.Body["ok"] != true {
		t.Errorf("chain: %v", r.Body)
	}
}

// mcpCase: the MCP surface offers the core tools and shares the REST semantics.
func mcpCase(t *testing.T, h *hub) {
	cs := h.mcp(h.tg.AgentA)
	id := h.task("over mcp", nil)
	out, isErr := h.tool(cs, "claim", map[string]any{"task_id": id})
	if isErr || num(out["fence"]) == 0 {
		t.Fatalf("claim over MCP: %v", out)
	}
	if out, isErr := h.tool(h.mcp(h.tg.AgentB), "claim", map[string]any{"task_id": id}); !isErr {
		t.Errorf("second claimer over MCP must get an error: %v", out)
	}
	if out, isErr := h.tool(cs, "get_task", map[string]any{"task_id": id}); isErr || out["task"] == nil {
		t.Errorf("get_task: %v", out)
	}
	if out, isErr := h.tool(cs, "complete", map[string]any{"task_id": id, "fence": out["fence"], "text": "ok"}); isErr {
		t.Errorf("complete over MCP: %v", out)
	}
}
