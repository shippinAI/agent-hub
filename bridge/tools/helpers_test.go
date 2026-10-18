package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"devagotchi/bridge/policy"
)

var selfBin string

func TestMain(m *testing.M) {
	dir, _ := os.MkdirTemp("", "dg-tools-")
	selfBin = filepath.Join(dir, "devagotchi")
	if out, err := exec.Command("go", "build", "-o", selfBin, "../cmd/devagotchi").CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "build: %v\n%s", err, out)
		os.Exit(1)
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

// newEnv returns an env rooted at a fresh temp dir with default policy plus extra capabilities.
func newEnv(t *testing.T, caps ...string) (*Env, string) {
	t.Helper()
	root := t.TempDir()
	p := policy.Defaults()
	p.Roots = []string{root}
	p.Capabilities = append(p.Capabilities, caps...)
	p.Limits.DefaultTimeoutMS = 3000
	p.Limits.MaxTimeoutMS = 20000
	e := NewEnv(policy.NewGate(p), "standalone", selfBin, nil, func(s string) { t.Log(s) })
	t.Cleanup(e.Close)
	return e, policy.Resolve(root)
}

func call(t *testing.T, e *Env, name string, args map[string]any) Out {
	t.Helper()
	tool, found := Find(name)
	if !found {
		t.Fatalf("no tool %s", name)
	}
	raw, _ := json.Marshal(args)
	return tool.Run(context.Background(), e, raw)
}

func must(t *testing.T, o Out, want ...string) Out {
	t.Helper()
	if o.Code != "" {
		t.Fatalf("unexpected error %s: %s", o.Code, o.Text)
	}
	for _, w := range want {
		if !strings.Contains(o.Text, w) {
			t.Fatalf("missing %q in:\n%s", w, o.Text)
		}
	}
	return o
}

func code(t *testing.T, o Out, want string) {
	t.Helper()
	if o.Code != want {
		t.Fatalf("code = %q, want %q: %s", o.Code, want, o.Text)
	}
}

func data(o Out) map[string]any {
	var m map[string]any
	json.Unmarshal(mustJSON(o.Data), &m)
	return m
}
