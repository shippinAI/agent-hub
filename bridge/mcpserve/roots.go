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
// Before protocol 2026-07-28 roots are fetched after initialize and on roots/list_changed; from 2026-07-28 on,
// server-to-client requests are forbidden, so the first tool call asks for them as a multi-round-trip input request.
type rootCache struct {
	mu  sync.Mutex
	got map[*mcp.ServerSession]*rootEntry
}

type rootEntry struct {
	ready chan struct{}
	roots []string
}

func newRootCache() *rootCache { return &rootCache{got: map[*mcp.ServerSession]*rootEntry{}} }

// refresh (re)fetches a session's roots in the background.
func (c *rootCache) refresh(ss *mcp.ServerSession) {
	e := &rootEntry{ready: make(chan struct{})}
	c.mu.Lock()
	c.got[ss] = e
	c.mu.Unlock()
	go func() {
		e.roots = fetchRoots(ss)
		close(e.ready)
	}()
}

func (c *rootCache) gate(ctx context.Context, ss *mcp.ServerSession, base *policy.Gate) *policy.Gate {
	c.mu.Lock()
	e := c.got[ss]
	c.mu.Unlock()
	if e == nil {
		return base
	}
	select {
	case <-e.ready:
	case <-time.After(3 * time.Second):
	case <-ctx.Done():
	}
	select {
	case <-e.ready:
		if len(e.roots) > 0 {
			return base.Narrow(e.roots)
		}
	default:
	}
	return base
}

func fetchRoots(ss *mcp.ServerSession) []string {
	ip := ss.InitializeParams()
	if ip == nil || ip.Capabilities == nil || ip.Capabilities.RootsV2 == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	res, err := ss.ListRoots(ctx, &mcp.ListRootsParams{})
	if err != nil || res == nil {
		return nil
	}
	return rootPaths(res)
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
