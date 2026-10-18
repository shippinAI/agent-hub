package devapi

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"agenthub/internal/mcptools"
	"agenthub/internal/store"
	"devagotchi/bridge/scope"
	"devagotchi/bridge/toolcaps"
)

type machineToolIn struct {
	Machine   string         `json:"machine"`
	Name      string         `json:"name" jsonschema:"bridge tool name (Desktop Commander-compatible: read_file, edit_block, start_search, start_process, ...)"`
	Arguments map[string]any `json:"arguments,omitempty"`
}

// opFor is the op a tool's capability authorizes as (scope.CapabilityFor maps it back).
var opFor = map[string]string{"": "fs.read", "fs.read": "fs.read", "fs.write": "fs.write", "exec": "exec",
	"proc.interactive": "proc.start", "proc.kill": "proc.start", "proc.list": "proc.list", "net.fetch": "net.fetch"}

func (a *API) machineToolTools(s *mcp.Server) {
	mcptools.Tool(s, "machine_tool", "Run one bridge tool on a machine under your grant for its capability "+
		"(the bridge rebuilds the policy from the signed grant: grant paths are the roots).",
		func(ctx context.Context, p *store.Principal, in machineToolIn) (any, error) {
			capability, ok := toolcaps.Cap[in.Name]
			if !ok {
				return nil, store.Fail(400, "invalid", "unknown bridge tool %q", in.Name)
			}
			c := scope.Call{Op: opFor[capability], Argv: strs(in.Arguments["argv"])}
			for _, k := range []string{"path", "file_path", "source", "destination", "cwd", "outputPath"} {
				if v, ok := in.Arguments[k].(string); ok && v != "" && !(k == "path" && in.Arguments["isUrl"] == true) {
					c.Paths = append(c.Paths, v)
				}
			}
			c.Paths = append(c.Paths, strs(in.Arguments["paths"])...)
			if u, err := url.Parse(fmt.Sprint(in.Arguments["path"])); err == nil && in.Arguments["isUrl"] == true {
				c.Hosts = []string{u.Hostname()}
			}
			res, err := a.Ops.Call(ctx, p, in.Machine, c.Op, "tool", map[string]any{"name": in.Name, "arguments": in.Arguments}, c, 120*time.Second)
			if err != nil {
				return nil, err
			}
			return res, nil
		})
}

func strs(v any) []string {
	l, _ := v.([]any)
	out := make([]string, 0, len(l))
	for _, x := range l {
		out = append(out, fmt.Sprint(x))
	}
	return out
}
