package hubtest

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type bearer struct{ tok string }

func (b bearer) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("Authorization", "Bearer "+b.tok)
	return http.DefaultTransport.RoundTrip(r)
}

func (e *env) mcpSession(who string) *mcp.ClientSession {
	c := mcp.NewClient(&mcp.Implementation{Name: "hubtest-" + who, Version: "0"}, nil)
	cs, err := c.Connect(context.Background(), &mcp.StreamableClientTransport{Endpoint: e.url + "/mcp",
		HTTPClient: &http.Client{Transport: bearer{e.tokens[who]}}, DisableStandaloneSSE: true}, nil)
	if err != nil {
		e.t.Fatalf("mcp connect as %s: %v", who, err)
	}
	e.t.Cleanup(func() { cs.Close() })
	return cs
}

func callTool(t *testing.T, cs *mcp.ClientSession, name string, args map[string]any) (map[string]any, bool) {
	t.Helper()
	r, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	out := map[string]any{}
	if len(r.Content) > 0 {
		_ = json.Unmarshal([]byte(r.Content[0].(*mcp.TextContent).Text), &out)
	}
	return out, r.IsError
}

// A real MCP client (official Go SDK) over Streamable HTTP with per-agent bearer tokens.
func TestMCPToolsOverHTTP(t *testing.T) {
	e := setup(t)
	cs := e.mcpSession("claude-code")
	if ins := cs.InitializeResult().Instructions; !strings.HasPrefix(ins, "agent-hub:") || len(ins[:512]) != 512 {
		t.Errorf("instructions: %q", ins)
	}
	tools, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, tl := range tools.Tools {
		names[tl.Name] = true
	}
	for _, n := range []string{"whoami", "list_ready", "claim_next", "claim", "heartbeat", "update", "add_note", "handoff", "release", "complete", "search", "create_task", "get_task", "wait_for_updates"} {
		if !names[n] {
			t.Errorf("missing tool %s", n)
		}
	}
	who, _ := callTool(t, cs, "whoami", nil)
	if who["principal"].(map[string]any)["id"] != "claude-code" {
		t.Fatalf("identity must come from the token: %v", who)
	}
	created, isErr := callTool(t, cs, "create_task", map[string]any{"project": "general", "title": "via MCP", "labels": []string{"mcp"}, "request_id": "c1"})
	again, _ := callTool(t, cs, "create_task", map[string]any{"project": "general", "title": "via MCP", "labels": []string{"mcp"}, "request_id": "c1"})
	if isErr || num(created["id"]) != num(again["id"]) {
		t.Fatalf("create + idempotent retry: %v / %v", created, again)
	}
	ready, _ := callTool(t, cs, "list_ready", map[string]any{})
	if len(ready["tasks"].([]any)) != 1 {
		t.Errorf("list_ready: %v", ready)
	}
	l, _ := callTool(t, cs, "claim_next", map[string]any{"ttl_seconds": 120})
	fence := l["fence"]
	if l["claimed"] != true {
		t.Fatalf("claim_next: %v", l)
	}
	id := num(created["id"])
	if b, isErr := callTool(t, cs, "heartbeat", map[string]any{"task_id": id, "fence": fence, "progress": map[string]any{"step": 1}}); isErr {
		t.Errorf("heartbeat: %v", b)
	}
	if n, isErr := callTool(t, cs, "add_note", map[string]any{"task_id": id, "body": "chose plan B", "kind": "decision", "fence": fence}); isErr {
		t.Errorf("add_note: %v", n)
	}
	bad, isErr := callTool(t, cs, "heartbeat", map[string]any{"task_id": id, "fence": num(fence) - 1})
	if !isErr || bad["error"].(map[string]any)["code"] != "stale_fence" {
		t.Errorf("stale fence via MCP: %v", bad)
	}
	other := e.mcpSession("codex")
	if r, _ := callTool(t, other, "claim", map[string]any{"task_id": id}); r["error"].(map[string]any)["code"] != "held_by_other" {
		t.Errorf("second agent claim: %v", r)
	}
	h, isErr := callTool(t, cs, "handoff", map[string]any{"task_id": id, "fence": fence, "note": "API done; tests next", "to": "codex"})
	if isErr || h["assignee"] != "codex" || h["holder"] != nil {
		t.Fatalf("handoff: %v", h)
	}
	l2, _ := callTool(t, other, "claim_next", map[string]any{})
	if l2["claimed"] != true || num(l2["task"].(map[string]any)["id"]) != id {
		t.Fatalf("assignee picks up the handoff: %v", l2)
	}
	got, _ := callTool(t, other, "get_task", map[string]any{"task_id": id})
	kinds := []string{}
	for _, n := range got["notes"].([]any) {
		kinds = append(kinds, n.(map[string]any)["kind"].(string))
	}
	if strings.Join(kinds, ",") != "decision,handoff" {
		t.Errorf("handoff thread: %v", kinds)
	}
	done, isErr := callTool(t, other, "complete", map[string]any{"task_id": id, "fence": l2["fence"], "text": "tests added"})
	if isErr || done["state"] != "review" {
		t.Errorf("complete: %v", done)
	}
	w, _ := callTool(t, other, "wait_for_updates", map[string]any{"after_seq": 0, "timeout_seconds": 1})
	if len(w["events"].([]any)) < 5 {
		t.Errorf("wait_for_updates: %v", w["head_seq"])
	}
	s, _ := callTool(t, other, "search", map[string]any{"query": "plan B"})
	if len(s["tasks"].([]any)) != 1 {
		t.Errorf("search by note text: %v", s)
	}
	resp, _ := http.Post(e.url+"/mcp", "application/json", strings.NewReader(`{}`))
	if resp.StatusCode != 401 {
		t.Errorf("/mcp without token: %d", resp.StatusCode)
	}
}
