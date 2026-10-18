package parity

import (
	"context"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// TestParityProtocol covers rows P1-P4: tool list, prompts, resources, and client roots narrowing.
func TestParityProtocol(t *testing.T) {
	cfg, root := fixture(t, "")
	cs := connect(t, cfg, filepath.Join(root, "sub"))
	ctx := context.Background()
	tl, err := cs.ListTools(ctx, nil)
	if err != nil || len(tl.Tools) < 28 {
		t.Fatalf("tools: %v %d", err, len(tl.Tools))
	}
	pr, err := cs.ListPrompts(ctx, nil)
	if err != nil || len(pr.Prompts) == 0 {
		t.Fatalf("prompts: %v", err)
	}
	rr, err := cs.ReadResource(ctx, &mcp.ReadResourceParams{URI: "devagotchi://config"})
	if err != nil || !strings.Contains(rr.Contents[0].Text, "roots") {
		t.Fatalf("resource: %v", err)
	}
	want(t, callTool(t, cs, "read_file", map[string]any{"path": filepath.Join(root, "sub", "code.go")}), "", "Needle")
	want(t, callTool(t, cs, "read_file", map[string]any{"path": filepath.Join(root, "notes.txt")}), "grant_required", "MCP roots")
}

// TestParityHTTP covers row P5: streamable HTTP with the bearer token.
func TestParityHTTP(t *testing.T) {
	cfg, root := fixture(t, "http_addr = \"127.0.0.1:0\"\n")
	cmd := exec.Command(bin, "mcp", "--config-dir", cfg, "--http", "--addr", "127.0.0.1:47677")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { cmd.Process.Kill(); cmd.Wait() }()
	var tok []byte
	for i := 0; i < 50 && len(tok) == 0; i++ {
		time.Sleep(100 * time.Millisecond)
		tok, _ = os.ReadFile(filepath.Join(cfg, "http-token"))
	}
	if resp, err := http.Post("http://127.0.0.1:47677/mcp", "application/json", strings.NewReader("{}")); err != nil || resp.StatusCode != 401 {
		t.Fatalf("want 401 without token, got %v %v", resp, err)
	}
	hc := &http.Client{Transport: bearer{strings.TrimSpace(string(tok))}}
	c := mcp.NewClient(&mcp.Implementation{Name: "parity-http", Version: "1"}, nil)
	cs, err := c.Connect(context.Background(), &mcp.StreamableClientTransport{Endpoint: "http://127.0.0.1:47677/mcp", HTTPClient: hc}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()
	want(t, callTool(t, cs, "read_file", map[string]any{"path": filepath.Join(root, "notes.txt"), "offset": -1}), "", "line 50")
}

type bearer struct{ tok string }

func (b bearer) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("Authorization", "Bearer "+b.tok)
	return http.DefaultTransport.RoundTrip(r)
}
