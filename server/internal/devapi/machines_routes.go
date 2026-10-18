package devapi

import (
	"net/http"

	"agenthub/internal/machines"
	"agenthub/internal/store"
)

func (a *API) machineRoutes(reg func(string, h)) {
	reg("GET /api/v1/machines", func(w http.ResponseWriter, r *http.Request, p *store.Principal) (any, error) {
		return a.Machines.List(r.Context())
	})
	reg("GET /api/v1/machines/{id}", func(w http.ResponseWriter, r *http.Request, p *store.Principal) (any, error) {
		return a.Machines.Get(r.Context(), r.PathValue("id"))
	})
	reg("POST /api/v1/machines", func(w http.ResponseWriter, r *http.Request, p *store.Principal) (any, error) {
		in, rid, err := body[machines.CreateIn](r)
		if err != nil {
			return nil, err
		}
		return a.Machines.Create(r.Context(), p, rid, in)
	})
	reg("PATCH /api/v1/machines/{id}", func(w http.ResponseWriter, r *http.Request, p *store.Principal) (any, error) {
		in, rid, err := body[struct {
			Label    string   `json:"label"`
			Profiles []string `json:"shell_profile_ids"`
			Version  int64    `json:"version"`
		}](r)
		if err != nil {
			return nil, err
		}
		return a.Machines.Patch(r.Context(), p, rid, r.PathValue("id"), in.Label, in.Profiles, in.Version)
	})
	reg("POST /api/v1/machines/{id}/enroll-code", func(w http.ResponseWriter, r *http.Request, p *store.Principal) (any, error) {
		return a.Machines.NewCode(r.Context(), p, r.Header.Get("Idempotency-Key"), r.PathValue("id"))
	})
	reg("POST /api/v1/machines/{id}/test", func(w http.ResponseWriter, r *http.Request, p *store.Principal) (any, error) {
		if err := humanOnly(p); err != nil {
			return nil, err
		}
		return a.Ops.Probe(r.Context(), r.PathValue("id"))
	})
	for _, act := range []string{"pause", "resume"} {
		paused := act == "pause"
		reg("POST /api/v1/machines/{id}/"+act, func(w http.ResponseWriter, r *http.Request, p *store.Principal) (any, error) {
			return a.Machines.SetPaused(r.Context(), p, r.Header.Get("Idempotency-Key"), r.PathValue("id"), paused)
		})
	}
	reg("POST /api/v1/machines/{id}/revoke-all", func(w http.ResponseWriter, r *http.Request, p *store.Principal) (any, error) {
		return a.Access.RevokeAll(r.Context(), p, r.Header.Get("Idempotency-Key"), r.PathValue("id"), "revoked")
	})
	reg("DELETE /api/v1/machines/{id}", func(w http.ResponseWriter, r *http.Request, p *store.Principal) (any, error) {
		id := r.PathValue("id")
		if _, err := a.Access.RevokeAll(r.Context(), p, "", id, "machine_removed"); err != nil {
			return nil, err
		}
		return a.Machines.Remove(r.Context(), p, r.Header.Get("Idempotency-Key"), id)
	})
	reg("GET /api/v1/gui-sessions", func(w http.ResponseWriter, r *http.Request, p *store.Principal) (any, error) {
		return rows(r, a.St, `SELECT to_jsonb(s) - 'monitors' FROM gui_sessions s WHERE (NOT $1 OR state<>'ended') ORDER BY started_at DESC LIMIT 100`, r.URL.Query().Get("live") == "true")
	})
	reg("POST /api/v1/gui-sessions/{id}/end", func(w http.ResponseWriter, r *http.Request, p *store.Principal) (any, error) {
		return a.Ops.GUIEnd(r.Context(), p, r.PathValue("id"), "operator")
	})
	reg("GET /api/v1/proc-leases", func(w http.ResponseWriter, r *http.Request, p *store.Principal) (any, error) {
		return rows(r, a.St, `SELECT to_jsonb(l) FROM proc_leases l ORDER BY started_at DESC LIMIT $1`, 100)
	})
	reg("POST /api/v1/proc-leases/{id}/kill", func(w http.ResponseWriter, r *http.Request, p *store.Principal) (any, error) {
		if err := humanOnly(p); err != nil {
			return nil, err
		}
		return a.Ops.ProcOp(r.Context(), p, r.PathValue("id"), 0, "proc.kill", map[string]any{})
	})
}

func rows(r *http.Request, st *store.Store, sql string, args ...any) ([]map[string]any, error) {
	rs, err := st.DB.Query(r.Context(), sql, args...)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	out := []map[string]any{}
	for rs.Next() {
		var m map[string]any
		if err := rs.Scan(&m); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rs.Err()
}
