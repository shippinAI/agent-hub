package devapi

import (
	"net/http"

	"agenthub/internal/store"
	"agenthub/internal/subs"
)

func (a *API) subRoutes(reg func(string, h)) {
	reg("GET /api/v1/subscriptions", func(w http.ResponseWriter, r *http.Request, p *store.Principal) (any, error) {
		return a.Subs.List(r.Context())
	})
	put := func(w http.ResponseWriter, r *http.Request, p *store.Principal) (any, error) {
		in, rid, err := body[subs.Input](r)
		if err != nil {
			return nil, err
		}
		if id := r.PathValue("id"); id != "" {
			in.ID = id
		}
		return a.Subs.Put(r.Context(), p, rid, in)
	}
	reg("POST /api/v1/subscriptions", put)
	reg("PATCH /api/v1/subscriptions/{id}", put)
	reg("PUT /api/v1/subscriptions/{id}/secret", func(w http.ResponseWriter, r *http.Request, p *store.Principal) (any, error) {
		in, rid, err := body[struct {
			Value string `json:"value"`
		}](r)
		if err != nil {
			return nil, err
		}
		return a.Subs.PutSecret(r.Context(), p, rid, r.PathValue("id"), in.Value)
	})
	reg("POST /api/v1/subscriptions/{id}/test", func(w http.ResponseWriter, r *http.Request, p *store.Principal) (any, error) {
		return a.Subs.Test(r.Context(), p, r.Header.Get("Idempotency-Key"), r.PathValue("id"))
	})
	reg("GET /api/v1/role-bindings", func(w http.ResponseWriter, r *http.Request, p *store.Principal) (any, error) {
		return a.Subs.Bindings(r.Context())
	})
	bind := func(w http.ResponseWriter, r *http.Request, p *store.Principal) (any, error) {
		in, rid, err := body[subs.Binding](r)
		if err != nil {
			return nil, err
		}
		if id := r.PathValue("id"); id != "" {
			in.ID = id
		}
		return a.Subs.PutBinding(r.Context(), p, rid, in)
	}
	reg("POST /api/v1/role-bindings", bind)
	reg("PATCH /api/v1/role-bindings/{id}", bind)
	for _, k := range []string{"subscriptions", "role-bindings"} {
		kind := k
		reg("DELETE /api/v1/"+k+"/{id}", func(w http.ResponseWriter, r *http.Request, p *store.Principal) (any, error) {
			return a.Subs.Delete(r.Context(), p, r.Header.Get("Idempotency-Key"), kind, r.PathValue("id"))
		})
	}
	reg("GET /api/v1/role-route", func(w http.ResponseWriter, r *http.Request, p *store.Principal) (any, error) {
		return a.Subs.Resolve(r.Context(), r.URL.Query().Get("role"), r.URL.Query().Get("chat_app"))
	})
}
