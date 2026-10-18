package runner

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"devagotchi/bridge/config"
	"devagotchi/bridge/wire"
)

func testRunner(t *testing.T) *Runner {
	t.Helper()
	self := filepath.Join(t.TempDir(), "devagotchi")
	if out, err := exec.Command("go", "build", "-o", self, "../cmd/devagotchi").CombinedOutput(); err != nil {
		t.Fatalf("build: %v %s", err, out)
	}
	return New(config.Config{CacheSeconds: 5}, nil, nil, self)
}

func frame(op string, g *wire.Grant, args map[string]any) wire.Frame {
	b, _ := json.Marshal(args)
	gb, _ := json.Marshal(g) // selectors arrive as JSON on the wire
	var wg wire.Grant
	json.Unmarshal(gb, &wg)
	return wire.Frame{Op: op, GrantID: "g1", Grant: &wg, Args: b}
}

func TestProcLeaseOps(t *testing.T) {
	r := testRunner(t)
	dir := t.TempDir()
	g := &wire.Grant{Capability: "proc.interactive", Level: "workspace-write", Selector: map[string]any{"paths": []string{dir}}}
	ctx := context.Background()
	res := r.proc(ctx, frame("proc.start", g, map[string]any{"argv": []string{"sh", "-c", "echo ready; read x; echo got $x"}, "cwd": dir, "lease_id": "L1", "fence": 7, "ttl_s": 60}))
	if !res.OK || !strings.Contains(res.Stdout, "ready") {
		t.Fatalf("start: %+v", res)
	}
	if res := r.proc(ctx, frame("proc.write", g, map[string]any{"lease_id": "L1", "fence": 6, "data": "x\n"})); res.Error != "stale_fence" {
		t.Fatalf("want stale_fence, got %+v", res)
	}
	var st struct{ Seq int64 }
	json.Unmarshal(res.Data, &st)
	r.proc(ctx, frame("proc.write", g, map[string]any{"lease_id": "L1", "fence": 7, "data": "abc\n"}))
	res = r.proc(ctx, frame("proc.read", g, map[string]any{"lease_id": "L1", "fence": 7, "after_seq": st.Seq, "wait_ms": 3000}))
	if !strings.Contains(res.Stdout, "got abc") {
		t.Fatalf("read: %+v", res)
	}
	bad := r.proc(ctx, frame("proc.start", g, map[string]any{"argv": []string{"true"}, "cwd": "/etc", "lease_id": "L2"}))
	if bad.Error != "scope_violation" {
		t.Fatalf("cwd outside grant: %+v", bad)
	}
}

func TestToolOpUsesGrantScope(t *testing.T) {
	r := testRunner(t)
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello\n"), 0o644)
	g := &wire.Grant{Capability: "fs.read", Level: "read-only", Selector: map[string]any{"paths": []string{dir}}}
	ctx := context.Background()
	call := func(name string, args map[string]any) wire.Result {
		return r.tool(ctx, frame("tool", g, map[string]any{"name": name, "arguments": args}))
	}
	if res := call("read_file", map[string]any{"path": filepath.Join(dir, "a.txt")}); !res.OK || !strings.Contains(res.Stdout, "hello") {
		t.Fatalf("read: %+v", res)
	}
	if res := call("read_file", map[string]any{"path": "/etc/hostname"}); res.Error != "grant_required" {
		t.Fatalf("outside: %+v", res)
	}
	if res := call("write_file", map[string]any{"path": filepath.Join(dir, "b.txt"), "content": "x"}); res.Error != "policy_denied" {
		t.Fatalf("write under fs.read: %+v", res)
	}
	pg := &wire.Grant{Capability: "exec", Level: "workspace-write", Selector: map[string]any{"paths": []string{dir}, "argv_prefix": [][]string{{"echo"}}}}
	res := r.tool(ctx, frame("tool", pg, map[string]any{"name": "run_command", "arguments": map[string]any{"command": "rm -rf ~"}}))
	if res.Error != "scope_violation" {
		t.Fatalf("shell string under prefixes: %+v", res)
	}
	res = r.tool(ctx, frame("tool", pg, map[string]any{"name": "run_command", "arguments": map[string]any{"argv": []string{"echo", "hi"}, "cwd": dir}}))
	if !res.OK || !strings.Contains(res.Stdout, "hi") {
		t.Fatalf("argv in prefix: %+v", res)
	}
}
