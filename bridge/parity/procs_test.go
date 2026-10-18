package parity

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestParitySearchProcs covers rows R16-R29 (search, terminal sessions, processes, local stats) over stdio.
func TestParitySearchProcs(t *testing.T) {
	cfg, root := fixture(t, "")
	cs := connect(t, cfg)
	var sid string
	var pid int
	steps := []struct {
		id string
		fn func(t *testing.T)
	}{
		{"R16 start_search content+files", func(t *testing.T) {
			r := want(t, callTool(t, cs, "start_search", map[string]any{"path": root, "pattern": "Needle", "searchType": "content"}), "", "code.go:3")
			sid, _ = r.data["status"].(map[string]any)["sessionId"].(string)
			want(t, callTool(t, cs, "start_search", map[string]any{"path": root, "pattern": "*.go"}), "", "code.go")
		}},
		{"R17 get_more_search_results", func(t *testing.T) {
			want(t, callTool(t, cs, "get_more_search_results", map[string]any{"sessionId": sid, "offset": 0}), "", "code.go")
		}},
		{"R18 list_searches", func(t *testing.T) { want(t, callTool(t, cs, "list_searches", nil), "", sid) }},
		{"R19 stop_search", func(t *testing.T) { want(t, callTool(t, cs, "stop_search", map[string]any{"sessionId": sid}), "") }},
		{"R20 start_process (REPL, prompt detection)", func(t *testing.T) {
			r := want(t, callTool(t, cs, "start_process", map[string]any{"command": "python3 -i -q", "shell": "bash-pty", "timeout_ms": 10000, "cwd": root}), "", "PID")
			if r.data["state"] != "waiting_for_input" {
				t.Fatalf("state %v: %s", r.data["state"], r.text)
			}
			pid = int(r.data["pid"].(float64))
		}},
		{"R21 interact_with_process", func(t *testing.T) {
			want(t, callTool(t, cs, "interact_with_process", map[string]any{"pid": pid, "input": "import os; print(len(os.listdir('.')), 6*7)"}), "", "42")
		}},
		{"R22 read_process_output", func(t *testing.T) {
			want(t, callTool(t, cs, "read_process_output", map[string]any{"pid": pid, "offset": -3, "timeout_ms": 500}), "", "42")
		}},
		{"R23 list_sessions", func(t *testing.T) { want(t, callTool(t, cs, "list_sessions", nil), "", "python3") }},
		{"R24 force_terminate", func(t *testing.T) {
			want(t, callTool(t, cs, "force_terminate", map[string]any{"pid": pid}), "")
			want(t, callTool(t, cs, "interact_with_process", map[string]any{"pid": pid, "input": "1"}), "not_running")
		}},
		{"R25 list_processes", func(t *testing.T) { want(t, callTool(t, cs, "list_processes", nil), "", "PID", "COMMAND") }},
		{"R26 kill_process (foreign pid, proc.kill)", func(t *testing.T) {
			c := exec.Command("sleep", "60")
			c.Start()
			defer c.Wait()
			want(t, callTool(t, cs, "kill_process", map[string]any{"pid": c.Process.Pid}), "")
			want(t, callTool(t, cs, "kill_process", map[string]any{"pid": 1}), "kill_failed")
		}},
		{"R27 get_usage_stats", func(t *testing.T) { want(t, callTool(t, cs, "get_usage_stats", nil), "", "start_search") }},
		{"R28 get_recent_tool_calls", func(t *testing.T) {
			want(t, callTool(t, cs, "get_recent_tool_calls", map[string]any{"maxResults": 5, "toolName": "list_sessions"}), "", "list_sessions")
		}},
		{"R29 give_feedback_to_desktop_commander", func(t *testing.T) {
			want(t, callTool(t, cs, "give_feedback_to_desktop_commander", nil), "", "example.invalid", "nothing was opened or sent")
		}},
		{"X02 honest timeout (timeout_ms above the cap)", func(t *testing.T) {
			want(t, callTool(t, cs, "start_process", map[string]any{"argv": []string{"sleep", "1"}, "timeout_ms": 999999}), "", "above this policy's max_timeout_ms")
			want(t, callTool(t, cs, "run_command", map[string]any{"argv": []string{"sleep", "5"}, "timeout_ms": 300}), "timeout", "killed")
		}},
		{"X03 sandbox confines processes", func(t *testing.T) {
			out := filepath.Join(filepath.Dir(root), "outside.txt")
			want(t, callTool(t, cs, "run_command", map[string]any{"command": "echo x > " + out + "; echo in > inside.txt", "cwd": root}), "")
			if _, err := os.Stat(out); err == nil {
				t.Fatal("wrote outside the roots")
			}
			if _, err := os.Stat(filepath.Join(root, "inside.txt")); err != nil {
				t.Fatal("could not write inside the root")
			}
		}},
		{"X04 run_code (in-memory)", func(t *testing.T) {
			want(t, callTool(t, cs, "run_code", map[string]any{"language": "python", "code": "print(sum(range(10)))"}), "", "45")
		}},
	}
	for _, s := range steps {
		t.Run(s.id, s.fn)
	}
}
