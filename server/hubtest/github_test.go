package hubtest

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

const ghSecret = "test-webhook-secret-0123456789"

const repo = "shippinAI/agent-hub"

func sha(c byte) string { return strings.Repeat(string(c), 40) }

// hook delivers one signed GitHub webhook (sig "" = sign correctly).
func (e *env) hook(event, delivery string, payload map[string]any, sig string) resp {
	body, _ := json.Marshal(payload)
	if sig == "" {
		m := hmac.New(sha256.New, []byte(ghSecret))
		m.Write(body)
		sig = "sha256=" + hex.EncodeToString(m.Sum(nil))
	}
	req, _ := http.NewRequest("POST", e.url+"/api/v1/integrations/github", bytes.NewReader(body))
	req.Header.Set("X-GitHub-Event", event)
	req.Header.Set("X-GitHub-Delivery", delivery)
	req.Header.Set("X-Hub-Signature-256", sig)
	r, err := http.DefaultClient.Do(req)
	if err != nil {
		e.t.Fatal(err)
	}
	defer r.Body.Close()
	out := resp{Status: r.StatusCode, Body: map[string]any{}}
	raw, _ := io.ReadAll(r.Body)
	_ = json.Unmarshal(raw, &out.Body)
	return out
}

var deliveries int

func (e *env) check(name, conclusion, branch, head string) resp {
	deliveries++
	return e.hook("check_run", fmt.Sprintf("d-%d", deliveries), map[string]any{"action": "completed", "repository": map[string]any{"full_name": repo},
		"check_run": map[string]any{"name": name, "status": "completed", "conclusion": conclusion, "head_sha": head,
			"html_url": "https://github.com/" + repo + "/runs/1", "check_suite": map[string]any{"head_branch": branch}}}, "")
}

func (e *env) push(branch, after string) resp {
	deliveries++
	return e.hook("push", fmt.Sprintf("d-%d", deliveries), map[string]any{"ref": "refs/heads/" + branch, "after": after,
		"repository": map[string]any{"full_name": repo},
		"commits":    []any{map[string]any{"id": after, "url": "https://github.com/" + repo + "/commit/" + after, "message": "fix the thing\n\nbody"}}}, "")
}

func links(e *env, id int64, kind string) []map[string]any {
	out := []map[string]any{}
	for _, l := range e.call("jesse", "GET", "/tasks/"+itoa(id), nil).Body["links"].([]any) {
		if m := l.(map[string]any); m["kind"] == kind {
			out = append(out, m)
		}
	}
	return out
}

