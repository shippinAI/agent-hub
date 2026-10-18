package devapi

import (
	"context"
	"encoding/json"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"agenthub/internal/mcptools"
	"agenthub/internal/store"
)

type guiStartIn struct {
	Machine  string   `json:"machine"`
	Monitors []string `json:"monitors,omitempty"`
	TTL      int      `json:"ttl_s,omitempty" jsonschema:"lease seconds, max gui_session_ttl_seconds"`
}

type guiIn struct {
	Session string           `json:"gui_session_id"`
	Fence   int64            `json:"fence,omitempty" jsonschema:"required for gui_input"`
	Region  []int            `json:"region,omitempty" jsonschema:"x,y,w,h"`
	MaxEdge int              `json:"max_edge,omitempty"`
	Actions []map[string]any `json:"actions,omitempty" jsonschema:"gui_input: [{type: move|click|drag|scroll|type|key, x, y, text, key}] in screenshot coordinates"`
	Window  string           `json:"window,omitempty"`
	Op      string           `json:"op,omitempty" jsonschema:"gui_window: list|focus|move|resize|minimise; gui_app: launch|quit"`
	App     string           `json:"app,omitempty"`
}

func (a *API) guiTools(s *mcp.Server) {
	mcptools.Tool(s, "gui_session_start", "Take the machine's GUI lease (one per machine) under gui.observe; returns session id, fence and display size.",
		func(ctx context.Context, p *store.Principal, in guiStartIn) (any, error) {
			return a.Ops.GUIStart(ctx, p, in.Machine, in.Monitors, in.TTL)
		})
	mcptools.Tool(s, "gui_session_end", "End your GUI session.", func(ctx context.Context, p *store.Principal, in guiIn) (any, error) {
		return a.Ops.GUIEnd(ctx, p, in.Session, "agent")
	})
	mcp.AddTool(s, &mcp.Tool{Name: "gui_screenshot", Description: "Screenshot (scaled to max_edge) returned as an image to you only, plus sha256. Never stored unless retention says so."},
		func(ctx context.Context, req *mcp.CallToolRequest, in guiIn) (*mcp.CallToolResult, any, error) {
			p := mcptools.PrincipalOf(req)
			res, err := a.Ops.GUIOp(ctx, p, in.Session, in.Fence, "gui.screenshot", map[string]any{"region": in.Region, "max_edge": in.MaxEdge}, "")
			if err != nil {
				return mcptools.ErrResult(err), nil, nil
			}
			if !res.OK {
				return mcptools.ErrResult(store.Fail(422, res.Error, "%s", res.Message)), nil, nil
			}
			var d struct {
				PNG []byte `json:"png"`
				SHA string `json:"sha256"`
				W   int    `json:"width"`
				H   int    `json:"height"`
			}
			_ = json.Unmarshal(res.Data, &d)
			meta, _ := json.Marshal(map[string]any{"sha256": d.SHA, "width": d.W, "height": d.H})
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.ImageContent{Data: d.PNG, MIMEType: "image/png"}, &mcp.TextContent{Text: string(meta)}}}, nil, nil
		})
	for _, t := range []struct{ name, op, desc string }{
		{"gui_input", "gui.input", "Send input actions; each is checked against the grant's apps/actions and the foreground app. Typed text is never logged."},
		{"gui_tree", "gui.tree", "Accessibility tree of a window (depth capped by the grant)."},
		{"gui_window", "gui.window", "List, focus, move, resize or minimise windows of granted apps."},
		{"gui_app", "gui.app", "Launch or quit a granted app."},
	} {
		t := t
		mcptools.Tool(s, t.name, t.desc, func(ctx context.Context, p *store.Principal, in guiIn) (any, error) {
			return a.Ops.GUIOp(ctx, p, in.Session, in.Fence, t.op, map[string]any{"actions": in.Actions, "window": in.Window, "op": in.Op, "app": in.App}, in.App)
		})
	}
}
