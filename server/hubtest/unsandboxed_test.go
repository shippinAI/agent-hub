package hubtest

import (
	"context"
	"testing"
	"time"
)

func TestUnsandboxedPolicyResumeAlertsDefaultPreset(t *testing.T) {
	e := setup(t)
	e.addMachine("box")
	cs := e.mcpSession("claude-code")
	ask := map[string]any{"machine": "box", "preset": "experimental-full-access", "reason": "Full access to debug a system service, I will explain each step."}
	if out, _ := callTool(t, cs, "request_access", ask); errCode(out) != "policy_denied" {
		t.Fatalf("unsandboxed without policy: %v", out)
	}
	if r := e.call("claude-code", "PUT", "/sandbox-policies", map[string]any{"unsandboxed_allowed": true}); r.code() != "humans_only" {
		t.Fatalf("agent enabled the policy: %v", r.Body)
	}
	if r := e.call("jesse", "PUT", "/sandbox-policies", map[string]any{"unsandboxed_allowed": true, "acknowledged_risk_ids": []string{"unsandboxed-any-file"}}); r.code() != "risks_not_acknowledged" {
		t.Fatalf("policy without every ack: %v", r.Body)
	}
	all := []string{"unsandboxed-any-file", "unsandboxed-network", "unsandboxed-persist"}
	if r := e.call("jesse", "PUT", "/sandbox-policies", map[string]any{"unsandboxed_allowed": true, "acknowledged_risk_ids": all}); r.Status != 200 {
		t.Fatalf("enable policy: %v", r.Body)
	}
	if r := e.call("jesse", "PUT", "/settings/grant_default_preset", map[string]any{"preset": "experimental-full-access"}); r.code() != "confirm_required" {
		t.Fatalf("full-access default without confirm: %v", r.Body)
	}
	if r := e.call("jesse", "PUT", "/settings/grant_default_preset", map[string]any{"preset": "experimental-full-access", "confirm": "experimental-full-access"}); r.Status != 200 {
		t.Fatalf("full-access default with confirm: %v", r.Body)
	}
	if r := e.call("jesse", "PUT", "/settings/grant_default_preset", map[string]any{"preset": "task-scoped"}); r.Status != 200 {
		t.Fatalf("back to task-scoped: %v", r.Body)
	}
	if r := e.call("jesse", "PUT", "/settings/grant_default_preset", map[string]any{"value": "experimental-full-access", "version": 1}); r.Status == 200 {
		t.Fatalf("generic settings path changed the default preset: %v", r.Body)
	}
	ask["duration"] = "1h"
	out, _ := callTool(t, cs, "request_access", ask)
	ids, _ := out["request_ids"].([]any)
	if len(ids) < 4 {
		t.Fatalf("preset did not expand: %v", out)
	}
	for _, id := range ids {
		if r := e.call("jesse", "POST", "/access-requests/"+id.(string)+"/approve", nil); r.Status != 200 {
			t.Fatalf("approve %v: %v", id, r.Body)
		}
	}
	al := e.call("jesse", "POST", "/resume-alerts/check", map[string]any{"ui_session_id": "s1", "reason": "page_load"})
	if al.Status != 200 {
		t.Fatalf("check: %v", al.Body)
	}
	alerts := e.callList("jesse", "/resume-alerts")
	if len(alerts) < len(ids) {
		t.Fatalf("expected an alert per risky grant, got %d", len(alerts))
	}
	first := alerts[0]["id"].(string)
	if r := e.call("jesse", "POST", "/resume-alerts/"+first+"/continue", nil); r.Status != 200 {
		t.Fatalf("ack: %v", r.Body)
	}
	e.call("jesse", "POST", "/resume-alerts/check", map[string]any{"ui_session_id": "s2", "reason": "reload"})
	if got := e.callList("jesse", "/resume-alerts"); len(got) < len(ids) {
		t.Fatalf("a reload must show the alert again, got %d", len(got))
	}
	for _, a := range e.callList("jesse", "/resume-alerts") {
		e.call("jesse", "POST", "/resume-alerts/"+a["id"].(string)+"/continue", nil)
	}
	if err := e.api.Access.AgentResumed(context.Background(), "claude-code", "probe"); err != nil {
		t.Fatalf("agent resumed: %v", err)
	}
	if len(e.callList("jesse", "/resume-alerts")) == 0 {
		t.Fatal("agent resume raised no alert")
	}
	for _, a := range e.callList("jesse", "/resume-alerts") {
		e.call("jesse", "POST", "/resume-alerts/"+a["id"].(string)+"/continue", nil)
	}
	e.mcpSession("claude-code")
	deadline := time.Now().Add(5 * time.Second)
	for len(e.callList("jesse", "/resume-alerts")) == 0 && time.Now().Before(deadline) {
		time.Sleep(100 * time.Millisecond)
	}
	if len(e.callList("jesse", "/resume-alerts")) == 0 {
		t.Fatal("a new agent MCP session under a full-access grant raised no alert")
	}
	if r := e.call("jesse", "PUT", "/sandbox-policies", map[string]any{"unsandboxed_allowed": false}); num(r.Body["ended_grants"]) == 0 {
		t.Fatalf("disabling the policy ended no grants: %v", r.Body)
	}
	if live := e.callList("jesse", "/grants?live=true"); len(live) != 0 {
		t.Fatalf("grants still live after policy off: %d", len(live))
	}
}
