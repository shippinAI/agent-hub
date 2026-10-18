package hubtest

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// buildBridge compiles the real bridge binary once per test run.
func buildBridge(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "devagotchi-bridge")
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/devagotchi-bridge")
	cmd.Dir = "../../bridge"
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build bridge: %v\n%s", err, out)
	}
	return bin
}

// startRealBridge enrolls and runs the real bridge against the test hub in poll mode. It returns the config dir.
func (e *env) startRealBridge(t *testing.T, bin, machine string) string {
	t.Helper()
	e.setSetting("bridge_poll_seconds", 1)
	r := e.call("jesse", "POST", "/machines", map[string]any{"id": machine, "label": "Real " + machine, "os": "linux"})
	if r.Status != 200 {
		t.Fatalf("create machine: %d %v", r.Status, r.Body)
	}
	code := r.Body["enrollment"].(map[string]any)["code"].(string)
	dir := t.TempDir()
	env := append(os.Environ(), "DEVAGOTCHI_CONFIG_DIR="+dir, "DEVAGOTCHI_TRANSPORT=poll")
	en := exec.Command(bin, "enroll", "--hub", e.url)
	en.Env, en.Stdin = env, strings.NewReader(code+"\n")
	if out, err := en.CombinedOutput(); err != nil {
		t.Fatalf("enroll: %v\n%s", err, out)
	}
	run := exec.Command(bin, "run")
	run.Env = env
	if err := run.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = run.Process.Kill(); _, _ = run.Process.Wait() })
	for i := 0; i < 80; i++ {
		if m := e.call("jesse", "GET", "/machines/"+machine, nil); m.Body["connected"] == true {
			return dir
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("real bridge never connected")
	return ""
}

// TestRealBridgeExecRevoke runs the actual bridge: real sandboxed exec and a mid-run revoke that kills it.
func TestRealBridgeExecRevoke(t *testing.T) {
	if _, err := exec.LookPath("bwrap"); err != nil {
		t.Skip("bwrap not installed")
	}
	bin := buildBridge(t)
	e := setup(t)
	e.startRealBridge(t, bin, "realpc")
	ws := t.TempDir()
	cs := e.mcpSession("claude-code")
	out, _ := callTool(t, cs, "request_access", map[string]any{"machine": "realpc", "capability": "exec",
		"selector":      map[string]any{"argv_prefix": [][]string{{"echo"}, {"sleep"}, {"cat"}}, "paths": []string{ws}},
		"sandbox_level": "workspace-write", "duration": "1h", "reason": "Run echo, sleep and cat in the sandbox for the e2e."})
	id := reqID(t, out)
	r := e.call("jesse", "POST", "/access-requests/"+id+"/approve", nil)
	if r.Status != 200 || (r.Body["status"] != "approved" && r.Body["status"] != "narrowed") {
		t.Fatalf("approve: %d %v", r.Status, r.Body)
	}
	grant := r.Body["grant_id"].(string)

	out, isErr := callTool(t, cs, "exec", map[string]any{"machine": "realpc", "argv": []string{"echo", "hello-from-sandbox"}})
	if isErr || !strings.Contains(stringOf(out["stdout"]), "hello-from-sandbox") {
		t.Fatalf("real exec: %v", out)
	}

	// Sandbox confinement: a file outside the granted path is not bound into the sandbox, so it cannot be read.
	secret := filepath.Join(t.TempDir(), "outside-secret.txt")
	if err := os.WriteFile(secret, []byte("TOP-SECRET-DO-NOT-LEAK"), 0o600); err != nil {
		t.Fatal(err)
	}
	out, _ = callTool(t, cs, "exec", map[string]any{"machine": "realpc", "argv": []string{"cat", secret}})
	if strings.Contains(stringOf(out["stdout"]), "TOP-SECRET") {
		t.Fatalf("sandbox leaked a file outside the grant: %v", out)
	}

	// Mid-run revoke kills the running process.
	done := make(chan map[string]any, 1)
	go func() {
		o, _ := callTool(t, e.mcpSession("claude-code"), "exec", map[string]any{"machine": "realpc", "argv": []string{"sleep", "60"}})
		done <- o
	}()
	time.Sleep(1500 * time.Millisecond)
	if r := e.call("jesse", "POST", "/grants/"+grant+"/revoke", map[string]any{"reason": "e2e"}); r.Status != 200 {
		t.Fatalf("revoke: %v", r.Body)
	}
	select {
	case o := <-done:
		if errCode(o) != "grant_revoked" {
			t.Fatalf("mid-run revoke result: %v", o)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("revoke did not kill the running sleep")
	}
	// No orphan sleep left behind.
	time.Sleep(500 * time.Millisecond)
	if ps, _ := exec.Command("pgrep", "-f", "sleep 60").Output(); len(strings.TrimSpace(string(ps))) > 0 {
		out, _ := exec.Command("bash", "-c", "ps -o pid,cmd -C sleep 2>/dev/null | grep '60' || true").Output()
		if strings.Contains(string(out), "sleep 60") {
			t.Fatalf("orphaned sleep after revoke:\n%s", out)
		}
	}
	_ = context.Background()
}

func stringOf(v any) string {
	s, _ := v.(string)
	return s
}
