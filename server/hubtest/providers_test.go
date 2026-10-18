package hubtest

import (
	"strings"
	"testing"
)

func errPaths(r resp) []string {
	e, _ := r.Body["error"].(map[string]any)
	cur, _ := e["current"].(map[string]any)
	list, _ := cur["errors"].([]any)
	out := []string{}
	for _, x := range list {
		out = append(out, x.(map[string]any)["path"].(string))
	}
	return out
}

func newRow(id string) map[string]any {
	return map[string]any{"schema_version": 1, "id": id, "vendor": "Example", "product": "Example CLI", "label": "Example",
		"icon_ref": "icons/example", "auth_kinds": []string{"client_login"}, "runner": []string{"user_device"},
		"editions": []string{"local"}, "adapter": map[string]any{"kind": "acp", "config": map[string]any{"argv": []string{"example", "acp"}}},
		"models": map[string]any{"source": "adapter"}, "usage": map[string]any{"tokens": "stream", "quota": "observed", "cost_basis": "client_estimate"},
		"state": map[string]any{"read": "live_only"}, "detect": map[string]any{"linux": map[string]any{"bins": []string{"example"}}},
		"capabilities": map[string]any{"streaming": true, "tools": true, "mcp_client": false, "mcp_host": false, "resume": false,
			"fork": false, "images": false, "reasoning_effort": false, "approvals": true}}
}

func TestProvidersREST(t *testing.T) {
	e := setup(t)
	list := e.callList("claude-code", "/providers")
	if len(list) < 20 {
		t.Fatalf("seeded providers: %d", len(list))
	}
	for _, p := range list {
		if _, bad := p["terms_status"]; bad {
			t.Fatalf("terms_status still served: %v", p["id"])
		}
		if p["schema_version"] != 1.0 || p["adapter"] == nil && p["id"] != "anthropic-claude-desktop" {
			t.Fatalf("row shape: %v", p)
		}
	}
	g := e.call("claude-code", "GET", "/providers/anthropic-api", nil)
	if mc, _ := g.Body["model_cache"].([]any); len(mc) != 2 {
		t.Fatalf("model cache: %v", g.Body)
	}
	if r := e.call("claude-code", "GET", "/providers/nope-nope", nil); r.Status != 404 {
		t.Fatalf("missing provider: %d", r.Status)
	}
	if r := e.call("claude-code", "POST", "/providers", newRow("example-cli")); r.code() != "humans_only" {
		t.Fatalf("agent created a provider: %v", r.Body)
	}
	bad := newRow("Example")
	bad["adapter"] = map[string]any{"kind": "openai_api", "config": map[string]any{}}
	bad["terms_status"] = "verified"
	r := e.call("jesse", "POST", "/providers", bad)
	if r.Status != 422 || r.code() != "validation_failed" {
		t.Fatalf("invalid row: %d %v", r.Status, r.Body)
	}
	if p := strings.Join(errPaths(r), ","); !strings.Contains(p, "terms_status") {
		t.Fatalf("error paths: %s", p)
	}
	delete(bad, "terms_status")
	if p := strings.Join(errPaths(e.call("jesse", "POST", "/providers", bad)), ","); !strings.Contains(p, "id") || !strings.Contains(p, "adapter.kind") {
		t.Fatalf("error paths: %s", p)
	}
	c := e.call("jesse", "POST", "/providers", newRow("example-cli"))
	if c.Status != 200 || c.Body["version"] != 1.0 {
		t.Fatalf("create: %v", c.Body)
	}
	if r := e.call("jesse", "POST", "/providers", newRow("example-cli")); r.code() != "exists" {
		t.Fatalf("duplicate create: %v", r.Body)
	}
	if r := e.call("jesse", "PATCH", "/providers/example-cli", map[string]any{"label": "x", "version": 9}); r.code() != "version_conflict" {
		t.Fatalf("stale patch: %v", r.Body)
	}
	u := e.call("jesse", "PATCH", "/providers/example-cli", map[string]any{"label": "Example 2", "version": 1})
	if u.Body["label"] != "Example 2" || u.Body["version"] != 2.0 || u.Body["vendor"] != "Example" {
		t.Fatalf("patch: %v", u.Body)
	}
	if r := e.call("jesse", "PATCH", "/providers/example-cli", map[string]any{"id": "other-id", "version": 2}); r.code() != "validation_failed" {
		t.Fatalf("id change: %v", r.Body)
	}
	// Router option: litellm needs an existing litellm endpoint.
	route := func(mode, ep string) resp {
		cur := e.call("jesse", "GET", "/providers/anthropic-api", nil).Body
		return e.call("jesse", "PATCH", "/providers/anthropic-api", map[string]any{"version": cur["version"],
			"adapter": map[string]any{"kind": "anthropic_compatible", "config": map[string]any{"router": map[string]any{"mode": mode, "endpoint_id": ep}}}})
	}
	if r := route("litellm", "no-such-router"); !strings.Contains(strings.Join(errPaths(r), ","), "adapter.config.router.endpoint_id") {
		t.Fatalf("missing router endpoint: %v", r.Body)
	}
	if r := route("litellm", "openrouter"); r.code() != "validation_failed" {
		t.Fatalf("router kind mismatch: %v", r.Body)
	}
	if r := route("litellm", "litellm-local"); r.Status != 200 {
		t.Fatalf("litellm route: %v", r.Body)
	}
	if r := e.call("jesse", "DELETE", "/router-endpoints/litellm-local", nil); r.code() != "in_use" {
		t.Fatalf("router in use deleted: %v", r.Body)
	}
	// Disable / enable.
	d := e.call("jesse", "POST", "/providers/example-cli/disable", nil)
	if d.Body["disabled"] != true {
		t.Fatalf("disable: %v", d.Body)
	}
	if l := e.callList("jesse", "/providers?enabled=true"); len(l) != len(list) {
		t.Fatalf("enabled list has %d, want %d", len(l), len(list))
	}
	if r := e.call("jesse", "POST", "/subscriptions", map[string]any{"provider_id": "example-cli", "label": "x", "kind": "cli_login"}); r.code() != "provider_disabled" {
		t.Fatalf("subscription on disabled provider: %v", r.Body)
	}
	if r := e.call("jesse", "POST", "/providers/example-cli/enable", nil); r.Body["disabled"] != false {
		t.Fatalf("enable: %v", r.Body)
	}
	v := e.call("jesse", "POST", "/providers/validate", newRow("ok-row"))
	if v.Body["ok"] != true {
		t.Fatalf("validate: %v", v.Body)
	}
	if s := e.call("claude-code", "GET", "/provider-schema", nil); len(s.Body["adapter_kinds"].([]any)) != 5 {
		t.Fatalf("schema: %v", s.Body)
	}
}
