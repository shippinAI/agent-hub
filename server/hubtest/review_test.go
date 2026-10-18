package hubtest

import (
	"context"
	"testing"

	"agenthub/internal/store"
)

// token gives a preset principal (e.g. the reviewer agent) a test token.
func (e *env) token(id string) {
	tok, err := e.st.CreateToken(context.Background(), store.System, id, "test", 0)
	if err != nil {
		e.t.Fatal(err)
	}
	e.tokens[id] = tok.Token
}

func (e *env) grantArgv(t *testing.T, prefixes ...[]string) {
	cs := e.mcpSession("claude-code")
	out, _ := callTool(t, cs, "request_access", map[string]any{"machine": "lab", "capability": "exec",
		"selector": map[string]any{"argv_prefix": prefixes}, "sandbox_level": "workspace-write", "duration": "1h",
		"reason": "Change-policy test: commands through the bridge."})
	if r := e.call("jesse", "POST", "/access-requests/"+reqID(t, out)+"/approve", nil); r.Status != 200 {
		t.Fatalf("approve: %v", r.Body)
	}
}

const aptCmd = "apt-get install -s jq"

func ticketBody() map[string]any {
	return map[string]any{"id": "CHG-0001", "title": "Install jq on lab (simulated)", "machine": "lab", "command": aptCmd,
		"fields": map[string]string{"topology": "lab is a single box, no dependents", "blast_radius": "only the lab box package set; simulation changes nothing", "snapshot": "/var/backups/CHG-0001.txt"},
		"lists":  map[string][]string{"research": {"https://manpages.debian.org/apt-get"}, "verification": {"dpkg -s jq"}}}
}

