package hubtest

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"agenthub/internal/seat"
	"devagotchi/bridge/sandbox/sandboxtest"
)

// fakeAgent stands in for `claude -p --output-format stream-json`: it reads the prompt from stdin, edits the
// worktree per FAKE_MODE and prints the stream a real run prints.
const fakeAgent = `P=$(cat)
S="s-$(date +%s%N)"
echo '{"type":"system","subtype":"init","session_id":"'$S'","model":"claude-test"}'
sleep 0.8
case "$P" in *"greets politely"*) seen=yes;; *) seen=no;; esac
case "$FAKE_MODE" in
ok) echo fixed >> hello.txt ;;
break) echo broken >> hello.txt ;;
esac
echo '{"type":"assistant","session_id":"'$S'","message":{"id":"m1","model":"claude-test","usage":{"input_tokens":100,"cache_read_input_tokens":900,"output_tokens":50}}}'
if [ "$FAKE_MODE" = error ]; then
  echo '{"type":"result","subtype":"error_during_execution","is_error":true,"result":"tool crashed","num_turns":1,"session_id":"'$S'","total_cost_usd":0.01,"usage":{"input_tokens":100,"output_tokens":50}}'
  exit 1
fi
echo '{"type":"result","subtype":"success","is_error":false,"result":"edited hello.txt; acceptance seen: '$seen'","num_turns":2,"session_id":"'$S'","total_cost_usd":0.06,"modelUsage":{"claude-test":{"inputTokens":100,"outputTokens":50,"cacheReadInputTokens":900,"costUSD":0.05},"claude-small":{"inputTokens":10,"outputTokens":5,"costUSD":0.01}}}'
`

