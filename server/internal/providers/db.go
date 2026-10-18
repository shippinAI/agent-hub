package providers

import (
	"encoding/json"
	"fmt"
	"strings"

	"agenthub/internal/store"
)

// cols are the providers columns a row writes, in args() order (migration 007).
var cols = []string{"schema_version", "vendor", "product", "label", "icon_ref", "docs_url", "auth_kinds", "runner",
	"editions", "adapter", "fallback_adapters", "host_for", "models", "usage", "state", "detect", "capabilities",
	"terms_url", "terms_note"}

// readOnly are response fields a client may echo back on save; they are ignored, never written.
var readOnly = []string{"version", "tenant_id", "disabled_at", "disabled", "model_cache"}

const selectRow = `SELECT (to_jsonb(t) - 'tenant_id') || jsonb_build_object('disabled', t.disabled_at IS NOT NULL) FROM providers t`

func args(p Provider) []any {
	js := func(v any) []byte { b, _ := json.Marshal(v); return b }
	var adapter any
	if p.Adapter != nil {
		adapter = js(p.Adapter)
	}
	fb := p.FallbackAdapters
	if fb == nil {
		fb = []Adapter{}
	}
	det := p.Detect
	if det == nil {
		det = Detect{}
	}
	return []any{p.SchemaVersion, p.Vendor, p.Product, p.Label, p.IconRef, p.DocsURL, p.AuthKinds, p.Runner,
		p.Editions, adapter, js(fb), p.HostFor, js(p.Models), js(p.Usage), js(p.State), js(det), js(p.Capabilities),
		p.TermsURL, p.TermsNote}
}

func insert(w *store.W, p Provider, onConflictSkip bool) (int64, error) {
	ph := make([]string, len(cols)+1)
	for i := range ph {
		ph[i] = fmt.Sprintf("$%d", i+1)
	}
	sql := `INSERT INTO providers(id,` + strings.Join(cols, ",") + `) VALUES(` + strings.Join(ph, ",") + `)`
	if onConflictSkip {
		sql += ` ON CONFLICT DO NOTHING`
	}
	tag, err := w.Exec(sql, append([]any{p.ID}, args(p)...)...)
	return tag.RowsAffected(), err
}

func update(w *store.W, p Provider) error {
	sets := []string{"version=version+1"}
	for i, c := range cols {
		sets = append(sets, fmt.Sprintf("%s=$%d", c, i+2))
	}
	_, err := w.Exec(`UPDATE providers SET `+strings.Join(sets, ",")+` WHERE id=$1`, append([]any{p.ID}, args(p)...)...)
	return err
}

// routers checks DB facts the pure validator cannot: each API adapter's router endpoint exists and its kind
// matches the router mode.
func routers(w *store.W, p Provider) Errs {
	var errs Errs
	all := append([]Adapter{}, p.FallbackAdapters...)
	paths := make([]string, 0, len(all)+1)
	for i := range all {
		paths = append(paths, fmt.Sprintf("fallback_adapters[%d]", i))
	}
	if p.Adapter != nil {
		all, paths = append([]Adapter{*p.Adapter}, all...), append([]string{"adapter"}, paths...)
	}
	for i, a := range all {
		if !IsAPI(a.Kind) {
			continue
		}
		var c APIConfig
		if json.Unmarshal(a.Config, &c) != nil || c.Router == nil || c.Router.Mode == "direct" {
			continue
		}
		var kind string
		path := paths[i] + ".config.router.endpoint_id"
		if err := w.Row(`SELECT kind FROM router_endpoints WHERE id=$1`, c.Router.EndpointID).Scan(&kind); err != nil {
			errs.add(path, "no router endpoint %q (add it under Router endpoints first)", c.Router.EndpointID)
		} else if kind != c.Router.Mode {
			errs.add(path, "endpoint %q is a %s router, not %s", c.Router.EndpointID, kind, c.Router.Mode)
		}
	}
	return errs
}

// Invalid turns field errors into the shared 422 envelope: {code, message, current: {errors: [...]}}.
func Invalid(errs Errs) error {
	return &store.Error{Code: "validation_failed", Status: 422, Current: map[string]any{"errors": errs},
		Message: fmt.Sprintf("%d problem(s); first: %s: %s", len(errs), errs[0].Path, errs[0].Message)}
}

// strip removes read-only response fields and returns the version the client sent (0 = create).
func strip(body map[string]json.RawMessage) int64 {
	var v int64
	_ = json.Unmarshal(body["version"], &v)
	for _, k := range readOnly {
		delete(body, k)
	}
	return v
}
