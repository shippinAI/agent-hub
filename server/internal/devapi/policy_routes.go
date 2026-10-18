package devapi

import (
	"net/http"
	"strconv"

	"agenthub/internal/changepolicy"
	"agenthub/internal/store"
)

type rollbackIn struct {
	Tenant      string `json:"tenant_id"`
	To          int64  `json:"to_version"`
	BaseVersion int64  `json:"base_version"`
	Reason      string `json:"reason"`
}

func (a *API) policyRoutes(reg func(string, h)) {
	reg("GET /api/v1/policy/doc", func(w http.ResponseWriter, r *http.Request, p *store.Principal) (any, error) {
		return a.Policy.Current(r.Context(), r.URL.Query().Get("tenant"))
	})
	reg("GET /api/v1/policy/versions", func(w http.ResponseWriter, r *http.Request, p *store.Principal) (any, error) {
		v, _ := strconv.ParseInt(r.URL.Query().Get("version"), 10, 64)
		return a.Policy.Versions(r.Context(), r.URL.Query().Get("tenant"), v)
	})
	reg("PUT /api/v1/policy/doc", func(w http.ResponseWriter, r *http.Request, p *store.Principal) (any, error) {
		in, rid, err := body[changepolicy.PutIn](r)
		if err != nil {
			return nil, err
		}
		return a.Policy.Put(r.Context(), p, rid, in)
	})
	reg("POST /api/v1/policy/rollback", func(w http.ResponseWriter, r *http.Request, p *store.Principal) (any, error) {
		in, rid, err := body[rollbackIn](r)
		if err != nil {
			return nil, err
		}
		return a.Policy.Rollback(r.Context(), p, rid, in.Tenant, in.To, in.BaseVersion, in.Reason)
	})
	reg("POST /api/v1/policy/decide", func(w http.ResponseWriter, r *http.Request, p *store.Principal) (any, error) {
		in, _, err := body[changepolicy.Ask](r)
		if err != nil {
			return nil, err
		}
		return a.Policy.Decide(r.Context(), p, in)
	})
	reg("GET /api/v1/policy/decisions", func(w http.ResponseWriter, r *http.Request, p *store.Principal) (any, error) {
		n, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		return a.St.Events(r.Context(), store.EventFilter{Action: "policy.decision", Limit: n, Desc: true})
	})
	a.ticketRoutes(reg)
}

func (a *API) ticketRoutes(reg func(string, h)) {
	reg("GET /api/v1/change-tickets", func(w http.ResponseWriter, r *http.Request, p *store.Principal) (any, error) {
		queue := ""
		if r.URL.Query().Get("queue") == "mine" {
			queue = p.ID
		}
		return a.Policy.List(r.Context(), r.URL.Query().Get("status"), queue)
	})
	reg("GET /api/v1/change-tickets/{id}", func(w http.ResponseWriter, r *http.Request, p *store.Principal) (any, error) {
		return a.ticketView(r, p, r.PathValue("id"))
	})
	reg("POST /api/v1/change-tickets", func(w http.ResponseWriter, r *http.Request, p *store.Principal) (any, error) {
		in, rid, err := body[changepolicy.TicketIn](r)
		if err != nil {
			return nil, err
		}
		return a.Policy.Create(r.Context(), p, rid, in)
	})
	reg("PUT /api/v1/change-tickets/{id}", func(w http.ResponseWriter, r *http.Request, p *store.Principal) (any, error) {
		in, rid, err := body[changepolicy.TicketIn](r)
		if err != nil {
			return nil, err
		}
		in.ID = r.PathValue("id")
		return a.Policy.Update(r.Context(), p, rid, in)
	})
	reg("POST /api/v1/change-tickets/{id}/evidence", func(w http.ResponseWriter, r *http.Request, p *store.Principal) (any, error) {
		in, rid, err := body[changepolicy.EvidenceIn](r)
		if err != nil {
			return nil, err
		}
		in.Ticket = r.PathValue("id")
		return a.Policy.AddEvidence(r.Context(), p, rid, in)
	})
	reg("POST /api/v1/change-tickets/{id}/submit", func(w http.ResponseWriter, r *http.Request, p *store.Principal) (any, error) {
		return a.Policy.Submit(r.Context(), p, r.Header.Get("Idempotency-Key"), r.PathValue("id"))
	})
	reg("POST /api/v1/change-tickets/{id}/reviews", func(w http.ResponseWriter, r *http.Request, p *store.Principal) (any, error) {
		in, rid, err := body[changepolicy.ReviewIn](r)
		if err != nil {
			return nil, err
		}
		in.Ticket = r.PathValue("id")
		return a.Policy.Review(r.Context(), p, rid, in)
	})
}

// ticketView is a ticket with evidence, reviews and a dry-run decision for its command (what would happen now).
func (a *API) ticketView(r *http.Request, p *store.Principal, id string) (any, error) {
	d, err := a.Policy.Get(r.Context(), id)
	if err != nil {
		return nil, err
	}
	m := ""
	if d.Ticket.Machine != nil {
		m = *d.Ticket.Machine
	}
	pre, perr := a.Policy.Decide(r.Context(), p, changepolicy.Ask{Tool: "hub:exec", Command: d.Ticket.Command, Machine: m, TicketID: id, DryRun: true})
	out := map[string]any{"ticket": d.Ticket, "evidence": d.Evidence, "reviews": d.Reviews, "preview": pre}
	if perr != nil {
		out["preview_error"] = perr.Error()
	}
	return out, nil
}
