package hubtest

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSubscriptionsSecretsRouting(t *testing.T) {
	e := setup(t)
	const key = "sk-test-0123456789abcdefSECRET"
	var sawKey bool
	router := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawKey = r.Header.Get("Authorization") == "Bearer "+key
		_, _ = io.ReadAll(r.Body)
		w.Write([]byte(`{"choices":[{"message":{"content":"p"}}]}`))
	}))
	defer router.Close()
	rt := e.call("jesse", "GET", "/router-endpoints", nil)
	ver := 0.0
	for _, x := range rt.Body["list"].([]any) {
		if m := x.(map[string]any); m["id"] == "litellm-local" {
			ver = m["version"].(float64)
		}
	}
	if r := e.call("jesse", "PUT", "/router-endpoints/litellm-local", map[string]any{"label": "local", "url": router.URL, "kind": "litellm", "version": ver}); r.Status != 200 {
		t.Fatalf("router url: %v", r.Body)
	}
	if r := e.call("claude-code", "POST", "/subscriptions", map[string]any{"provider_id": "anthropic-api", "label": "x", "kind": "api_key"}); r.code() != "humans_only" {
		t.Fatalf("agent created a subscription: %v", r.Body)
	}
	r := e.call("jesse", "POST", "/subscriptions", map[string]any{"provider_id": "anthropic-api", "label": "Anthropic via router", "kind": "api_key"})
	sub := r.Body["id"].(string)
	s := e.call("jesse", "PUT", "/subscriptions/"+sub+"/secret", map[string]any{"value": key})
	if len(s.Body) != 1 || s.Body["fp"] == nil {
		t.Fatalf("secret response must be {fp} only: %v", s.Body)
	}
	tr := e.call("jesse", "POST", "/subscriptions/"+sub+"/test", nil)
	if tr.Body["ok"] != true || !sawKey {
		t.Fatalf("router test: %v sawKey=%v", tr.Body, sawKey)
	}
	for _, path := range []string{"/subscriptions", "/subscriptions/" + sub + "/test", "/events?limit=500"} {
		raw := e.call("jesse", "GET", path, nil)
		if strings.Contains(strFmt(raw.Body), "SECRET") {
			t.Fatalf("%s leaked the secret", path)
		}
	}
	var leaked int
	_ = e.pool.QueryRow(t.Context(), `SELECT count(*) FROM events WHERE data::text LIKE '%SECRET%'`).Scan(&leaked)
	if leaked != 0 {
		t.Fatal("secret in the event log")
	}
	if r := e.call("jesse", "POST", "/subscriptions", map[string]any{"provider_id": "claude-subscription", "label": "Max", "kind": "cli_login"}); r.code() != "cli_login_rule" {
		t.Fatalf("cli_login without machine/worker: %v", r.Body)
	}
	b := e.call("jesse", "POST", "/role-bindings", map[string]any{"role_id": "builder", "chat_app": "claude", "subscription_id": sub, "priority": 1})
	if b.Status != 200 {
		t.Fatalf("binding: %v", b.Body)
	}
	route := e.call("claude-code", "GET", "/role-route?role=builder", nil)
	if route.Body["subscription_id"] != sub {
		t.Fatalf("route: %v", route.Body)
	}
}

func strFmt(m map[string]any) string {
	var sb strings.Builder
	for k, v := range m {
		sb.WriteString(k)
		sb.WriteString("=")
		sb.WriteString(strings.TrimSpace(strings.ReplaceAll(fmtAny(v), "\n", " ")))
	}
	return sb.String()
}
