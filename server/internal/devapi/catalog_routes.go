package devapi

import (
	"encoding/json"
	"net/http"

	"agenthub/internal/catalog"
	"agenthub/internal/store"
)

func (a *API) catalogRoutes(reg func(string, h)) {
	for _, k := range catalog.Kinds() {
		kind := k
		reg("GET /api/v1/"+kind, func(w http.ResponseWriter, r *http.Request, p *store.Principal) (any, error) {
			return a.Catalog.List(r.Context(), kind)
		})
		put := func(w http.ResponseWriter, r *http.Request, p *store.Principal) (any, error) {
			in, rid, err := body[map[string]json.RawMessage](r)
			if err != nil {
				return nil, err
			}
			id := r.PathValue("id")
			if id == "" {
				_ = json.Unmarshal(in["id"], &id)
			}
			return a.Catalog.Put(r.Context(), p, rid, kind, id, in)
		}
		reg("POST /api/v1/"+kind, put)
		reg("PUT /api/v1/"+kind+"/{id}", put)
		reg("PATCH /api/v1/"+kind+"/{id}", put)
		reg("DELETE /api/v1/"+kind+"/{id}", func(w http.ResponseWriter, r *http.Request, p *store.Principal) (any, error) {
			return a.Catalog.Delete(r.Context(), p, r.Header.Get("Idempotency-Key"), kind, r.PathValue("id"))
		})
	}
}
