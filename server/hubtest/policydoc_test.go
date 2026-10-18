package hubtest

import (
	"context"
	"encoding/json"
	"testing"
)

// TestPolicyDocEditAndFailClosed: policies are versioned tenant data edited by humans only, with optimistic
// concurrency and validation; disabling a rule takes effect on the next call; a tenant with no policy fails closed.
func TestPolicyDocEditAndFailClosed(t *testing.T) {
	e := setup(t)
	e.addMachine("lab")
	e.grantArgv(t, []string{"rm"}, []string{"echo"})
	cur := e.call("jesse", "GET", "/policy/doc", nil)
	if cur.Status != 200 || num(cur.Body["version"]) != 1 {
		t.Fatalf("seeded doc: %v", cur.Body)
	}
	doc := cur.Body["doc"].(map[string]any)
	if r := e.call("claude-code", "PUT", "/policy/doc", map[string]any{"base_version": 1, "doc": doc, "reason": "agent edit"}); r.code() != "humans_only" {
		t.Fatalf("agent edited the policy: %v", r.Body)
	}
	bad := map[string]any{"rules": []any{map[string]any{"id": "x1", "effect": "maybe"}}}
	if r := e.call("jesse", "PUT", "/policy/doc", map[string]any{"base_version": 1, "doc": bad, "reason": "bad doc"}); r.Status != 400 {
		t.Fatalf("invalid doc accepted: %v", r.Body)
	}
	cs := e.mcpSession("claude-code")
	rm := map[string]any{"machine": "lab", "argv": []string{"rm", "-rf", "/etc"}}
	if out, isErr := callTool(t, cs, "exec", rm); !isErr || errCode(out) != "policy_denied" {
		t.Fatalf("v1 must deny: %v", out)
	}
	for _, r := range doc["rules"].([]any) {
		if m := r.(map[string]any); m["id"] == "destructive-root-delete" {
			m["enabled"] = false
		}
	}
	if r := e.call("jesse", "PUT", "/policy/doc", map[string]any{"base_version": 1, "doc": doc, "reason": "test: disable root delete"}); num(r.Body["version"]) != 2 {
		t.Fatalf("edit: %v", r.Body)
	}
	if r := e.call("jesse", "PUT", "/policy/doc", map[string]any{"base_version": 1, "doc": doc, "reason": "lost update"}); r.Status != 409 {
		t.Fatalf("stale base_version accepted: %v", r.Body)
	}
	if out, isErr := callTool(t, cs, "exec", rm); isErr {
		t.Fatalf("v2 disabled the rule; call should reach the bridge: %v", out)
	}
	if r := e.call("jesse", "POST", "/policy/rollback", map[string]any{"to_version": 1, "base_version": 2, "reason": "restore"}); num(r.Body["version"]) != 3 {
		t.Fatalf("rollback: %v", r.Body)
	}
	if out, isErr := callTool(t, cs, "exec", rm); !isErr || errCode(out) != "policy_denied" {
		t.Fatalf("v3 (= v1) must deny again: %v", out)
	}
	vs := e.call("jesse", "GET", "/policy/versions", nil)
	if l, _ := vs.Body["list"].([]any); len(l) != 3 {
		t.Fatalf("versions: %v", vs.Body)
	}
	dec := e.call("jesse", "GET", "/policy/decisions?limit=10", nil)
	if l, _ := dec.Body["list"].([]any); len(l) != 3 {
		t.Fatalf("decision log: %v", dec.Body)
	}
	try := e.call("claude-code", "POST", "/policy/decide", map[string]any{"tool": "Bash", "command": "ssh admin@wi-router-1 uptime", "dry_run": true})
	if try.Body["allow"] != false || !json.Valid([]byte(fmtAny(try.Body["decision"]))) {
		t.Fatalf("WI geo block via decide: %v", try.Body)
	}
	ctx := context.Background()
	_, _ = e.pool.Exec(ctx, `INSERT INTO tenants(id,name) VALUES('acme','Acme')`)
	_, _ = e.pool.Exec(ctx, `UPDATE machines SET tenant_id='acme' WHERE id='lab'`)
	if out, isErr := callTool(t, cs, "exec", map[string]any{"machine": "lab", "argv": []string{"echo", "hi"}}); !isErr || errCode(out) != "policy_unavailable" {
		t.Fatalf("tenant without a policy must fail closed: %v", out)
	}
	t.Logf("agent edit refused; invalid doc refused; v2 disable took effect; stale base 409; rollback v3 denies again; WI geo block; tenant without policy -> policy_unavailable")
}
