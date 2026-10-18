package devapi

import (
	"net/http"

	"agenthub/internal/guidekit"
	"agenthub/internal/guides"
	"agenthub/internal/store"
)

func (a *API) guideRoutes(reg func(string, h)) {
	reg("GET /api/v1/guides", func(w http.ResponseWriter, r *http.Request, p *store.Principal) (any, error) {
		return a.Guides.List(r.Context(), r.URL.Query().Get("q"))
	})
	reg("GET /api/v1/guides/{id}", func(w http.ResponseWriter, r *http.Request, p *store.Principal) (any, error) {
		return a.Guides.Get(r.Context(), r.PathValue("id"))
	})
	reg("PATCH /api/v1/guides/{id}", func(w http.ResponseWriter, r *http.Request, p *store.Principal) (any, error) {
		in, rid, err := body[struct {
			Enabled bool `json:"enabled"`
		}](r)
		if err != nil {
			return nil, err
		}
		return a.Guides.SetEnabled(r.Context(), p, rid, r.PathValue("id"), in.Enabled)
	})
	reg("GET /api/v1/guides/{id}/versions", func(w http.ResponseWriter, r *http.Request, p *store.Principal) (any, error) {
		return a.Guides.Versions(r.Context(), r.PathValue("id"))
	})
	draft := func(w http.ResponseWriter, r *http.Request, p *store.Principal) (any, error) {
		in, rid, err := body[guides.Draft](r)
		if err != nil {
			return nil, err
		}
		if id := r.PathValue("id"); id != "" {
			in.GuideID = id
		}
		return a.Guides.SaveDraft(r.Context(), p, rid, in)
	}
	reg("POST /api/v1/guides", draft)
	reg("POST /api/v1/guides/{id}/versions", draft)
	reg("POST /api/v1/guides/{id}/versions/{v}/publish", func(w http.ResponseWriter, r *http.Request, p *store.Principal) (any, error) {
		return a.Guides.Publish(r.Context(), p, r.Header.Get("Idempotency-Key"), r.PathValue("v"))
	})
	reg("POST /api/v1/guides/{id}/versions/{v}/retire", func(w http.ResponseWriter, r *http.Request, p *store.Principal) (any, error) {
		return a.Guides.Retire(r.Context(), p, r.Header.Get("Idempotency-Key"), r.PathValue("v"))
	})
	reg("GET /api/v1/guides/{id}/diff", func(w http.ResponseWriter, r *http.Request, p *store.Principal) (any, error) {
		return a.Guides.Diff(r.Context(), r.URL.Query().Get("from"), r.URL.Query().Get("to"))
	})
	reg("POST /api/v1/guides/import", func(w http.ResponseWriter, r *http.Request, p *store.Principal) (any, error) {
		in, rid, err := body[guidekit.Bundle](r)
		if err != nil {
			return nil, err
		}
		return a.Guides.Import(r.Context(), p, rid, in)
	})
	reg("GET /api/v1/guide-updates", func(w http.ResponseWriter, r *http.Request, p *store.Principal) (any, error) {
		return a.Guides.Updates(r.Context())
	})
	reg("POST /api/v1/guide-updates/{id}/{decision}", func(w http.ResponseWriter, r *http.Request, p *store.Principal) (any, error) {
		return a.Guides.Decide(r.Context(), p, r.Header.Get("Idempotency-Key"), r.PathValue("id"), r.PathValue("decision"))
	})
	reg("GET /api/v1/guide-runs", func(w http.ResponseWriter, r *http.Request, p *store.Principal) (any, error) {
		return a.Guides.ListRuns(r.Context(), r.URL.Query().Get("machine"), 100)
	})
	reg("POST /api/v1/guide-runs", func(w http.ResponseWriter, r *http.Request, p *store.Principal) (any, error) {
		in, rid, err := body[guides.StartReq](r)
		if err != nil {
			return nil, err
		}
		return a.Guides.Start(r.Context(), p, rid, in)
	})
	reg("GET /api/v1/guide-runs/{id}", func(w http.ResponseWriter, r *http.Request, p *store.Principal) (any, error) {
		run, err := a.Guides.GetRun(r.Context(), r.PathValue("id"))
		if err != nil {
			return nil, err
		}
		return a.Guides.View(r.Context(), run), nil
	})
	reg("POST /api/v1/guide-runs/{id}/step", func(w http.ResponseWriter, r *http.Request, p *store.Principal) (any, error) {
		in, rid, err := body[guides.StepReq](r)
		if err != nil {
			return nil, err
		}
		in.RunID = r.PathValue("id")
		return a.Guides.Step(r.Context(), p, rid, in)
	})
}
