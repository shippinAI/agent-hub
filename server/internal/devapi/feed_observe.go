package devapi

import (
	"context"
	"log"
	"slices"

	"agenthub/internal/actionfeed"
	"agenthub/internal/mcptools"
)

// observeTool records every MCP tool call in the action feed (category "tool"), except audit_tool_skip.
func (a *API) observeTool(ctx context.Context, c mcptools.ToolCall) {
	set, _ := a.St.LoadSettings(ctx)
	if slices.Contains(set.Strings("audit_tool_skip"), c.Tool) {
		return
	}
	e := actionfeed.Event{Category: "tool", Kind: actionfeed.KindTool, Actor: c.Principal.ID, Op: "mcp.tool", Tool: c.Tool,
		Args: c.Args, Status: "ok", Stdout: c.Output, DurationMS: c.Duration.Milliseconds()}
	if c.IsError {
		e.Status, e.Error, e.Stdout = "error", c.Output, ""
	}
	if _, err := a.Feed.Record(ctx, e); err != nil {
		log.Printf("actionfeed: tool %s: %v", c.Tool, err)
	}
}
