package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSearch(t *testing.T) {
	e, root := newEnv(t)
	os.MkdirAll(filepath.Join(root, "src"), 0o755)
	os.WriteFile(filepath.Join(root, "src/main.go"), []byte("package main\n// needle here\n"), 0o644)
	os.WriteFile(filepath.Join(root, ".gitignore"), []byte("skip/\n"), 0o644)
	os.MkdirAll(filepath.Join(root, "skip"), 0o755)
	os.WriteFile(filepath.Join(root, "skip/x.go"), []byte("needle\n"), 0o644)
	o := must(t, call(t, e, "start_search", map[string]any{"path": root, "pattern": "needle", "searchType": "content"}), "main.go:2")
	if strings.Contains(o.Text, "skip/x.go") {
		t.Fatal(".gitignore not honored")
	}
	must(t, call(t, e, "start_search", map[string]any{"path": root, "pattern": "*.go"}), "main.go")
	id := data(o)["status"].(map[string]any)["sessionId"].(string)
	must(t, call(t, e, "get_more_search_results", map[string]any{"sessionId": id}), "main.go")
	must(t, call(t, e, "list_searches", nil), id)
	call(t, e, "stop_search", map[string]any{"sessionId": id})
}

func TestProcessSession(t *testing.T) {
	e, root := newEnv(t)
	o := must(t, call(t, e, "start_process", map[string]any{"command": "echo hi; read x; echo got $x; sleep 30", "timeout_ms": 1500, "cwd": root}), "hi")
	pid := int(data(o)["pid"].(float64))
	o = must(t, call(t, e, "interact_with_process", map[string]any{"pid": pid, "input": "abc", "wait_for_prompt": false, "timeout_ms": 3000}), "got abc")
	must(t, call(t, e, "list_sessions", nil), "PID")
	must(t, call(t, e, "read_process_output", map[string]any{"pid": pid, "offset": -5}), "got abc")
	must(t, call(t, e, "force_terminate", map[string]any{"pid": pid}))
	code(t, call(t, e, "read_process_output", map[string]any{"pid": 999999}), "not_found")
}

func TestPTYPrompt(t *testing.T) {
	e, _ := newEnv(t)
	o := must(t, call(t, e, "start_process", map[string]any{"argv": []string{"python3", "-i", "-q"}, "pty": true, "timeout_ms": 8000}))
	if data(o)["state"] != "waiting_for_input" {
		t.Fatalf("want prompt, got %v: %s", data(o)["state"], o.Text)
	}
	pid := int(data(o)["pid"].(float64))
	must(t, call(t, e, "interact_with_process", map[string]any{"pid": pid, "input": "print(6*7)"}), "42")
}

func TestHonestTimeouts(t *testing.T) {
	e, _ := newEnv(t)
	start := time.Now()
	o := call(t, e, "run_command", map[string]any{"argv": []string{"sleep", "10"}, "timeout_ms": 500})
	code(t, o, "timeout")
	if time.Since(start) > 5*time.Second || !strings.Contains(o.Text, "killed") {
		t.Fatalf("timeout not enforced: %s", o.Text)
	}
	o = must(t, call(t, e, "start_process", map[string]any{"argv": []string{"sleep", "5"}, "timeout_ms": 999999}), "above this policy's max_timeout_ms")
	if data(o)["timeout_ms_applied"].(float64) != 20000 && data(o)["state"] != "finished" {
		t.Fatal(o.Text)
	}
	must(t, call(t, e, "run_command", map[string]any{"command": "echo out; echo err >&2; exit 3"}), "exit code 3", "out", "err")
	must(t, call(t, e, "run_code", map[string]any{"language": "python", "code": "print(2**10)"}), "1024")
}

func TestSandboxConfinesProcess(t *testing.T) {
	e, root := newEnv(t)
	o := call(t, e, "run_command", map[string]any{"command": "touch " + root + "/ok && touch /tmp/dg-escape-$$ ; ls /home 2>&1 | head -1"})
	must(t, o)
	if _, err := os.Stat(filepath.Join(root, "ok")); err != nil {
		t.Fatal("write inside root failed")
	}
	m, _ := filepath.Glob("/tmp/dg-escape-*")
	if len(m) > 0 {
		os.Remove(m[0])
		t.Fatal("process wrote outside its roots")
	}
}

func TestConfigAndAnalytics(t *testing.T) {
	e, root := newEnv(t, "proc.list")
	must(t, call(t, e, "get_config", nil), root, "telemetry: false", "sandbox")
	must(t, call(t, e, "list_processes", nil), "PID")
	code(t, call(t, e, "get_usage_stats", nil), "disabled")
	must(t, call(t, e, "give_feedback_to_desktop_commander", nil), "Nothing was sent")
}
