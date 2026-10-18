package parity

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var bin string

func TestMain(m *testing.M) {
	dir, _ := os.MkdirTemp("", "dg-parity-")
	bin = filepath.Join(dir, "devagotchi")
	if out, err := exec.Command("go", "build", "-o", bin, "../cmd/devagotchi").CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "build: %v\n%s", err, out)
		os.Exit(1)
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

// fixture writes a policy and sample files; it returns the config dir and the root.
func fixture(t *testing.T, extra string) (cfg, root string) {
	t.Helper()
	base := t.TempDir()
	cfg, root = filepath.Join(base, "cfg"), filepath.Join(base, "root")
	os.MkdirAll(cfg, 0o700)
	os.MkdirAll(filepath.Join(root, "sub"), 0o755)
	var lines []string
	for i := 1; i <= 50; i++ {
		lines = append(lines, fmt.Sprintf("line %d", i))
	}
	os.WriteFile(filepath.Join(root, "notes.txt"), []byte(strings.Join(lines, "\n")+"\n"), 0o644)
	os.WriteFile(filepath.Join(root, "sub", "code.go"), []byte("package sub\n\nfunc Needle() {}\n"), 0o644)
	var img bytes.Buffer
	png.Encode(&img, image.NewRGBA(image.Rect(0, 0, 4, 4)))
	os.WriteFile(filepath.Join(root, "pic.png"), img.Bytes(), 0o644)
	pol := fmt.Sprintf("sandbox_level = \"workspace-write\"\nroots = [%q]\ncapabilities = [\"fs.read\", \"fs.write\", \"exec\", "+
		"\"proc.interactive\", \"proc.list\", \"proc.kill\", \"net.fetch\"]\nhistory = true\nfeedback_url = \"https://example.invalid/feedback\"\n%s"+
		"[limits]\ndefault_timeout_ms = 5000\nmax_timeout_ms = 30000\n", root, extra)
	os.WriteFile(filepath.Join(cfg, "policy.toml"), []byte(pol), 0o600)
	return cfg, policyResolve(root)
}

func policyResolve(p string) string { r, _ := filepath.EvalSymlinks(p); return r }

// connect starts `devagotchi mcp` over stdio with the official client. roots (optional) are the client's MCP roots.
func connect(t *testing.T, cfg string, roots ...string) *mcp.ClientSession {
	t.Helper()
	c := mcp.NewClient(&mcp.Implementation{Name: "parity-test", Version: "1"}, nil)
	for _, r := range roots {
		c.AddRoots(&mcp.Root{URI: "file://" + filepath.ToSlash(r), Name: filepath.Base(r)})
	}
	cmd := exec.Command(bin, "mcp", "--config-dir", cfg, "--quiet")
	cs, err := c.Connect(context.Background(), &mcp.CommandTransport{Command: cmd}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cs.Close() })
	return cs
}

type res struct {
	text string
	err  bool
	data map[string]any
	imgs int
}

func callTool(t *testing.T, cs *mcp.ClientSession, name string, args map[string]any) res {
	t.Helper()
	r, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s: protocol error %v", name, err)
	}
	out := res{err: r.IsError}
	for _, c := range r.Content {
		switch v := c.(type) {
		case *mcp.TextContent:
			out.text += v.Text
		case *mcp.ImageContent:
			out.imgs++
		}
	}
	out.data, _ = r.StructuredContent.(map[string]any)
	return out
}

// want asserts success (or the given error code) and that every substring appears.
func want(t *testing.T, r res, code string, subs ...string) res {
	t.Helper()
	if code == "" && r.err || code != "" && (!r.err || r.data["error"] != code) {
		t.Fatalf("want error %q, got err=%v data=%v text:\n%s", code, r.err, r.data["error"], r.text)
	}
	for _, s := range subs {
		if !strings.Contains(r.text, s) {
			t.Fatalf("missing %q in:\n%s", s, r.text)
		}
	}
	return r
}
