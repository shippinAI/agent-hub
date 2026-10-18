package hubtest

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"

	"agenthub/internal/policygate"
)

// fakePDP answers like gold-policy's POST /decide: deny when the argv touches "ip link", 500 when broken.
type fakePDP struct {
	mu     sync.Mutex
	broken bool
	seen   []map[string]any
}

func (f *fakePDP) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var in map[string]any
	_ = json.NewDecoder(r.Body).Decode(&in)
	f.mu.Lock()
	f.seen = append(f.seen, in)
	broken := f.broken
	f.mu.Unlock()
	if broken || r.URL.Path != "/v1/decide" {
		http.Error(w, `{"error":"boom"}`, 500)
		return
	}
	argv, _ := json.Marshal(in["argv"])
	allow := !strings.Contains(string(argv), `"ip","link"`)
	reason := ""
	if !allow {
		reason = "[network-commit-confirm] Network changes run only through gold-policy run"
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"decision_id": "d-1", "allow": allow, "reason": reason, "decision": map[string]any{"policy_version": 7}})
}

// grantExec approves an exec grant for echo + ip on machine lab for claude-code.
func (e *env) grantExec(t *testing.T) {
	cs := e.mcpSession("claude-code")
	out, _ := callTool(t, cs, "request_access", map[string]any{"machine": "lab", "capability": "exec",
		"selector": map[string]any{"argv_prefix": [][]string{{"echo"}, {"ip"}}}, "sandbox_level": "workspace-write", "duration": "1h",
		"reason": "Policy gate test: echo and ip through the bridge."})
	if r := e.call("jesse", "POST", "/access-requests/"+reqID(t, out)+"/approve", nil); r.Status != 200 {
		t.Fatalf("approve: %v", r.Body)
	}
}

func (e *env) grantUses(t *testing.T) int64 {
	var n int64
	if err := e.pool.QueryRow(context.Background(), `SELECT uses FROM grants WHERE machine_id='lab' AND ended_at IS NULL`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestPolicyGateBlocksBridgeCalls(t *testing.T) {
	e := setup(t)
	pdp := &fakePDP{}
	ps := httptest.NewServer(pdp)
	defer ps.Close()
	e.api.Access.Policy = policygate.New(ps.URL + "/v1")
	e.addMachine("lab")
	e.grantExec(t)
	cs := e.mcpSession("claude-code")
	if out, isErr := callTool(t, cs, "exec", map[string]any{"machine": "lab", "argv": []string{"echo", "hi"}}); isErr || out["stdout"] != "ran echo hi" {
		t.Fatalf("allowed call blocked: %v", out)
	}
	used := e.grantUses(t)
	out, isErr := callTool(t, cs, "exec", map[string]any{"machine": "lab", "argv": []string{"ip", "link", "set", "br0", "down"}})
	if !isErr || errCode(out) != "policy_denied" {
		t.Fatalf("grant allowed it, policy must still deny: %v", out)
	}
	if n := e.grantUses(t); n != used {
		t.Fatalf("a denied call consumed a grant use: %d -> %d", used, n)
	}
	pdp.mu.Lock()
	last := pdp.seen[len(pdp.seen)-1]
	pdp.mu.Unlock()
	if last["tool"] != "hub:exec" || last["principal"] == "" || !strings.Contains(fmtAny(last["hosts"]), "lab") {
		t.Fatalf("PDP request shape: %v", last)
	}
	pdp.mu.Lock()
	pdp.broken = true
	pdp.mu.Unlock()
	if out, isErr := callTool(t, cs, "exec", map[string]any{"machine": "lab", "argv": []string{"echo", "hi"}}); !isErr || errCode(out) != "policy_unavailable" {
		t.Fatalf("PDP down must fail closed: %v", out)
	}
	ps.Close()
	if out, isErr := callTool(t, cs, "exec", map[string]any{"machine": "lab", "argv": []string{"echo", "hi"}}); !isErr || errCode(out) != "policy_unavailable" {
		t.Fatalf("PDP unreachable must fail closed: %v", out)
	}
	var n int
	_ = e.pool.QueryRow(context.Background(), `SELECT count(*) FROM events WHERE action='policy.denied'`).Scan(&n)
	if n != 3 {
		t.Fatalf("want 3 policy.denied audit events, got %d", n)
	}
	t.Logf("allow passed, deny blocked without consuming a use, 500 and unreachable failed closed, %d policy.denied events", n)
}

// TestPolicyGateRealPDP runs against a live `gold-policy serve` (box-only) when HUB_TEST_POLICY_URL is set.
func TestPolicyGateRealPDP(t *testing.T) {
	u := os.Getenv("HUB_TEST_POLICY_URL")
	if u == "" {
		t.Skip("HUB_TEST_POLICY_URL not set")
	}
	e := setup(t)
	e.api.Access.Policy = policygate.New(u)
	e.addMachine("lab")
	e.grantExec(t)
	cs := e.mcpSession("claude-code")
	if out, isErr := callTool(t, cs, "exec", map[string]any{"machine": "lab", "argv": []string{"echo", "hi"}}); isErr {
		t.Fatalf("gold-policy blocked a harmless echo: %v", out)
	}
	out, isErr := callTool(t, cs, "exec", map[string]any{"machine": "lab", "argv": []string{"ip", "link", "set", "br0", "down"}})
	if !isErr || errCode(out) != "policy_denied" {
		t.Fatalf("gold-policy did not block a raw network change: %v", out)
	}
	t.Logf("real PDP verdict: %v", out)
}