func gitT(t *testing.T, dir string, args ...string) string {
	t.Helper()
	c := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=t", "-c", "user.email=t@t", "-c", "commit.gpgsign=false"}, args...)...)
	out, err := c.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// TestSeatRuntime runs the agent runtime end to end against a real hub: claim, worktree, agent, commit, links,
// usage, checks as evidence, push, draft PR, complete; and the two ways a run fails (a red check, an agent error),
// both handed to a human so no seat re-claims them in a loop.
func TestSeatRuntime(t *testing.T) {
	t.Run("unsandboxed", func(t *testing.T) { seatFlow(t, "unsandboxed", "") })
	t.Run("sandboxed", func(t *testing.T) {
		sandboxtest.Require(t)
		bin := filepath.Join(t.TempDir(), "hub-seat")
		if out, err := exec.Command("go", "build", "-o", bin, "agenthub/cmd/hub-seat").CombinedOutput(); err != nil {
			t.Fatalf("build hub-seat: %v %s", err, out)
		}
		seatFlow(t, "workspace-write", bin)
	})
}

func seatFlow(t *testing.T, level, self string) {
	e := setup(t)
	e.requireEvidence("test.unit")
	repoDir, origin := t.TempDir(), t.TempDir()
	gitT(t, origin, "init", "-q", "--bare")
	gitT(t, repoDir, "init", "-q", "-b", "master")
	if err := os.WriteFile(filepath.Join(repoDir, "hello.txt"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitT(t, repoDir, "add", ".")
	gitT(t, repoDir, "commit", "-q", "-m", "init")
	gitT(t, repoDir, "remote", "add", "origin", origin)
	gitT(t, repoDir, "push", "-q", "origin", "master")

	var mu sync.Mutex
	var prs []map[string]any
	gh := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/repos/"+repo+"/pulls" || r.Header.Get("Authorization") != "Bearer gh-token" {
			http.Error(w, "unexpected", 400)
			return
		}
		var in map[string]any
		_ = json.NewDecoder(r.Body).Decode(&in)
		mu.Lock()
		prs = append(prs, in)
		mu.Unlock()
		w.WriteHeader(201)
		_, _ = w.Write([]byte(`{"number":7,"html_url":"https://github.com/` + repo + `/pull/7"}`))
	}))
	defer gh.Close()

	workRoot := t.TempDir()
	cfg := func(mode string) seat.Config {
		return seat.Config{Name: "seat-1", Hub: &seat.Client{URL: e.url, Token: e.tokens["claude-code"]}, Repo: repoDir, RepoName: repo,
			WorkRoot: workRoot, Agent: []string{"sh", "-c", fakeAgent}, Env: map[string]string{"FAKE_MODE": mode}, Sandbox: level, Self: self,
			Checks: []seat.Check{{Kind: "test.unit", Argv: []string{"sh", "-c", "grep -q fixed hello.txt"}}},
			Push:   "origin", PR: &seat.GitHubPR{Token: "gh-token", API: gh.URL, Base: "master"}, EscalateTo: "jesse",
			Beat: 200 * time.Millisecond, Timeout: time.Minute}
	}
	ctx := context.Background()

	// 1. A good run completes with its branch pushed, a draft PR, evidence for the head commit and its cost.
	id := e.newTask("jesse", "Make hello friendlier", map[string]any{"acceptance": []string{"greets politely"}})
	o, err := seat.RunOnce(ctx, cfg("ok"))
	if err != nil || o.State != "completed" || o.TaskID != id {
		t.Fatalf("good run: %+v %v", o, err)
	}
	if o.Branch != "hub/"+itoa(id)+"-make-hello-friendlier" || len(o.Head) != 40 {
		t.Fatalf("branch/head: %+v", o)
	}
	det := e.call("jesse", "GET", "/tasks/"+itoa(id), nil).Body
	if task := det["task"].(map[string]any); task["state"] != "review" || task["last_worker"] != "claude-code" {
		t.Fatalf("task after run: %v", task)
	}
	if b := links(e, id, "branch"); len(b) != 1 || b[0]["head_sha"] != o.Head || b[0]["author"] != "claude-code" {
		t.Errorf("branch link: %v", b)
	}
	if c := links(e, id, "commit"); len(c) != 1 || c[0]["ref"] != o.Head {
		t.Errorf("commit link: %v", c)
	}
	if pr := links(e, id, "pr"); len(pr) != 1 || pr[0]["ref"] != "7" || pr[0]["url"] != "https://github.com/"+repo+"/pull/7" {
		t.Errorf("pr link: %v", pr)
	}
	if len(prs) != 1 || prs[0]["draft"] != true || prs[0]["base"] != "master" || prs[0]["head"] != o.Branch {
		t.Errorf("PR request: %v", prs)
	}
	ev := det["evidence"].([]any)
	if len(ev) != 1 || ev[0].(map[string]any)["status"] != "pass" || ev[0].(map[string]any)["data"].(map[string]any)["head_sha"] != o.Head {
		t.Errorf("evidence: %v", ev)
	}
	if u := det["usage"].(map[string]any); num(u["events"]) != 2 || num(u["estimate_micros"]) != 60_000 || num(u["input_tokens"]) != 1010 {
		t.Errorf("usage: %v", u)
	}
	if got := gitT(t, origin, "rev-parse", "refs/heads/"+o.Branch); got != o.Head {
		t.Errorf("pushed head %s, want %s", got, o.Head)
	}
	if msg := gitT(t, repoDir, "log", "-1", "--format=%an %s%n%b", o.Branch); !strings.Contains(msg, "seat-1 hub #"+itoa(id)) ||
		!strings.Contains(msg, "acceptance seen: yes") {
		t.Errorf("commit: %q", msg)
	}
	if _, err := os.Stat(filepath.Join(workRoot, itoa(id)+"-1")); !os.IsNotExist(err) {
		t.Errorf("worktree left behind: %v", err)
	}
	if fi, err := os.Stat(filepath.Join(workRoot, "logs", itoa(id)+"-1.jsonl")); err != nil || fi.Size() == 0 {
		t.Errorf("transcript: %v", err)
	}
	run := e.call("jesse", "GET", "/runs/"+itoa(o.RunID), nil).Body
	rec := run["run"].(map[string]any)
	var joined strings.Builder
	for _, c := range run["chunks"].([]any) {
		joined.WriteString(c.(map[string]any)["body"].(string))
	}
	local, _ := os.ReadFile(filepath.Join(workRoot, "logs", itoa(id)+"-1.jsonl"))
	sum := sha256.Sum256(local)
	if rec["status"] != "completed" || rec["head_sha"] != o.Head || num(rec["turns"]) != 2 || rec["model"] != "claude-test" ||
		rec["adapter"] != "claude-code" || rec["seat"] != "seat-1" || !strings.HasPrefix(rec["session_id"].(string), "s-") {
		t.Errorf("run record: %v", rec)
	}
	if joined.String() != string(local) || rec["transcript_sha256"] != hex.EncodeToString(sum[:]) || rec["truncated"] != false {
		t.Errorf("transcript on the hub (%d bytes) differs from the seat's copy (%d bytes): %v", joined.Len(), len(local), rec)
	}
	if rs := det["runs"].([]any); len(rs) != 1 {
		t.Errorf("task runs: %v", rs)
	}
	var beats int
	_ = e.pool.QueryRow(ctx, `SELECT count(*) FROM events WHERE action='task.heartbeat' AND task_id=$1`, id).Scan(&beats)
	if beats < 1 {
		t.Errorf("no heartbeats during a run")
	}

	// 2. A red check: the work is kept on the branch, complete is refused, the task goes to a human.
	bad := e.newTask("jesse", "Break hello", nil)
	if r := e.call("jesse", "GET", "/tasks/"+itoa(id), nil); r.Body["task"].(map[string]any)["state"] != "review" {
		t.Fatal("task 1 moved")
	}
	o, err = seat.RunOnce(ctx, cfg("break"))
	if err != nil || o.State != "handed_off" || o.TaskID != bad || !strings.Contains(o.Reason, "failing [test.unit]") {
		t.Fatalf("red check run: %+v %v", o, err)
	}
	det = e.call("jesse", "GET", "/tasks/"+itoa(bad), nil).Body
	task := det["task"].(map[string]any)
	if task["assignee"] != "jesse" || task["holder"] != nil {
		t.Errorf("failed task not handed to jesse: %v", task)
	}
	if ev := det["evidence"].([]any); len(ev) != 1 || ev[0].(map[string]any)["status"] != "fail" {
		t.Errorf("failing evidence: %v", ev)
	}
	notes := det["notes"].([]any)
	if last := notes[len(notes)-1].(map[string]any); last["kind"] != "handoff" || !strings.Contains(last["body"].(string), o.Branch) {
		t.Errorf("handoff note: %v", last)
	}

	// 3. The agent reports an error: nothing to keep, cost still recorded, task handed off.
	crash := e.newTask("jesse", "Crash", nil)
	o, err = seat.RunOnce(ctx, cfg("error"))
	if err != nil || o.State != "handed_off" || o.TaskID != crash || !strings.Contains(o.Reason, "did not finish") {
		t.Fatalf("agent error run: %+v %v", o, err)
	}
	if r := e.call("jesse", "GET", "/runs/"+itoa(o.RunID), nil).Body["run"].(map[string]any); r["status"] != "handed_off" ||
		!strings.Contains(r["reason"].(string), "did not finish") {
		t.Errorf("failed run record: %v", r)
	}
	if u := e.call("jesse", "GET", "/tasks/"+itoa(crash), nil).Body["usage"].(map[string]any); num(u["estimate_micros"]) != 10_000 {
		t.Errorf("usage of a failed run: %v", u)
	}
	if v := e.call("jesse", "GET", "/events/verify", nil); v.Body["ok"] != true {
		t.Errorf("chain: %v", v.Body)
	}
}
