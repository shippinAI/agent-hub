package devapi

import (
	"net/http"
	"strconv"
	"time"

	"agenthub/internal/actionfeed"
	"agenthub/internal/auditkeep"
	"agenthub/internal/store"
)

func (a *API) auditRoutes(reg func(string, h)) {
	reg("GET /api/v1/actions", func(w http.ResponseWriter, r *http.Request, p *store.Principal) (any, error) {
		if err := humanOnly(p); err != nil {
			return nil, err
		}
		q := r.URL.Query()
		f := actionfeed.Filter{AfterID: actionfeed.ParseID(q.Get("after_id")), BeforeID: actionfeed.ParseID(q.Get("before_id")),
			Category: q.Get("category"), CallID: q.Get("call_id"), Actor: q.Get("actor"), Machine: q.Get("machine"), Order: q.Get("order")}
		f.Limit, _ = strconv.Atoi(q.Get("limit"))
		if wait, _ := strconv.Atoi(q.Get("wait")); wait > 0 && f.Order != "desc" {
			a.Actions.Wait(r.Context(), f.AfterID, time.Duration(min(wait, 60))*time.Second)
		}
		evs, err := a.Feed.List(r.Context(), f)
		return map[string]any{"actions": evs, "head": a.Feed.Head(r.Context())}, err
	})
	reg("GET /api/v1/actions/a2ui", func(w http.ResponseWriter, r *http.Request, p *store.Principal) (any, error) {
		if err := humanOnly(p); err != nil {
			return nil, err
		}
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		evs, err := a.Feed.List(r.Context(), actionfeed.Filter{Order: "desc", Limit: limit})
		return map[string]any{"mime": "application/a2ui+json", "messages": actionfeed.A2UI(evs)}, err
	})
	reg("GET /api/v1/audit/retention", func(w http.ResponseWriter, r *http.Request, p *store.Principal) (any, error) {
		if err := humanOnly(p); err != nil {
			return nil, err
		}
		cats, err := a.Keep.Categories(r.Context())
		if err != nil {
			return nil, err
		}
		pts, err := a.Keep.Partitions(r.Context(), "")
		if err != nil {
			return nil, err
		}
		set, _ := a.St.LoadSettings(r.Context())
		var arch []map[string]any
		rows, err := a.St.DB.Query(r.Context(), `SELECT category, count(*), coalesce(sum(rows),0), coalesce(sum(bytes),0), max(created_at)
			FROM audit_archives GROUP BY category ORDER BY category`)
		if err == nil {
			for rows.Next() {
				var c string
				var n, rs, b int64
				var at *time.Time
				_ = rows.Scan(&c, &n, &rs, &b, &at)
				arch = append(arch, map[string]any{"category": c, "segments": n, "rows": rs, "bytes": b, "last": at})
			}
			rows.Close()
		}
		return map[string]any{"categories": cats, "partitions": pts, "archives": arch,
			"settings": map[string]any{"audit_archive_dir": set.String("audit_archive_dir", ""), "audit_total_max_bytes": set.Int("audit_total_max_bytes", 1<<30),
				"audit_partition_ahead_days": set.Int("audit_partition_ahead_days", 2), "audit_prune_interval_seconds": set.Int("audit_prune_interval_seconds", 300),
				"audit_on_archive_failure": set.String("audit_on_archive_failure", "drop"), "signing": a.Keep.Signer != nil}}, nil
	})
	reg("PUT /api/v1/audit/categories/{category}", func(w http.ResponseWriter, r *http.Request, p *store.Principal) (any, error) {
		in, rid, err := body[auditkeep.Category](r)
		if err != nil {
			return nil, err
		}
		in.Category = r.PathValue("category")
		return a.Keep.Put(r.Context(), p, rid, in)
	})
	run := func(name string, fn func(r *http.Request) (any, error)) h {
		return func(w http.ResponseWriter, r *http.Request, p *store.Principal) (any, error) {
			if _, err := a.St.Write(r.Context(), p, "", "audit_"+name, nil, func(w *store.W) (any, error) {
				if err := w.Admin("audit"); err != nil {
					return nil, err
				}
				return nil, w.Event("audit."+name+"_requested", nil, nil, map[string]any{"category": r.URL.Query().Get("category")})
			}); err != nil {
				return nil, err
			}
			return fn(r)
		}
	}
	reg("POST /api/v1/audit/prune", run("prune", func(r *http.Request) (any, error) { return a.Keep.Prune(r.Context()) }))
	reg("POST /api/v1/audit/flush", run("flush", func(r *http.Request) (any, error) {
		segs, err := a.Keep.Flush(r.Context(), r.URL.Query().Get("category"))
		return map[string]any{"archived": segs}, err
	}))
}
