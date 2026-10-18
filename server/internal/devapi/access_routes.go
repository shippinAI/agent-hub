package devapi

import (
	"net/http"

	"agenthub/internal/access"
	"agenthub/internal/store"
)

func (a *API) accessRoutes(reg func(string, h)) {
	q := func(r *http.Request, k string) string { return r.URL.Query().Get(k) }
	reg("GET /api/v1/access-requests", func(w http.ResponseWriter, r *http.Request, p *store.Principal) (any, error) {
		return a.Access.Requests(r.Context(), q(r, "status"), q(r, "machine"))
	})
	reg("POST /api/v1/access-requests", func(w http.ResponseWriter, r *http.Request, p *store.Principal) (any, error) {
		in, rid, err := body[access.Req](r)
		if err != nil {
			return nil, err
		}
		return a.Access.Request(r.Context(), p, rid, in)
	})
	reg("POST /api/v1/access-requests/{id}/approve", func(w http.ResponseWriter, r *http.Request, p *store.Principal) (any, error) {
		in, rid, err := body[access.Narrow](r)
		if err != nil {
			return nil, err
		}
		return a.Access.Approve(r.Context(), p, rid, r.PathValue("id"), in)
	})
	reg("POST /api/v1/access-requests/{id}/deny", func(w http.ResponseWriter, r *http.Request, p *store.Principal) (any, error) {
		in, rid, err := body[struct {
			Note string `json:"note"`
		}](r)
		if err != nil {
			return nil, err
		}
		return a.Access.Deny(r.Context(), p, rid, r.PathValue("id"), in.Note)
	})
	reg("GET /api/v1/grants", func(w http.ResponseWriter, r *http.Request, p *store.Principal) (any, error) {
		return a.Access.Grants(r.Context(), q(r, "live") == "true", q(r, "machine"), q(r, "principal"))
	})
	reg("POST /api/v1/grants/{id}/revoke", func(w http.ResponseWriter, r *http.Request, p *store.Principal) (any, error) {
		in, rid, err := body[struct {
			Reason string `json:"reason"`
		}](r)
		if err != nil {
			return nil, err
		}
		return a.Access.Revoke(r.Context(), p, rid, r.PathValue("id"), in.Reason)
	})
	reg("GET /api/v1/sandbox-policies", func(w http.ResponseWriter, r *http.Request, p *store.Principal) (any, error) {
		return a.Access.Policies(r.Context())
	})
	reg("PUT /api/v1/sandbox-policies", func(w http.ResponseWriter, r *http.Request, p *store.Principal) (any, error) {
		in, rid, err := body[access.PolicyReq](r)
		if err != nil {
			return nil, err
		}
		return a.Access.SetPolicy(r.Context(), p, rid, in)
	})
	reg("PUT /api/v1/settings/grant_default_preset", func(w http.ResponseWriter, r *http.Request, p *store.Principal) (any, error) {
		in, rid, err := body[struct {
			Preset  string `json:"preset"`
			Confirm string `json:"confirm"`
		}](r)
		if err != nil {
			return nil, err
		}
		return a.Access.SetDefaultPreset(r.Context(), p, rid, in.Preset, in.Confirm)
	})
	reg("POST /api/v1/resume-alerts/check", func(w http.ResponseWriter, r *http.Request, p *store.Principal) (any, error) {
		in, rid, err := body[struct {
			Session string `json:"ui_session_id"`
			Reason  string `json:"reason"`
		}](r)
		if err != nil {
			return nil, err
		}
		return a.Access.Check(r.Context(), p, rid, in.Session, firstOr(in.Reason, "page_load"))
	})
	reg("GET /api/v1/resume-alerts", func(w http.ResponseWriter, r *http.Request, p *store.Principal) (any, error) {
		if err := humanOnly(p); err != nil {
			return nil, err
		}
		return a.Access.Open(r.Context(), p.ID)
	})
	reg("POST /api/v1/resume-alerts/{id}/{action}", func(w http.ResponseWriter, r *http.Request, p *store.Principal) (any, error) {
		return a.Access.Ack(r.Context(), p, r.Header.Get("Idempotency-Key"), r.PathValue("id"), r.PathValue("action"))
	})
	reg("GET /api/v1/access-summary", func(w http.ResponseWriter, r *http.Request, p *store.Principal) (any, error) {
		return a.summary(r)
	})
}

func (a *API) summary(r *http.Request) (any, error) {
	var live, high, gui, pending int
	var unsandboxed bool
	err := a.St.DB.QueryRow(r.Context(), `SELECT
		(SELECT count(*) FROM grants WHERE ended_at IS NULL),
		(SELECT count(*) FROM grants WHERE ended_at IS NULL AND (sandbox_level='unsandboxed' OR full_access OR risk_snapshot @> '[{"severity":"high"}]')),
		(SELECT count(*) FROM gui_sessions WHERE state<>'ended'),
		(SELECT count(*) FROM access_requests WHERE status='pending' AND expires_at>now()),
		EXISTS(SELECT 1 FROM sandbox_policies WHERE unsandboxed_allowed AND (expires_at IS NULL OR expires_at>now()))`).Scan(&live, &high, &gui, &pending, &unsandboxed)
	return map[string]any{"live_grants": live, "high_risk": high, "gui_active": gui, "pending_requests": pending, "unsandboxed_allowed": unsandboxed}, err
}

func firstOr(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
