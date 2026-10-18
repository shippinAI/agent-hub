package devapi

import (
	"context"
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"agenthub/internal/access"
	"agenthub/internal/auth"
	"agenthub/internal/bridgehub"
	"agenthub/internal/catalog"
	"agenthub/internal/config"
	"agenthub/internal/guides"
	"agenthub/internal/httpapi"
	"agenthub/internal/machineops"
	"agenthub/internal/machines"
	"agenthub/internal/mcptools"
	"agenthub/internal/secretbox"
	"agenthub/internal/store"
	"agenthub/internal/stream"
	"agenthub/internal/subs"
)

// Build wires the whole server: hub REST + SSE + MCP plus the Devagotchi services and bridge endpoints.
func Build(ctx context.Context, st *store.Store, res *auth.Resolver, br *stream.Broker, cfg config.Config, box *secretbox.Box) (*httpapi.Server, *API) {
	hub := bridgehub.New()
	ms := &machines.Service{St: st, Box: box}
	hub.Seen = func(m string, connected bool) { ms.Seen(context.Background(), m, connected) }
	acc := &access.Service{St: st}
	ops := &machineops.Ops{St: st, Access: acc, Machines: ms, Hub: hub}
	acc.Notify = ops
	a := &API{St: st, Machines: ms, Access: acc, Ops: ops, Hub: hub, Catalog: &catalog.Service{St: st},
		Guides: &guides.Service{St: st, HubURL: cfg.PublicURL}, Subs: &subs.Service{St: st, Box: box}}
	a.Guides.Hooks = guideHooks{a}
	ext := mcptools.Ext{Instructions: Instructions, Register: func(s *mcp.Server) {
		a.accessTools(s)
		a.opsTools(s)
		a.guiTools(s)
		a.guideTools(s)
		a.guideResources(ctx, s)
	}, OnInit: func(c context.Context, p *store.Principal) { _ = acc.AgentResumed(c, p.ID, "mcp_session") }}
	srv := &httpapi.Server{St: st, Auth: res, Broker: br, Cfg: cfg, MCP: mcptools.Handler(st, br, res, ext)}
	a.Srv = srv
	srv.Extra = func(mux *http.ServeMux) { a.Mount(mux) }
	return srv, a
}
