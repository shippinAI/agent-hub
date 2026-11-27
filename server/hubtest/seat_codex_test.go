package hubtest

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"agenthub/internal/seat"
)

// fakeCodex stands in for `codex exec --json -`, printing the event stream that codex-rs/exec emits.
const fakeCodex = `P=$(cat)
echo '{"type":"thread.started","thread_id":"th-1"}'
echo '{"type":"turn.started"}'
echo '{"type":"item.started","item":{"id":"item_0","type":"command_execution","command":"ls","aggregated_output":"","exit_code":null,"status":"in_progress"}}'
echo '{"type":"item.completed","item":{"id":"item_0","type":"command_execution","command":"ls","aggregated_output":"hello.txt","exit_code":0,"status":"completed"}}'
if [ "$FAKE_MODE" = fail ]; then
  echo '{"type":"turn.failed","error":{"message":"stream disconnected"}}'
  exit 1
fi
echo fixed >> hello.txt
echo '{"type":"item.completed","item":{"id":"item_1","type":"file_change","changes":[{"path":"hello.txt","kind":"update"}],"status":"completed"}}'
echo '{"type":"item.completed","item":{"id":"item_2","type":"agent_message","text":"appended fixed to hello.txt"}}'
echo '{"type":"turn.completed","usage":{"input_tokens":1200,"cached_input_tokens":1000,"output_tokens":80,"reasoning_output_tokens":30}}'
`

// TestSeatCodex: the same seat contract with Codex as the agent. Usage is recorded under the configured model
// (Codex's stream names none) and priced from the catalog; a failed turn hands the task off.
func TestSeatCodex(t *testing.T) {
	e := setup(t)
	e.requireEvidence("test.unit")
	repoDir := t.TempDir()
	gitT(t, repoDir, "init", "-q", "-b", "master")
	if err := os.WriteFile(filepath.Join(repoDir, "hello.txt"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitT(t, repoDir, "add", ".")
	gitT(t, repoDir, "commit", "-q", "-m", "init")
	cfg := func(mode string) seat.Config {
		return seat.Config{Name: "codex-seat", Hub: &seat.Client{URL: e.url, Token: e.tokens["codex"]}, Repo: repoDir,
			WorkRoot: t.TempDir(), Adapter: "codex", Model: "gpt-test", Agent: []string{"sh", "-c", fakeCodex},
			Env: map[string]string{"FAKE_MODE": mode}, Sandbox: "unsandboxed", EscalateTo: "jesse",
			Checks: []seat.Check{{Kind: "test.unit", Argv: []string{"sh", "-c", "grep -q fixed hello.txt"}}},
			Beat:   200 * time.Millisecond, Timeout: time.Minute}
	}
	ctx := context.Background()

	id := e.newTask("jesse", "Codex fixes hello", nil)
	o, err := seat.RunOnce(ctx, cfg("ok"))
	if err != nil || o.State != "completed" || o.TaskID != id || o.Turns != 3 {
		t.Fatalf("codex run: %+v %v", o, err)
	}
	if out := gitT(t, repoDir, "show", o.Head+":hello.txt"); out != "hello\nfixed" {
		t.Errorf("committed file: %q", out)
	}
	var model, provider, basis string
	var in, cached, out, reasoning int64
	err = e.pool.QueryRow(ctx, `SELECT model,provider_id,coalesce(cost_basis,''),input_tokens,cache_read_tokens,output_tokens,reasoning_tokens
		FROM usage_events WHERE task_id=$1`, id).Scan(&model, &provider, &basis, &in, &cached, &out, &reasoning)
	if err != nil || model != "gpt-test" || provider != "openai-api" || in != 1200 || cached != 1000 || out != 80 || reasoning != 30 {
		t.Errorf("codex usage: %s %s %s %d %d %d %d %v", model, provider, basis, in, cached, out, reasoning, err)
	}
	run := e.call("jesse", "GET", "/runs/"+itoa(o.RunID), nil).Body["run"].(map[string]any)
	if run["adapter"] != "codex" || run["model"] != "gpt-test" || run["session_id"] != "th-1" || run["status"] != "completed" {
		t.Errorf("codex run record: %v", run)
	}

	fail := e.newTask("jesse", "Codex loses its stream", nil)
	o, err = seat.RunOnce(ctx, cfg("fail"))
	if err != nil || o.State != "handed_off" || o.TaskID != fail || !strings.Contains(o.Reason, "did not finish") {
		t.Fatalf("failed codex run: %+v %v", o, err)
	}
	if v := e.call("jesse", "GET", "/events/verify", nil); v.Body["ok"] != true {
		t.Errorf("chain: %v", v.Body)
	}
}
