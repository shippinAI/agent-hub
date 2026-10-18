package mcpserve

import (
	"context"
	"encoding/json"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"devagotchi/bridge/tools"
)

// addResources exposes the effective policy and the recent call history as read-only resources.
func addResources(s *mcp.Server, env *tools.Env) {
	s.AddResource(&mcp.Resource{URI: "devagotchi://config", Name: "config", Title: "Effective policy", MIMEType: "application/json",
		Description: "What get_config returns: roots, capabilities, grants, limits, sandbox (no secrets)."},
		func(ctx context.Context, r *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
			t, _ := tools.Find("get_config")
			return jsonResource(r.Params.URI, t.Run(ctx, env, nil).Data), nil
		})
	s.AddResource(&mcp.Resource{URI: "devagotchi://history/recent", Name: "recent-calls", Title: "Recent tool calls",
		MIMEType: "application/json", Description: "The last 50 tool calls (redacted), kept on this machine only."},
		func(ctx context.Context, r *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
			var calls any = []any{}
			if env.History != nil {
				calls = env.History.Recent(50, "", time.Time{})
			}
			return jsonResource(r.Params.URI, calls), nil
		})
}

func jsonResource(uri string, v any) *mcp.ReadResourceResult {
	b, _ := json.MarshalIndent(v, "", "  ")
	return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{{URI: uri, MIMEType: "application/json", Text: string(b)}}}
}
