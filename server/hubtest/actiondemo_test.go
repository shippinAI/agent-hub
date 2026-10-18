package hubtest

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

// TestActionFeedDemo drives a real bridge through denied, allowed, failing and editing calls, then keeps the
// hub up for HUB_UI_DEMO_HOLD seconds so tests/actions-ui.sh can screenshot the feed. Skipped unless set.
// It writes {url, token} for the human to HUB_UI_DEMO_FILE (0600); the token is a throwaway test token.
func TestActionFeedDemo(t *testing.T) {
	hold, _ := strconv.Atoi(os.Getenv("HUB_UI_DEMO_HOLD"))
	if hold <= 0 {
		t.Skip("HUB_UI_DEMO_HOLD not set")
	}
	if _, err := exec.LookPath("bwrap"); err != nil {
		t.Skip("bwrap not installed")
	}
	bin := buildBridge(t)
	e := setup(t)
	e.startRealBridge(t, bin, "devbox")
	e.setSetting("audit_archive_dir", t.TempDir())
	ws := t.TempDir()
	cs := e.mcpSession("claude-code")
	callTool(t, cs, "exec", map[string]any{"machine": "devbox", "argv": []string{"rm", "-rf", ws}})
	for _, req := range []map[string]any{
		{"capability": "exec", "selector": map[string]any{"argv_prefix": [][]string{{"echo"}, {"ls"}, {"cat"}}, "paths": []string{ws}}},
		{"capability": "fs.write", "selector": map[string]any{"paths": []string{ws}}},
	} {
		req["machine"], req["sandbox_level"], req["duration"] = "devbox", "workspace-write", "1h"
		req["reason"] = "Demo: run commands and edit a file so the action feed has real entries."
		out, _ := callTool(t, cs, "request_access", req)
		if r := e.call("jesse", "POST", "/access-requests/"+reqID(t, out)+"/approve", nil); r.Status != 200 {
			t.Fatalf("approve: %v", r.Body)
		}
	}
	callTool(t, cs, "exec", map[string]any{"machine": "devbox", "argv": []string{"echo", "deploying", "--api-key", "sk-demo1234567890abcdef"}, "cwd": ws})
	callTool(t, cs, "exec", map[string]any{"machine": "devbox", "argv": []string{"ls", filepath.Join(ws, "missing")}, "cwd": ws})
	f := filepath.Join(ws, "config.yaml")
	callTool(t, cs, "machine_tool", map[string]any{"machine": "devbox", "name": "write_file",
		"arguments": map[string]any{"path": f, "content": "replicas: 1\nimage: hub:0.1\n", "mode": "rewrite"}})
	callTool(t, cs, "machine_tool", map[string]any{"machine": "devbox", "name": "edit_block",
		"arguments": map[string]any{"file_path": f, "old_string": "replicas: 1", "new_string": "replicas: 3"}})
	callTool(t, cs, "exec", map[string]any{"machine": "devbox", "argv": []string{"cat", f}, "cwd": ws})
	callTool(t, e.mcpSession("codex"), "list_ready", map[string]any{})
	if out := os.Getenv("HUB_UI_DEMO_FILE"); out != "" {
		b, _ := json.Marshal(map[string]string{"url": e.url, "token": e.tokens["jesse"]})
		if err := os.WriteFile(out, b, 0o600); err != nil {
			t.Fatal(err)
		}
		defer os.Remove(out)
	}
	t.Logf("demo hub at %s for %ds", e.url, hold)
	time.Sleep(time.Duration(hold) * time.Second)
}
