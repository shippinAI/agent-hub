package httpapi

import (
	"encoding/json"
	"net/http"

	"agenthub/internal/policy"
	"agenthub/internal/store"
)

func (s *Server) adminRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/config", s.authed(func(w http.ResponseWriter, r *http.Request, p *store.Principal) (any, error) {
		ctx := r.Context()
		wfs, err := s.St.Workflows(ctx)
		if err != nil {
			return nil, err
		}
		prs, _ := s.St.Projects(ctx)
		roles, _ := s.St.Roles(ctx)
		people, _ := s.St.Principals(ctx)
		sets, _ := s.St.SettingsList(ctx)
		pols, err := s.St.Policies(ctx)
		return map[string]any{"workflows": wfs, "projects": prs, "roles": roles, "principals": people, "settings": sets,
			"policies": pols, "state_kinds": store.StateKinds}, err
	}))
	mux.HandleFunc("GET /api/v1/principals", s.authed(func(w http.ResponseWriter, r *http.Request, p *store.Principal) (any, error) {
		return s.St.Principals(r.Context())
	}))
	put := func(w http.ResponseWriter, r *http.Request, p *store.Principal) (any, error) {
		var in store.Principal
		rid, err := decode(r, &in)
		if err != nil {
			return nil, err
		}
		if id := r.PathValue("id"); id != "" {
			in.ID = id
		}
		return s.St.PutPrincipal(r.Context(), p, rid, in)
	}
	mux.HandleFunc("POST /api/v1/principals", s.authed(put))
	mux.HandleFunc("PUT /api/v1/principals/{id}", s.authed(put))
	mux.HandleFunc("GET /api/v1/principals/{id}/tokens", s.authed(func(w http.ResponseWriter, r *http.Request, p *store.Principal) (any, error) {
		if !p.IsHuman && p.ID != r.PathValue("id") {
			return nil, &store.Error{Code: "humans_only", Status: 403, Message: "agents may only list their own tokens"}
		}
		return s.St.Tokens(r.Context(), r.PathValue("id"))
	}))
	mux.HandleFunc("POST /api/v1/principals/{id}/tokens", s.authed(func(w http.ResponseWriter, r *http.Request, p *store.Principal) (any, error) {
		var in struct {
			Name        string `json:"name"`
			ExpiresDays int    `json:"expires_days"`
		}
		if _, err := decode(r, &in); err != nil {
			return nil, err
		}
		return s.St.CreateToken(r.Context(), p, r.PathValue("id"), in.Name, in.ExpiresDays)
	}))
	mux.HandleFunc("DELETE /api/v1/tokens/{id}", s.authed(func(w http.ResponseWriter, r *http.Request, p *store.Principal) (any, error) {
		return s.St.RevokeToken(r.Context(), p, r.Header.Get("Idempotency-Key"), r.PathValue("id"))
	}))
	s.configRoutes(mux)
}

func (s *Server) configRoutes(mux *http.ServeMux) {
	putters := map[string]func(r *http.Request, p *store.Principal, rid string, raw json.RawMessage) (any, error){
		"roles": func(r *http.Request, p *store.Principal, rid string, raw json.RawMessage) (any, error) {
			var v store.Role
			if err := unmarshal(raw, &v); err != nil {
				return nil, err
			}
			v.ID = firstSet(r.PathValue("id"), v.ID)
			return s.St.PutRole(r.Context(), p, rid, v)
		},
		"policies": func(r *http.Request, p *store.Principal, rid string, raw json.RawMessage) (any, error) {
			var v policy.Rule
			if err := unmarshal(raw, &v); err != nil {
				return nil, err
			}
			v.ID = firstSet(r.PathValue("id"), v.ID)
			return s.St.PutPolicy(r.Context(), p, rid, v)
		},
		"workflows": func(r *http.Request, p *store.Principal, rid string, raw json.RawMessage) (any, error) {
			var v store.Workflow
			if err := unmarshal(raw, &v); err != nil {
				return nil, err
			}
			v.ID = firstSet(r.PathValue("id"), v.ID)
			return s.St.PutWorkflow(r.Context(), p, rid, v)
		},
		"projects": func(r *http.Request, p *store.Principal, rid string, raw json.RawMessage) (any, error) {
			var v store.Project
			if err := unmarshal(raw, &v); err != nil {
				return nil, err
			}
			v.ID = firstSet(r.PathValue("id"), v.ID)
			return s.St.PutProject(r.Context(), p, rid, v)
		},
		"settings": func(r *http.Request, p *store.Principal, rid string, raw json.RawMessage) (any, error) {
			var v store.SettingRow
			if err := unmarshal(raw, &v); err != nil {
				return nil, err
			}
			v.Key = firstSet(r.PathValue("id"), v.Key)
			return s.St.PutSetting(r.Context(), p, rid, v)
		},
	}
	for kind, fn := range putters {
		h := s.authed(func(w http.ResponseWriter, r *http.Request, p *store.Principal) (any, error) {
			var raw json.RawMessage
			rid, err := decode(r, &raw)
			if err != nil {
				return nil, err
			}
			return fn(r, p, rid, raw)
		})
		mux.HandleFunc("PUT /api/v1/"+kind+"/{id}", h)
		mux.HandleFunc("POST /api/v1/"+kind, h)
		if kind != "settings" {
			k := kind
			mux.HandleFunc("DELETE /api/v1/"+kind+"/{id}", s.authed(func(w http.ResponseWriter, r *http.Request, p *store.Principal) (any, error) {
				return s.St.DeleteConfig(r.Context(), p, r.Header.Get("Idempotency-Key"), k, r.PathValue("id"))
			}))
		}
	}
}

func unmarshal(raw json.RawMessage, v any) error {
	if err := json.Unmarshal(raw, v); err != nil {
		return &store.Error{Code: "invalid", Status: 400, Message: "bad JSON: " + err.Error()}
	}
	return nil
}

func firstSet(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