// TestReviewerPassUnlocksChange: an infra change is blocked without a ticket, blocked until a reviewer other than the
// author PASSes, runs after the PASS, and goes stale when the ticket changes. All decisions land in the chain.
func TestReviewerPassUnlocksChange(t *testing.T) {
	e := setup(t)
	e.token("clyffy-infra-auditor")
	e.addMachine("lab")
	e.grantArgv(t, []string{"apt-get"}, []string{"rm"}, []string{"echo"})
	cs := e.mcpSession("claude-code")
	argv := []string{"apt-get", "install", "-s", "jq"}
	if out, isErr := callTool(t, cs, "exec", map[string]any{"machine": "lab", "argv": []string{"echo", "hi"}}); isErr {
		t.Fatalf("plain call must pass the policy: %v", out)
	}
	if out, isErr := callTool(t, cs, "exec", map[string]any{"machine": "lab", "argv": []string{"rm", "-rf", "/"}}); !isErr || errCode(out) != "policy_denied" {
		t.Fatalf("rm -rf / must be denied by policy: %v", out)
	}
	if out, isErr := callTool(t, cs, "exec", map[string]any{"machine": "lab", "argv": argv}); !isErr || errCode(out) != "policy_denied" {
		t.Fatalf("system change without a ticket must be denied: %v", out)
	}
	if out, isErr := callTool(t, cs, "ticket_create", ticketBody()); isErr {
		t.Fatalf("ticket_create: %v", out)
	}
	if out, isErr := callTool(t, cs, "ticket_evidence_add", map[string]any{"ticket": "CHG-0001", "kind": "snapshot", "path": "/var/backups/CHG-0001.txt",
		"text": "CHG-0001 dpkg -l snapshot: jq not installed"}); isErr {
		t.Fatalf("evidence: %v", out)
	}
	if out, _ := callTool(t, cs, "ticket_submit", map[string]any{"ticket": "CHG-0001"}); out["status"] != "pending_review" {
		t.Fatalf("submit: %v", out)
	}
	if out, isErr := callTool(t, cs, "exec", map[string]any{"machine": "lab", "argv": argv, "ticket": "CHG-0001"}); !isErr || errCode(out) != "policy_denied" {
		t.Fatalf("ticket without PASS must be denied: %v", out)
	}
	view, _ := callTool(t, cs, "review_ticket", map[string]any{"ticket": "CHG-0001"})
	sha := view["ticket"].(map[string]any)["sha256"].(string)
	if out, isErr := callTool(t, cs, "review_submit", map[string]any{"ticket": "CHG-0001", "verdict": "PASS", "summary": "lgtm", "ticket_sha256": sha}); !isErr || errCode(out) != "self_review" {
		t.Fatalf("author reviewed own ticket: %v", out)
	}
	if out, isErr := callTool(t, e.mcpSession("cursor"), "review_submit", map[string]any{"ticket": "CHG-0001", "verdict": "PASS", "summary": "lgtm", "ticket_sha256": sha}); !isErr || errCode(out) != "policy_denied" {
		t.Fatalf("non-reviewer reviewed: %v", out)
	}
	rv := e.mcpSession("clyffy-infra-auditor")
	q, _ := callTool(t, rv, "review_queue", nil)
	if l, _ := q["tickets"].([]any); len(l) != 1 {
		t.Fatalf("review queue: %v", q)
	}
	if out, isErr := callTool(t, rv, "review_submit", map[string]any{"ticket": "CHG-0001", "verdict": "PASS", "summary": "Simulated install, evidence ok", "ticket_sha256": "0000"}); !isErr || errCode(out) != "stale_review" {
		t.Fatalf("stale hash accepted: %v", out)
	}
	if out, _ := callTool(t, rv, "review_submit", map[string]any{"ticket": "CHG-0001", "verdict": "PASS", "summary": "Simulated install, evidence ok", "ticket_sha256": sha}); out["status"] != "passed" {
		t.Fatalf("PASS: %v", out)
	}
	if out, isErr := callTool(t, cs, "exec", map[string]any{"machine": "lab", "argv": argv, "ticket": "CHG-0001"}); isErr || out["stdout"] != "ran apt-get install -s jq" {
		t.Fatalf("PASS must unlock the change: %v", out)
	}
	if out, isErr := callTool(t, cs, "exec", map[string]any{"machine": "lab", "argv": []string{"apt-get", "install", "-s", "curl"}, "ticket": "CHG-0001"}); !isErr || errCode(out) != "policy_denied" {
		t.Fatalf("ticket must not cover another command: %v", out)
	}
	upd := ticketBody()
	upd["version"] = view["ticket"].(map[string]any)["version"]
	upd["title"] = "Install jq on lab (simulated, edited)"
	if out, isErr := callTool(t, cs, "ticket_update", upd); isErr || out["status"] != "pending_review" {
		t.Fatalf("edit must send the ticket back to review: %v", out)
	}
	if out, isErr := callTool(t, cs, "exec", map[string]any{"machine": "lab", "argv": argv, "ticket": "CHG-0001"}); !isErr || errCode(out) != "policy_denied" {
		t.Fatalf("stale PASS must not count: %v", out)
	}
	var allow, deny int
	_ = e.pool.QueryRow(context.Background(), `SELECT count(*) FILTER (WHERE data->>'allow'='true'), count(*) FILTER (WHERE data->>'allow'='false')
		FROM events WHERE action='policy.decision'`).Scan(&allow, &deny)
	if allow != 2 || deny != 5 {
		t.Fatalf("decision log: %d allow, %d deny", allow, deny)
	}
	var leaked int
	_ = e.pool.QueryRow(context.Background(), `SELECT count(*) FROM events WHERE action='policy.decision' AND data::text LIKE '%install -s jq%'`).Scan(&leaked)
	if leaked != 0 {
		t.Fatalf("decision log carries raw argv")
	}
	if r := e.call("jesse", "GET", "/events/verify", nil); r.Status != 200 || r.Body["ok"] != true {
		t.Fatalf("chain verify: %d %v", r.Status, r.Body)
	}
	t.Logf("plain allowed; rm -rf / denied; no ticket denied; self-review and non-reviewer refused; stale hash refused; PASS unlocked; other command denied; edit made the PASS stale; %d allow/%d deny logged", allow, deny)
}
