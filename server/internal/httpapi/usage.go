package httpapi

import (
	"net/http"
	"strconv"
	"time"

	"agenthub/internal/store"
)

func (s *Server) usageRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/usage", s.authed(func(w http.ResponseWriter, r *http.Request, p *store.Principal) (any, error) {
		var in store.UsageInput
		rid, err := decode(r, &in)
		if err != nil {
			return nil, err
		}
		return s.St.RecordUsage(r.Context(), p, rid, in)
	}))
	mux.HandleFunc("GET /api/v1/usage", s.authed(func(w http.ResponseWriter, r *http.Request, p *store.Principal) (any, error) {
		q := r.URL.Query()
		f := store.UsageFilter{GroupBy: q.Get("group_by"), Principal: q.Get("principal")}
		if f.Principal == "me" {
			f.Principal = p.ID
		}
		f.TaskID, _ = strconv.ParseInt(q.Get("task"), 10, 64)
		for k, dst := range map[string]**time.Time{"from": &f.From, "to": &f.To} {
			if v := q.Get(k); v != "" {
				t, err := time.Parse(time.RFC3339, v)
				if err != nil {
					return nil, &store.Error{Code: "invalid", Status: 400, Message: k + " must be RFC 3339"}
				}
				*dst = &t
			}
		}
		totals, err := s.St.Usage(r.Context(), f)
		return map[string]any{"group_by": f.GroupBy, "totals": totals, "units": "cost in micro-USD"}, err
	}))
	mux.HandleFunc("GET /api/v1/budgets", s.authed(func(w http.ResponseWriter, r *http.Request, p *store.Principal) (any, error) {
		b, err := s.St.Budgets(r.Context())
		return map[string]any{"budgets": b}, err
	}))
	mux.HandleFunc("GET /api/v1/prices", s.authed(func(w http.ResponseWriter, r *http.Request, p *store.Principal) (any, error) {
		pr, err := s.St.Prices(r.Context())
		return map[string]any{"prices": pr}, err
	}))
}
