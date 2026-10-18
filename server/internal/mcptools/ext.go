package mcptools

import (
	"bytes"
	"context"
	"io"
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"agenthub/internal/store"
)

// Ext extends the MCP server: more tools/resources/prompts, an instructions suffix, and a hook that runs
// when a client initializes a session (resume alerts for risky grants, 01 invariant 7).
type Ext struct {
	Register     func(*mcp.Server)
	Instructions string
	OnInit       func(ctx context.Context, p *store.Principal)
}

// Tool registers a typed JSON tool with the hub error envelope (for other packages).
func Tool[In any](s *mcp.Server, name, desc string, fn func(ctx context.Context, p *store.Principal, in In) (any, error)) {
	tool(s, name, desc, fn)
}

// ErrResult renders an error as an MCP tool error.
func ErrResult(err error) *mcp.CallToolResult { return errResult(err) }

// PrincipalOf returns the caller of a tool request.
func PrincipalOf(req *mcp.CallToolRequest) *store.Principal { return principal(req) }

func onInit(h http.Handler, fn func(context.Context, *store.Principal)) http.Handler {
	if fn == nil {
		return h
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && r.Body != nil {
			b, _ := io.ReadAll(io.LimitReader(r.Body, 4<<20))
			r.Body = io.NopCloser(bytes.NewReader(b))
			if bytes.Contains(b, []byte(`"method":"initialize"`)) || bytes.Contains(b, []byte(`"method": "initialize"`)) {
				if ti := auth.TokenInfoFromContext(r.Context()); ti != nil {
					if p, ok := ti.Extra["principal"].(*store.Principal); ok {
						go fn(context.WithoutCancel(r.Context()), p)
					}
				}
			}
		}
		h.ServeHTTP(w, r)
	})
}
