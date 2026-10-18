// Package mcptools exposes the hub as a remote MCP server (Streamable HTTP, stateless, bearer per agent).
package mcptools

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	hubauth "agenthub/internal/auth"
	"agenthub/internal/store"
	"agenthub/internal/stream"
)

// Instructions: the first 512 characters are self-contained (Codex truncates there).
const Instructions = `agent-hub: shared task board for many agents. Loop: claim_next (or list_ready then claim) -> work -> ` +
	`heartbeat {task_id, fence} every heartbeat_every_seconds -> add_note for decisions/blockers -> complete {fence, text: summary} ` +
	`or handoff {note, to?}. Every write on a held task needs the fence from your claim; update also needs version. ` +
	`stale_fence / lease_expired / not_holder = you lost the task: stop writing and re-read. Reuse request_id when retrying. ` +
	`Identity = your token.

Errors come back as isError with {"error":{"code","message","current"}}. version_conflict returns the current task: ` +
	`re-apply your change on top of it. wait_for_updates long-polls the event log (after_seq).`

type deps struct {
	st *store.Store
	br *stream.Broker
}

// Handler returns the /mcp handler wrapped in bearer-token auth.
func Handler(st *store.Store, br *stream.Broker, res *hubauth.Resolver, ext Ext) http.Handler {
	s := mcp.NewServer(&mcp.Implementation{Name: "agent-hub", Version: "0.1.0"},
		&mcp.ServerOptions{Instructions: Instructions + ext.Instructions})
	d := deps{st: st, br: br}
	d.readTools(s)
	d.leaseTools(s)
	d.editTools(s)
	if ext.Register != nil {
		ext.Register(s)
	}
	h := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return s },
		&mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true})
	verify := func(ctx context.Context, tok string, _ *http.Request) (*auth.TokenInfo, error) {
		p, err := res.Token(ctx, tok)
		if err != nil || p.Kind == "machine" {
			return nil, auth.ErrInvalidToken
		}
		return &auth.TokenInfo{UserID: p.ID, Extra: map[string]any{"principal": p}}, nil
	}
	return auth.RequireBearerToken(verify, &auth.RequireBearerTokenOptions{AllowMissingExpiration: true})(onInit(h, ext.OnInit))
}

func principal(req *mcp.CallToolRequest) *store.Principal {
	if req.Extra == nil || req.Extra.TokenInfo == nil {
		return nil
	}
	p, _ := req.Extra.TokenInfo.Extra["principal"].(*store.Principal)
	return p
}

// tool registers a typed tool whose result is JSON text and whose errors use the REST error codes.
func tool[In any](s *mcp.Server, name, desc string, fn func(ctx context.Context, p *store.Principal, in In) (any, error)) {
	mcp.AddTool(s, &mcp.Tool{Name: name, Description: desc},
		func(ctx context.Context, req *mcp.CallToolRequest, in In) (*mcp.CallToolResult, any, error) {
			p := principal(req)
			if p == nil {
				return errResult(&store.Error{Code: "unauthenticated", Message: "no principal"}), nil, nil
			}
			out, err := fn(ctx, p, in)
			if err != nil {
				return errResult(err), nil, nil
			}
			b, err := json.Marshal(out)
			if err != nil {
				return nil, nil, err
			}
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(b)}}}, nil, nil
		})
}

func errResult(err error) *mcp.CallToolResult {
	e, ok := store.AsError(err)
	if !ok {
		e = &store.Error{Code: "internal", Message: "internal error"}
	}
	b, _ := json.Marshal(map[string]any{"error": e})
	return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: string(b)}}}
}
