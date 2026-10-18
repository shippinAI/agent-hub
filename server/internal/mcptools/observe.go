package mcptools

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"agenthub/internal/store"
)

// ToolCall is one finished MCP tool call, handed to Ext.Observe (the action feed records it, 39).
type ToolCall struct {
	Principal *store.Principal
	Tool      string
	Args      json.RawMessage
	IsError   bool
	Output    string // the tool's text content (the observer redacts and caps it)
	Duration  time.Duration
}

// observe wraps tools/call so every tool call, including failures, reaches fn after it completes.
func observe(fn func(context.Context, ToolCall)) mcp.Middleware {
	return func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
			if method != "tools/call" {
				return next(ctx, method, req)
			}
			start := time.Now()
			res, err := next(ctx, method, req)
			tc := ToolCall{Duration: time.Since(start), IsError: err != nil}
			if r, ok := req.(*mcp.CallToolRequest); ok {
				tc.Principal = principal(r)
				if r.Params != nil {
					tc.Tool, tc.Args = r.Params.Name, r.Params.Arguments
				}
			}
			if out, ok := res.(*mcp.CallToolResult); ok && out != nil {
				tc.IsError = tc.IsError || out.IsError
				var b strings.Builder
				for _, c := range out.Content {
					if t, ok := c.(*mcp.TextContent); ok {
						b.WriteString(t.Text)
					}
				}
				tc.Output = b.String()
			} else if err != nil {
				tc.Output = err.Error()
			}
			if tc.Principal != nil {
				fn(context.WithoutCancel(ctx), tc)
			}
			return res, err
		}
	}
}
