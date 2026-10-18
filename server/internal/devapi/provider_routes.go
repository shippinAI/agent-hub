package devapi

import (
	"encoding/json"
	"net/http"

	"agenthub/internal/providers"
	"agenthub/internal/store"
)

// providerRoutes: list/get/create/update/disable provider rows (doc 34). Validation failures are 422
// validation_failed with current.errors = [{path, message}].
func (a *API) providerRoutes(reg func(string, h)) {
	reg("GET /api/v1/providers", func(w http.ResponseWriter, r *http.Request, p *store.Principal) (any, error) {
		return a.Providers.List(r.Context(), r.URL.Query().Get("enabled") == "true")
	})
	reg("GET /api/v1/providers/{id}", func(w http.ResponseWriter, r *http.Request, p *store.Principal) (any, error) {
		return a.Providers.Get(r.Context(), r.PathValue("id"))
	})
	reg("GET /api/v1/provider-schema", func(w http.ResponseWriter, r *http.Request, p *store.Principal) (any, error) {
		return providerSchema(), nil
	})
	reg("POST /api/v1/providers/validate", func(w http.ResponseWriter, r *http.Request, p *store.Principal) (any, error) {
		in, _, err := body[map[string]json.RawMessage](r)
		if err != nil {
			return nil, err
		}
		return a.Providers.Check(r.Context(), in)
	})
	save := func(merge bool) h {
		return func(w http.ResponseWriter, r *http.Request, p *store.Principal) (any, error) {
			in, rid, err := body[map[string]json.RawMessage](r)
			if err != nil {
				return nil, err
			}
			return a.Providers.Save(r.Context(), p, rid, r.PathValue("id"), in, merge)
		}
	}
	reg("POST /api/v1/providers", func(w http.ResponseWriter, r *http.Request, p *store.Principal) (any, error) {
		in, rid, err := body[map[string]json.RawMessage](r)
		if err != nil {
			return nil, err
		}
		delete(in, "version") // POST always creates
		return a.Providers.Save(r.Context(), p, rid, "", in, false)
	})
	reg("PUT /api/v1/providers/{id}", save(false))
	reg("PATCH /api/v1/providers/{id}", save(true))
	for path, off := range map[string]bool{"disable": true, "enable": false} {
		disabled := off
		reg("POST /api/v1/providers/{id}/"+path, func(w http.ResponseWriter, r *http.Request, p *store.Principal) (any, error) {
			return a.Providers.SetDisabled(r.Context(), p, r.Header.Get("Idempotency-Key"), r.PathValue("id"), disabled)
		})
	}
	reg("PUT /api/v1/providers/{id}/models", func(w http.ResponseWriter, r *http.Request, p *store.Principal) (any, error) {
		in, rid, err := body[struct {
			Models []providers.ModelIn `json:"models"`
		}](r)
		if err != nil {
			return nil, err
		}
		return a.Providers.PutModels(r.Context(), p, rid, r.PathValue("id"), in.Models)
	})
}

// providerSchema is the vocabulary clients build forms from (the same lists the validator enforces).
func providerSchema() map[string]any {
	return map[string]any{"schema_version": providers.SchemaVersion, "adapter_kinds": providers.AdapterKinds,
		"api_adapters": providers.APIAdapters, "router_modes": providers.RouterModes, "auth_kinds": providers.AuthKinds,
		"runners": providers.Runners, "editions": providers.Editions, "oses": providers.OSes,
		"model_sources": providers.ModelSources, "token_sources": providers.TokenSources,
		"quota_sources": providers.QuotaSources, "cost_bases": providers.CostBases, "state_reads": providers.StateReads,
		"defaults": map[string]int{"refresh_minutes": providers.DefaultRefreshMinutes, "turn_timeout_ms": providers.DefaultTurnTimeoutMS,
			"detect_timeout_ms": providers.DefaultDetectTimeout, "max_retries": providers.DefaultMaxRetries, "backoff_ms": providers.DefaultBackoffMS}}
}
