package mcpserve

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"devagotchi/bridge/policy"
)

const newProtocol = "2026-07-28"

// resolve returns the gate for this call, or an input-required result asking the client for its roots.
func (c *rootCache) resolve(ctx context.Context, req *mcp.CallToolRequest, base *policy.Gate) (*policy.Gate, *mcp.CallToolResult) {
	ss := req.Session
	if ss == nil {
		return base, nil
	}
	ip := ss.InitializeParams()
	if ip == nil || ip.ProtocolVersion < newProtocol {
		return c.gate(ctx, ss, base), nil
	}
	if r, ok := req.Params.InputResponses["roots"].(*mcp.ListRootsResult); ok {
		e := &rootEntry{ready: make(chan struct{}), roots: rootPaths(r)}
		close(e.ready)
		c.mu.Lock()
		c.got[ss] = e
		c.mu.Unlock()
	}
	c.mu.Lock()
	_, known := c.got[ss]
	c.mu.Unlock()
	if !known && ip.Capabilities != nil && ip.Capabilities.RootsV2 != nil {
		return nil, &mcp.CallToolResult{InputRequests: mcp.InputRequestMap{"roots": &mcp.ListRootsParams{}}}
	}
	return c.gate(ctx, ss, base), nil
}

// forget drops cached roots (roots/list_changed on the new protocol: ask again on the next call).
func (c *rootCache) forget(ss *mcp.ServerSession) {
	c.mu.Lock()
	delete(c.got, ss)
	c.mu.Unlock()
}

func rootPaths(r *mcp.ListRootsResult) []string {
	var out []string
	for _, x := range r.Roots {
		if p := fileURIPath(x.URI); p != "" {
			out = append(out, p)
		}
	}
	return out
}
