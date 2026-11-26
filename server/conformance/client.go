// Package conformance is a black-box test suite for the agent-hub protocol: it talks to a running hub only over
// REST (/api/v1) and MCP (/mcp) and checks the contract in docs/spec.md. The Go server runs it in hubtest; any
// other build (a Rust twin) runs it with HUB_CONFORMANCE_URL and HUB_CONFORMANCE_TOKENS, see docs/conformance.md.
//
// The suite needs three principals: a human admin and two agents with the default agent policies. Every case
// works in a project of its own, so it can run against a hub that already holds other work.
package conformance

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Target is the hub under test: its base URL (no /api/v1) and bearer tokens.
type Target struct {
	URL    string `json:"url"`
	Human  string `json:"human"`   // a human with admin rights
	AgentA string `json:"agent_a"` // two distinct agent principals
	AgentB string `json:"agent_b"`
}

type resp struct {
	Status int
	Header http.Header
	Body   map[string]any
}

func (r resp) code() string {
	if e, ok := r.Body["error"].(map[string]any); ok {
		s, _ := e["code"].(string)
		return s
	}
	return ""
}

type hub struct {
	t       *testing.T
	tg      Target
	project string
}

var seq atomic.Int64

// newHub gives a case its own project on the default workflow.
func newHub(t *testing.T, tg Target) *hub {
	h := &hub{t: t, tg: tg, project: fmt.Sprintf("conf-%d-%d", time.Now().UnixNano()%1e9, seq.Add(1))}
	if r := h.call(tg.Human, "PUT", "/projects/"+h.project, map[string]any{"name": h.project, "workflow_id": "default", "version": 0}); r.Status != 200 {
		t.Fatalf("create project: %d %v", r.Status, r.Body)
	}
	t.Cleanup(h.releaseAll) // agents have a WIP limit; leave nothing held for the next case
	return h
}

func (h *hub) releaseAll() {
	ts, _ := h.call(h.tg.Human, "GET", "/tasks?project="+h.project+"&limit=500", nil).Body["tasks"].([]any)
	for _, x := range ts {
		if t := x.(map[string]any); t["holder"] != nil {
			h.call(h.tg.Human, "POST", fmt.Sprintf("/tasks/%d/force-release", num(t["id"])), map[string]any{"reason": "conformance cleanup"})
		}
	}
}

func (h *hub) call(tok, method, path string, body any, hdrs ...string) resp {
	h.t.Helper()
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, strings.TrimRight(h.tg.URL, "/")+"/api/v1"+path, rd)
	req.Header.Set("Content-Type", "application/json")
	if tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	for _, x := range hdrs {
		k, v, _ := strings.Cut(x, ": ")
		req.Header.Set(k, v)
	}
	r, err := http.DefaultClient.Do(req)
	if err != nil {
		h.t.Fatal(err)
	}
	defer r.Body.Close()
	out := resp{Status: r.StatusCode, Header: r.Header, Body: map[string]any{}}
	raw, _ := io.ReadAll(r.Body)
	_ = json.Unmarshal(raw, &out.Body)
	return out
}

func (h *hub) task(title string, extra map[string]any) int64 {
	h.t.Helper()
	body := map[string]any{"title": title, "project": h.project}
	for k, v := range extra {
		body[k] = v
	}
	r := h.call(h.tg.Human, "POST", "/tasks", body)
	if r.Status != 200 {
		h.t.Fatalf("create task: %d %v", r.Status, r.Body)
	}
	return num(r.Body["id"])
}

func (h *hub) claim(tok string, id int64) int64 {
	h.t.Helper()
	r := h.call(tok, "POST", fmt.Sprintf("/tasks/%d/claim", id), map[string]any{})
	if r.Status != 200 || num(r.Body["fence"]) == 0 {
		h.t.Fatalf("claim #%d: %d %v", id, r.Status, r.Body)
	}
	return num(r.Body["fence"])
}

func (h *hub) get(id int64) map[string]any {
	h.t.Helper()
	t, _ := h.call(h.tg.Human, "GET", fmt.Sprintf("/tasks/%d", id), nil).Body["task"].(map[string]any)
	return t
}

func num(v any) int64 { f, _ := v.(float64); return int64(f) }

type bearer struct{ tok string }

func (b bearer) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("Authorization", "Bearer "+b.tok)
	return http.DefaultTransport.RoundTrip(r)
}

func (h *hub) mcp(tok string) *mcp.ClientSession {
	h.t.Helper()
	c := mcp.NewClient(&mcp.Implementation{Name: "hub-conformance", Version: "1"}, nil)
	cs, err := c.Connect(context.Background(), &mcp.StreamableClientTransport{Endpoint: strings.TrimRight(h.tg.URL, "/") + "/mcp",
		HTTPClient: &http.Client{Transport: bearer{tok}}, DisableStandaloneSSE: true}, nil)
	if err != nil {
		h.t.Fatalf("mcp connect: %v", err)
	}
	h.t.Cleanup(func() { cs.Close() })
	return cs
}

func (h *hub) tool(cs *mcp.ClientSession, name string, args map[string]any) (map[string]any, bool) {
	h.t.Helper()
	r, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		h.t.Fatalf("%s: %v", name, err)
	}
	out := map[string]any{}
	if len(r.Content) > 0 {
		if tc, ok := r.Content[0].(*mcp.TextContent); ok {
			_ = json.Unmarshal([]byte(tc.Text), &out)
		}
	}
	return out, r.IsError
}
