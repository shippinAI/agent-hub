package hubtest

import "testing"

// TestScoresAndRouting: run outcomes come from what happened to the task next (accepted by the next lane,
// reworked by the same lane, or not completed), and with routing on claim_next steers a lane's work away
// from a principal whose success rate there is low, except work assigned to it.
func TestScoresAndRouting(t *testing.T) {
	e := setup(t)
	// work runs one lease: claim, open a run, finish it with status, complete when it completed.
	work := func(who string, id int64, status string, cost int64) {
		t.Helper()
		p := "/tasks/" + itoa(id)
		cl := e.call(who, "POST", p+"/claim", map[string]any{})
		if cl.Status != 200 {
			t.Fatalf("%s claims #%d: %v", who, id, cl.Body)
		}
		f := num(cl.Body["fence"])
		r := e.call(who, "POST", p+"/runs", map[string]any{"fence": f, "adapter": "codex"})
		if r.Status != 200 {
			t.Fatalf("run on #%d: %v", id, r.Body)
		}
		if cost > 0 {
			e.usage(who, map[string]any{"task_id": id, "fence": f, "provider_id": "openai-api", "model": "gpt-test", "billing_kind": "api_key",
				"source": "stream", "input_tokens": 10, "output_tokens": 1, "cost_micros": cost, "cost_basis": "client_estimate"})
		}
		e.call(who, "POST", "/runs/"+itoa(num(r.Body["id"]))+"/finish", map[string]any{"status": status})
		if status == "completed" {
			if c := e.call(who, "POST", p+"/complete", map[string]any{"fence": f, "summary": "done"}); c.Status != 200 {
				t.Fatalf("complete #%d: %v", id, c.Body)
			}
		} else {
			e.call(who, "POST", p+"/handoff", map[string]any{"fence": f, "note": "gave up", "to": "jesse"})
		}
	}
	accepted := e.newTask("jesse", "accepted", nil)
	work("codex", accepted, "completed", 4000)
	work("jesse", accepted, "completed", 0) // the review lane finishes it

	reworked := e.newTask("jesse", "reworked", nil)
	work("codex", reworked, "completed", 0)
	task := taskOf(e.call("jesse", "GET", "/tasks/"+itoa(reworked), nil))
	if r := e.call("jesse", "PATCH", "/tasks/"+itoa(reworked), map[string]any{"state": "ready", "version": task["version"]}); r.Status != 200 {
		t.Fatalf("send back: %v", r.Body)
	}
	work("codex", reworked, "failed", 0)
	work("codex", e.newTask("jesse", "handed off", nil), "handed_off", 0)

	sc := e.call("jesse", "GET", "/scores?group_by=principal", nil).Body["scores"].([]any)
	var codex, reviewer map[string]any
	for _, s := range sc {
		if m := s.(map[string]any); m["key"] == "codex" {
			codex = m
		} else if m["key"] == "jesse" {
			reviewer = m
		}
	}
	if codex == nil || num(codex["runs"]) != 4 || num(codex["accepted"]) != 1 || num(codex["reworked"]) != 1 || num(codex["failed"]) != 1 ||
		num(codex["handed_off"]) != 1 || codex["success_pct"] != 25.0 || num(codex["cost_per_accepted_micros"]) != 4000 {
		t.Fatalf("codex score: %v", codex)
	}
	if reviewer == nil || num(reviewer["pending"]) != 1 || reviewer["success_pct"] != nil {
		t.Errorf("a run nobody has judged yet is pending, not scored: %v", reviewer)
	}
	if lanes := e.call("jesse", "GET", "/scores?group_by=lane&principal=codex", nil).Body["scores"].([]any); len(lanes) != 1 ||
		lanes[0].(map[string]any)["key"] != "in_progress" {
		t.Errorf("by lane: %v", lanes)
	}
	if r := e.call("jesse", "GET", "/scores?group_by=mood", nil); r.code() != "invalid" {
		t.Errorf("bad group_by: %v", r.Body)
	}

	// Routing off (the default): codex still gets build work.
	next := e.newTask("jesse", "next build", nil)
	if r := e.call("codex", "POST", "/claim-next", map[string]any{}); r.Body["claimed"] != true || num(taskOf(r)["id"]) != next {
		t.Fatalf("routing off: %v", r.Body)
	}
	f := num(e.call("codex", "GET", "/tasks/"+itoa(next), nil).Body["task"].(map[string]any)["fence"])
	e.call("codex", "POST", "/tasks/"+itoa(next)+"/release", map[string]any{"fence": f, "reason": "test"})

	// Routing on: below 50% over at least 3 scored runs, codex is steered off the build lane; others are not.
	e.setSetting("route_min_runs", 3)
	r := e.call("codex", "POST", "/claim-next", map[string]any{})
	if r.Body["claimed"] != false || len(r.Body["routed_away"].([]any)) != 1 {
		t.Fatalf("routing on: %v", r.Body)
	}
	if r := e.call("claude-code", "POST", "/claim-next", map[string]any{}); r.Body["claimed"] != true || num(taskOf(r)["id"]) != next {
		t.Errorf("another agent takes the work: %v", r.Body)
	}
	mine := e.newTask("jesse", "assigned to codex", map[string]any{"assignee": "codex"})
	if r := e.call("codex", "POST", "/claim-next", map[string]any{}); r.Body["claimed"] != true || num(taskOf(r)["id"]) != mine {
		t.Errorf("assigned work is never routed away: %v", r.Body)
	}
	e.setSetting("route_min_success_pct", 20)
	other := e.newTask("jesse", "after lowering the bar", nil)
	if r := e.call("codex", "POST", "/claim-next", map[string]any{}); r.Body["claimed"] != true || num(taskOf(r)["id"]) != other {
		t.Errorf("25%% clears a 20%% bar: %v", r.Body)
	}

	cs := e.mcpSession("codex")
	if out, isErr := callTool(t, cs, "run_scores", map[string]any{"mine": true}); isErr || len(out["scores"].([]any)) != 1 {
		t.Errorf("run_scores: %v", out)
	}
	if v := e.call("jesse", "GET", "/events/verify", nil); v.Body["ok"] != true {
		t.Errorf("chain: %v", v.Body)
	}
}
