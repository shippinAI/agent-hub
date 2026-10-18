package hubtest

import (
	"strings"
	"testing"
)

// Every machine call lands in the action feed: denied, started (policy + approver), finished (exit, output),
// and the MCP tool call itself, with secrets masked before storage (39).
func TestActionFeedMachineCalls(t *testing.T) {
	e := setup(t)
	e.addMachine("lab")
	cs := e.mcpSession("claude-code")
	secret := "sk-abcdefghijklmnop1234"
	if out, isErr := callTool(t, cs, "exec", map[string]any{"machine": "lab", "argv": []string{"echo", "hi"}}); !isErr {
		t.Fatalf("exec without grant: %v", out)
	}
	out, _ := callTool(t, cs, "request_access", map[string]any{"machine": "lab", "capability": "exec", "selector": map[string]any{"argv_prefix": [][]string{{"echo"}}},
		"sandbox_level": "workspace-write", "duration": "1h", "reason": "Run echo to prove the action feed records machine calls."})
	if r := e.call("jesse", "POST", "/access-requests/"+reqID(t, out)+"/approve", nil); r.Status != 200 {
		t.Fatalf("approve: %v", r.Body)
	}
	if out, isErr := callTool(t, cs, "exec", map[string]any{"machine": "lab", "argv": []string{"echo", "hi", "--api-key", secret}}); isErr {
		t.Fatalf("exec: %v", out)
	}
	r := e.call("jesse", "GET", "/actions?category=proc", nil)
	acts, _ := r.Body["actions"].([]any)
	kinds := map[string]map[string]any{}
	for _, a := range acts {
		m := a.(map[string]any)
		kinds[m["kind"].(string)] = m
	}
	den, start, fin := kinds["call.denied"], kinds["call.started"], kinds["call.finished"]
	if den == nil || start == nil || fin == nil {
		t.Fatalf("missing kinds: %v", r.Body)
	}
	if p := den["policy"].(map[string]any); p["decision"] != "deny" || p["code"] != "grant_required" {
		t.Errorf("denied policy: %v", p)
	}
	if p := start["policy"].(map[string]any); p["decision"] != "allow" || p["approver"] != "jesse" || p["grant_id"] == "" {
		t.Errorf("started policy: %v", p)
	}
	if start["call_id"] != fin["call_id"] || fin["status"] != "ok" || num(fin["exit_code"]) != 0 || start["actor"] != "claude-code" {
		t.Errorf("finished: %v", fin)
	}
	all := fmtAny(r.Body)
	if strings.Contains(all, secret) || !strings.Contains(all, "[redacted]") {
		t.Errorf("secret not redacted: %s", all)
	}
	tools := fmtAny(e.call("jesse", "GET", "/actions?category=tool", nil).Body)
	if !strings.Contains(tools, `"tool":"exec"`) || !strings.Contains(tools, `"tool":"request_access"`) || strings.Contains(tools, secret) {
		t.Errorf("tool events: %s", tools)
	}
	if r := e.call("claude-code", "GET", "/actions", nil); r.code() != "humans_only" {
		t.Errorf("agents read the feed: %v", r.Body)
	}
	a2 := e.call("jesse", "GET", "/actions/a2ui?limit=5", nil)
	msgs, _ := a2.Body["messages"].([]any)
	if len(msgs) != 3 || !strings.Contains(fmtAny(msgs[0]), `"createSurface"`) || !strings.Contains(fmtAny(msgs[2]), `"/actions"`) {
		t.Errorf("a2ui: %v", a2.Body)
	}
}

// A disabled category records nothing; turning it back on resumes; agents cannot change categories.
func TestActionFeedCategorySwitch(t *testing.T) {
	e := setup(t)
	cs := e.mcpSession("codex")
	body := map[string]any{"enabled": false, "retain_days": 30, "archive": true, "capture_output": true, "max_output_bytes": 65536}
	if r := e.call("codex", "PUT", "/audit/categories/tool", body); r.Status != 403 {
		t.Fatalf("agent changed audit config: %v", r.Body)
	}
	if r := e.call("jesse", "PUT", "/audit/categories/tool", body); r.Status != 200 || r.Body["enabled"] != false {
		t.Fatalf("disable: %v", r.Body)
	}
	e.api.Keep.Lookup(t.Context(), "tool") // cache refresh happens on Put; this just exercises the path
	callTool(t, cs, "list_ready", map[string]any{})
	if n := len(e.callActions("tool")); n != 0 {
		t.Fatalf("disabled category recorded %d events", n)
	}
	body["enabled"] = true
	e.call("jesse", "PUT", "/audit/categories/tool", body)
	callTool(t, cs, "list_ready", map[string]any{})
	if n := len(e.callActions("tool")); n != 1 {
		t.Fatalf("re-enabled category recorded %d events", n)
	}
}

func (e *env) callActions(cat string) []any {
	acts, _ := e.call("jesse", "GET", "/actions?category="+cat, nil).Body["actions"].([]any)
	return acts
}