// TestGitHubProvenance: a task links to its branch; signed GitHub deliveries add commits, the PR and check
// runs; a green check becomes evidence that satisfies a lane requiring it; a push makes it stale; a branch
// that only names the task (hub-N) is linked for provenance but cannot feed evidence.
func TestGitHubProvenance(t *testing.T) {
	e := setup(t)
	e.requireEvidence("ci.go")
	id := e.newTask("jesse", "provenance", nil)
	p := "/tasks/" + itoa(id)
	l := e.call("codex", "POST", p+"/claim", map[string]any{})
	f := num(l.Body["fence"])

	if r := e.hook("ping", "d-ping", map[string]any{"zen": "hi"}, "sha256=00"); r.Status != 401 {
		t.Fatalf("bad signature accepted: %d %v", r.Status, r.Body)
	}
	if r := e.call("codex", "POST", p+"/links", map[string]any{"kind": "branch", "repo": "not a repo", "ref": "x"}); r.code() != "invalid" {
		t.Errorf("bad repo: %v", r.Body)
	}
	if r := e.call("codex", "POST", p+"/links", map[string]any{"kind": "branch", "repo": repo, "ref": "fix/thing", "fence": f}); r.Status != 200 {
		t.Fatalf("link branch: %d %v", r.Status, r.Body)
	}
	if r := e.push("fix/thing", sha('a')); r.Status != 200 || len(r.Body["tasks"].([]any)) != 1 {
		t.Fatalf("push: %d %v", r.Status, r.Body)
	}
	if b := links(e, id, "branch"); len(b) != 1 || b[0]["head_sha"] != sha('a') || b[0]["author"] != "codex" {
		t.Fatalf("branch head after push: %v", b)
	}
	if c := links(e, id, "commit"); len(c) != 1 {
		t.Fatalf("commit links: %v", c)
	}
	if r := e.hook("pull_request", "d-pr", map[string]any{"action": "opened", "repository": map[string]any{"full_name": repo},
		"pull_request": map[string]any{"number": 15, "html_url": "https://github.com/" + repo + "/pull/15", "title": "fix", "state": "open",
			"draft": true, "head": map[string]any{"ref": "fix/thing", "sha": sha('a')}}}, ""); r.Status != 200 {
		t.Fatalf("pr: %v", r.Body)
	}
	if pr := links(e, id, "pr"); len(pr) != 1 || pr[0]["state"] != "draft" || pr[0]["ref"] != "15" {
		t.Fatalf("pr link: %v", pr)
	}

	// A failing check blocks, a later green one passes.
	if r := e.check("go", "failure", "fix/thing", sha('a')); num(r.Body["evidence"]) != 1 {
		t.Fatalf("failing check evidence: %v", r.Body)
	}
	if r := e.call("codex", "POST", p+"/complete", map[string]any{"fence": f, "summary": "x"}); r.code() != "evidence_missing" {
		t.Fatalf("complete after red CI: %v", r.Body)
	}
	e.check("go", "success", "fix/thing", sha('a'))
	e.check("codeql", "neutral", "fix/thing", sha('a'))
	// A new push: the green result was for the old head, so it no longer counts.
	e.push("fix/thing", sha('b'))
	if r := e.call("codex", "POST", p+"/complete", map[string]any{"fence": f, "summary": "x"}); r.code() != "evidence_missing" {
		t.Fatalf("stale green counted after push: %v", r.Body)
	}
	if r := e.check("go", "success", "fix/thing", sha('a')); num(r.Body["evidence"]) != 0 {
		t.Errorf("check on an old commit attached evidence: %v", r.Body)
	}
	e.check("go", "success", "fix/thing", sha('b'))
	if r := e.call("codex", "POST", p+"/complete", map[string]any{"fence": f, "summary": "green"}); r.Status != 200 || r.Body["state"] != "review" {
		t.Fatalf("complete after green CI on head: %d %v", r.Status, r.Body)
	}
	if cr := links(e, id, "check_run"); len(cr) != 2 {
		t.Errorf("check_run links (go, codeql): %v", cr)
	}

	// Redelivery is applied once.
	before := len(e.call("jesse", "GET", p, nil).Body["evidence"].([]any))
	r1 := e.hook("check_run", "d-same", map[string]any{"action": "completed", "repository": map[string]any{"full_name": repo},
		"check_run": map[string]any{"name": "go", "conclusion": "failure", "head_sha": sha('b'), "check_suite": map[string]any{"head_branch": "fix/thing"}}}, "")
	r2 := e.hook("check_run", "d-same", map[string]any{"action": "completed", "repository": map[string]any{"full_name": repo},
		"check_run": map[string]any{"name": "go", "conclusion": "failure", "head_sha": sha('b'), "check_suite": map[string]any{"head_branch": "fix/thing"}}}, "")
	if r1.Status != 200 || r2.Body["duplicate"] != true {
		t.Errorf("redelivery: %v / %v", r1.Body, r2.Body)
	}
	if after := len(e.call("jesse", "GET", p, nil).Body["evidence"].([]any)); after != before+1 {
		t.Errorf("redelivery applied twice: %d -> %d", before, after)
	}

	// A branch that only names a task gets linked, but its checks prove nothing for that task.
	other := e.newTask("jesse", "named only", nil)
	lo := e.call("cursor", "POST", "/tasks/"+itoa(other)+"/claim", map[string]any{})
	e.push("hub-"+itoa(other)+"-sneaky", sha('c'))
	if b := links(e, other, "branch"); len(b) != 1 || b[0]["author"] != "github" {
		t.Fatalf("name-based link: %v", b)
	}
	if r := e.check("go", "success", "hub-"+itoa(other)+"-sneaky", sha('c')); num(r.Body["evidence"]) != 0 {
		t.Errorf("name-linked branch fed evidence: %v", r.Body)
	}
	if r := e.call("cursor", "POST", "/tasks/"+itoa(other)+"/complete", map[string]any{"fence": lo.Body["fence"], "summary": "x"}); r.code() != "evidence_missing" {
		t.Errorf("name-linked green satisfied the gate: %v", r.Body)
	}
	if v := e.call("codex", "GET", "/events/verify", nil); v.Body["ok"] != true {
		t.Errorf("chain: %v", v.Body)
	}
}
