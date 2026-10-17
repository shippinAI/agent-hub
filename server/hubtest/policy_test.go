package hubtest

import "testing"

func TestPolicyAndSafetyRules(t *testing.T) {
	e := setup(t)
	id := e.newTask("jesse", "policy task", nil)
	p := "/tasks/" + itoa(id)
	e.call("claude-code", "POST", p+"/claim", map[string]any{})
	if r := e.call("codex", "POST", p+"/force-release", map[string]any{"reason": "mine now"}); r.code() != "humans_only" {
		t.Errorf("agent force-release: %d %s", r.Status, r.code())
	}
	if r := e.call("jesse", "POST", p+"/force-release", map[string]any{"reason": "x"}); r.code() != "invalid" {
		t.Errorf("force-release needs a reason: %s", r.code())
	}
	if r := e.call("jesse", "POST", p+"/force-release", map[string]any{"reason": "agent is stuck"}); r.Status != 200 || r.Body["holder"] != nil {
		t.Errorf("human force-release: %v", r.Body)
	}
	if r := e.call("codex", "POST", "/policies", map[string]any{"name": "grab", "subject": "*", "action": "*", "effect": "allow", "priority": 999}); r.code() != "humans_only" {
		t.Errorf("agent edits policy: %s", r.code())
	}
	me := e.call("jesse", "GET", "/me", nil).Body["principal"].(map[string]any)
	if r := e.call("jesse", "PUT", "/principals/jesse", map[string]any{"display_name": "J", "kind": "human", "is_human": true,
		"roles": []string{"admin", "reviewer", "observer"}, "enabled": true, "version": me["version"]}); r.code() != "self_escalation" {
		t.Errorf("self role change: %s", r.code())
	}
	h := e.newTask("jesse", "only a human should do this", map[string]any{"labels": []string{"human-only"}})
	if r := e.call("cursor", "POST", "/tasks/"+itoa(h)+"/claim", map[string]any{}); r.code() != "policy_denied" {
		t.Errorf("agent claims human-only task: %s", r.code())
	}
	// reviewer != author is workflow data (lane distinct_from_previous)
	w := e.newTask("jesse", "review flow", nil)
	wp := "/tasks/" + itoa(w)
	l := e.call("codex", "POST", wp+"/claim", map[string]any{})
	if r := e.call("codex", "POST", wp+"/complete", map[string]any{"fence": l.Body["fence"], "summary": "built"}); r.Body["state"] != "review" {
		t.Fatalf("complete -> review: %v", r.Body)
	}
	if r := e.call("codex", "POST", wp+"/claim", map[string]any{}); r.code() != "distinct_required" {
		t.Errorf("author claims own review: %s", r.code())
	}
	if r := e.call("cursor", "POST", wp+"/claim", map[string]any{}); r.code() != "policy_denied" {
		t.Errorf("non-reviewer claims review: %s", r.code())
	}
	rv := e.call("clyffy", "POST", wp+"/claim", map[string]any{})
	if rv.Status != 200 || taskOf(rv)["state"] != "reviewing" {
		t.Fatalf("reviewer claim: %v", rv.Body)
	}
	if r := e.call("clyffy", "POST", wp+"/complete", map[string]any{"fence": rv.Body["fence"], "summary": "LGTM"}); r.Body["state"] != "done" {
		t.Errorf("review complete -> done: %v", r.Body)
	}
	// WIP limit is data (wip_limit_default=2)
	for i := 0; i < 3; i++ {
		e.newTask("jesse", "wip", nil)
	}
	e.call("cursor", "POST", "/claim-next", map[string]any{})
	e.call("cursor", "POST", "/claim-next", map[string]any{})
	if r := e.call("cursor", "POST", "/claim-next", map[string]any{}); r.code() != "wip_limit" {
		t.Errorf("third claim over WIP 2: %s", r.code())
	}
}
