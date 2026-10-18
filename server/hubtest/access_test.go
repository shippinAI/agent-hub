package hubtest

import (
	"context"
	"testing"
	"time"
)

func (e *env) addMachine(id string) *fakeBridge {
	e.setSetting("bridge_poll_seconds", 1)
	r := e.call("jesse", "POST", "/machines", map[string]any{"id": id, "label": "Lab " + id, "os": "linux"})
	if r.Status != 200 {
		e.t.Fatalf("create machine: %d %v", r.Status, r.Body)
	}
	code := r.Body["enrollment"].(map[string]any)["code"].(string)
	b := e.enrollFake(code)
	for i := 0; i < 50; i++ {
		if m := e.call("jesse", "GET", "/machines/"+id, nil); m.Body["connected"] == true {
			return b
		}
		time.Sleep(100 * time.Millisecond)
	}
	e.t.Fatal("fake bridge never connected")
	return nil
}

func reqID(t *testing.T, out map[string]any) string {
	ids, _ := out["request_ids"].([]any)
	if len(ids) == 0 {
		t.Fatalf("no request ids: %v", out)
	}
	return ids[0].(string)
}

func TestAccessFlowExecRevoke(t *testing.T) {
	e := setup(t)
	b := e.addMachine("lab")
	if r := e.call("claude-code", "POST", "/machines", map[string]any{"id": "x", "label": "x", "os": "linux"}); r.code() != "humans_only" {
		t.Fatalf("agent enrolled a machine: %v", r.Body)
	}
	cs := e.mcpSession("claude-code")
	out, isErr := callTool(t, cs, "exec", map[string]any{"machine": "lab", "argv": []string{"echo", "hi"}})
	if !isErr || errCode(out) != "grant_required" {
		t.Fatalf("exec without grant: %v", out)
	}
	if out, _ := callTool(t, cs, "request_access", map[string]any{"machine": "lab", "capability": "exec", "selector": map[string]any{"argv_prefix": [][]string{{"echo"}, {"sleep"}}},
		"sandbox_level": "workspace-write", "duration": "1h", "reason": "short"}); errCode(out) != "reason_too_short" {
		t.Fatalf("short reason accepted: %v", out)
	}
	out, _ = callTool(t, cs, "request_access", map[string]any{"machine": "lab", "capability": "exec", "selector": map[string]any{"argv_prefix": [][]string{{"echo"}, {"sleep"}}},
		"sandbox_level": "workspace-write", "duration": "1h", "reason": "I need to run echo and sleep to test the bridge path."})
	id := reqID(t, out)
	if r := e.call("claude-code", "POST", "/access-requests/"+id+"/approve", nil); r.code() != "humans_only" {
		t.Fatalf("agent approved: %v", r.Body)
	}
	if r := e.call("jesse", "POST", "/access-requests/"+id+"/approve", map[string]any{"selector": map[string]any{"argv_prefix": [][]string{{"bash"}}}}); r.code() != "scope_wider_than_request" {
		t.Fatalf("wider approve: %v", r.Body)
	}
	if r := e.call("jesse", "POST", "/access-requests/"+id+"/approve", map[string]any{"sandbox_level": "unsandboxed"}); r.code() != "scope_wider_than_request" {
		t.Fatalf("wider level: %v", r.Body)
	}
	r := e.call("jesse", "POST", "/access-requests/"+id+"/approve", map[string]any{"selector": map[string]any{"argv_prefix": [][]string{{"echo", "hi"}, {"sleep"}}}})
	if r.Body["status"] != "narrowed" {
		t.Fatalf("narrow: %v", r.Body)
	}
	grant := r.Body["grant_id"].(string)
	w, _ := callTool(t, cs, "wait_for_access", map[string]any{"request_id": id, "timeout_s": 2})
	if w["status"] != "approved" {
		t.Fatalf("wait: %v", w)
	}
	out, isErr = callTool(t, cs, "exec", map[string]any{"machine": "lab", "argv": []string{"echo", "hi"}})
	if isErr || out["stdout"] != "ran echo hi" {
		t.Fatalf("exec: %v", out)
	}
	if out, _ := callTool(t, cs, "exec", map[string]any{"machine": "lab", "argv": []string{"echo", "other"}}); errCode(out) != "grant_required" {
		t.Fatalf("narrowed prefix not enforced: %v", out)
	}
	if m := e.call("jesse", "GET", "/machines/lab", nil); m.Body["online"] != true {
		t.Fatalf("machine not online after a completed call: %v", m.Body)
	}
	done := make(chan map[string]any, 1)
	go func() {
		o, _ := callTool(t, e.mcpSession("claude-code"), "exec", map[string]any{"machine": "lab", "argv": []string{"sleep", "60"}})
		done <- o
	}()
	time.Sleep(700 * time.Millisecond)
	if r := e.call("jesse", "POST", "/grants/"+grant+"/revoke", map[string]any{"reason": "test"}); r.Status != 200 {
		t.Fatalf("revoke: %v", r.Body)
	}
	select {
	case o := <-done:
		if errCode(o) != "grant_revoked" {
			t.Fatalf("mid-run revoke result: %v", o)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("revoke did not kill the running call")
	}
	if out, _ := callTool(t, cs, "exec", map[string]any{"machine": "lab", "argv": []string{"echo", "hi"}}); errCode(out) != "grant_required" {
		t.Fatalf("exec after revoke: %v", out)
	}
	if b.badSig != 0 {
		t.Fatalf("bridge saw %d bad signatures", b.badSig)
	}
	_ = context.Background()
}
