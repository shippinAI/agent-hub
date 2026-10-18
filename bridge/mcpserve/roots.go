package mcpserve

import (
	"context"
	"net/url"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"devagotchi/bridge/policy"
)

// rootCache remembers each session's MCP roots. Client roots can only narrow the policy, never widen it.
type rootCache struct {
	mu  sync.Mutex
	got map[*mcp.ServerSession][]string
}

func newRootCache() *rootCache { return &rootCache{got: map[*mcp.ServerSession][]string{}} }

func (c *rootCache) forget(ss *mcp.ServerSession) {
	c.mu.Lock()
	delete(c.got, ss)
	c.mu.Unlock()
}

func (c *rootCache) gate(ctx context.Context, ss *mcp.ServerSession, base *policy.Gate) *policy.Gate {
	if ss == nil {
		return base
	}
	c.mu.Lock()
	roots, seen := c.got[ss]
	c.mu.Unlock()
	if !seen {
		roots = fetchRoots(ctx, ss)
		c.mu.Lock()
		c.got[ss] = roots
		c.mu.Unlock()
	}
	if len(roots) == 0 {
		return base
	}
	return base.Narrow(roots)
}

func fetchRoots(ctx context.Context, ss *mcp.ServerSession) []string {
	ip := ss.InitializeParams()
	if ip == nil || ip.Capabilities == nil || ip.Capabilities.RootsV2 == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	res, err := ss.ListRoots(ctx, &mcp.ListRootsParams{})
	if err != nil || res == nil {
		return nil
	}
	var out []string
	for _, r := range res.Roots {
		if p := fileURIPath(r.URI); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func fileURIPath(uri string) string {
	u, err := url.Parse(uri)
	if err != nil || u.Scheme != "file" {
		return ""
	}
	p := u.Path
	if runtime.GOOS == "windows" {
		p = strings.TrimPrefix(p, "/")
	}
	return p
}
