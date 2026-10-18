package mcpserve

import (
	"encoding/json"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"devagotchi/bridge/history"
	"devagotchi/bridge/tools"
)

// result maps a tool Out to MCP content: text, images, and structured data (always an object).
func result(o tools.Out) *mcp.CallToolResult {
	r := &mcp.CallToolResult{IsError: o.Code != ""}
	r.Content = append(r.Content, &mcp.TextContent{Text: o.Text})
	for _, im := range o.Images {
		r.Content = append(r.Content, &mcp.ImageContent{Data: im.Data, MIMEType: im.Mime})
	}
	sc := map[string]any{}
	if o.Data != nil {
		b, _ := json.Marshal(o.Data)
		if json.Unmarshal(b, &sc) != nil {
			sc = map[string]any{"value": json.RawMessage(b)}
		}
	}
	if o.Code != "" {
		sc["error"] = o.Code
	}
	if len(sc) > 0 {
		r.StructuredContent = sc
	}
	return r
}

func (s *Server) record(tool string, args json.RawMessage, o tools.Out, client string, start time.Time) {
	if s.env.History == nil {
		return
	}
	s.env.History.Record(history.Call{At: start, Tool: tool, Args: args, OK: o.Code == "", Error: o.Code,
		DurationMS: time.Since(start).Milliseconds(), Output: o.Text, Client: client})
}
