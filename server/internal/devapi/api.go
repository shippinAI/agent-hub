// Package devapi mounts the Devagotchi REST API (/api/v1, 06) and the bridge endpoints (/bridge/v1).
package devapi

import (
	"net/http"

	"agenthub/internal/access"
	"agenthub/internal/bridgehub"
	"agenthub/internal/catalog"
	"agenthub/internal/changepolicy"
	"agenthub/internal/guides"
	"agenthub/internal/httpapi"
	"agenthub/internal/machineops"
	"agenthub/internal/machines"
	"agenthub/internal/store"
	"agenthub/internal/subs"
)

// API holds the domain services.
type API struct {
	Srv      *httpapi.Server
	St       *store.Store
	Machines *machines.Service
	Access   *access.Service
	Guides   *guides.Service
	Subs     *subs.Service
	Catalog  *catalog.Service
	Ops      *machineops.Ops
	Hub      *bridgehub.Hub
	Policy   *changepolicy.Service
}

type h = httpapi.Handler

// Mount registers every route.
func (a *API) Mount(mux *http.ServeMux) {
	reg := func(pattern string, fn h) { mux.HandleFunc(pattern, a.Srv.Authed(fn)) }
	a.machineRoutes(reg)
	a.accessRoutes(reg)
	a.guideRoutes(reg)
	a.subRoutes(reg)
	a.catalogRoutes(reg)
	a.policyRoutes(reg)
	a.bridgeRoutes(mux)
}

func body[T any](r *http.Request) (T, string, error) {
	var v T
	rid, err := httpapi.Decode(r, &v)
	return v, rid, err
}

func humanOnly(p *store.Principal) error {
	if !p.IsHuman {
		return store.Fail(403, "humans_only", "this screen is for humans")
	}
	return nil
}
