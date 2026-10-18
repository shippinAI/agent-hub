package providers

import "testing"

func apiRow(t *testing.T, kind string, cfg map[string]any) []byte {
	t.Helper()
	b := with(t, "adapter", map[string]any{"kind": kind, "config": cfg})
	b = []byte(string(with2(t, b, "runner", []string{"hub"})))
	return with2(t, b, "fallback_adapters", []any{})
}

func TestAdapterKindsAreExactlyFive(t *testing.T) {
	want := []string{"acp", "codex_app_server", "headless_json", "openai_compatible", "anthropic_compatible"}
	if len(AdapterKinds) != 5 {
		t.Fatalf("adapter kinds: %v", AdapterKinds)
	}
	for i, k := range want {
		if AdapterKinds[i] != k {
			t.Fatalf("adapter kinds: %v", AdapterKinds)
		}
	}
}

func TestRouterOption(t *testing.T) {
	ok := []map[string]any{
		{"base_url": "https://api.example.com/v1"}, // router defaults to direct
		{"base_url": "https://api.example.com/v1", "router": map[string]any{"mode": "direct"}},
		{"router": map[string]any{"mode": "litellm", "endpoint_id": "litellm-local"}},
		{"router": map[string]any{"mode": "openrouter", "endpoint_id": "openrouter"}},
		{"router": map[string]any{"mode": "custom", "endpoint_id": "my-proxy"}, "api": "responses"},
	}
	for i, c := range ok {
		if _, errs := Parse(apiRow(t, "openai_compatible", c)); len(errs) > 0 {
			t.Errorf("ok case %d: %v", i, errs)
		}
	}
	bad := []struct {
		cfg  map[string]any
		path string
	}{
		{map[string]any{}, "adapter.config.base_url"},
		{map[string]any{"router": map[string]any{"mode": "litellm"}}, "adapter.config.router.endpoint_id"},
		{map[string]any{"base_url": "https://x.io", "router": map[string]any{"mode": "direct", "endpoint_id": "x"}}, "adapter.config.router.endpoint_id"},
		{map[string]any{"base_url": "https://x.io", "router": map[string]any{"mode": "bedrock"}}, "adapter.config.router.mode"},
		{map[string]any{"base_url": "https://x.io", "secret_ref": "sk-live-abc"}, "adapter.config.secret_ref"},
		{map[string]any{"base_url": "https://x.io", "api": "completions"}, "adapter.config.api"},
	}
	for i, c := range bad {
		if _, errs := Parse(apiRow(t, "openai_compatible", c.cfg)); !hasErr(errs, c.path) {
			t.Errorf("bad case %d: want %s, got %v", i, c.path, errs)
		}
	}
	if _, errs := Parse(apiRow(t, "anthropic_compatible", map[string]any{"base_url": "https://x.io", "api": "responses"})); !hasErr(errs, "adapter.config.api") {
		t.Errorf("anthropic api field accepted: %v", errs)
	}
}

func TestResolveTargets(t *testing.T) {
	look := func(id string) string { return map[string]string{"litellm-local": "http://127.0.0.1:4000/"}[id] }
	cases := []struct {
		kind, cfg, url, shape string
	}{
		{"anthropic_compatible", `{"base_url":"https://api.anthropic.com"}`, "https://api.anthropic.com/v1/messages", "messages"},
		{"openai_compatible", `{"base_url":"https://api.x.ai/v1"}`, "https://api.x.ai/v1/chat/completions", "chat_completions"},
		{"openai_compatible", `{"base_url":"https://api.openai.com/v1","api":"responses"}`, "https://api.openai.com/v1/responses", "responses"},
		{"anthropic_compatible", `{"router":{"mode":"litellm","endpoint_id":"litellm-local"}}`, "http://127.0.0.1:4000/chat/completions", "chat_completions"},
	}
	for _, c := range cases {
		tg, ok := Resolve(&Adapter{Kind: c.kind, Config: []byte(c.cfg)}, look)
		if !ok || tg.URL != c.url || tg.Shape != c.shape {
			t.Errorf("%s %s: %+v %v", c.kind, c.cfg, tg, ok)
		}
	}
	if _, ok := Resolve(&Adapter{Kind: "acp", Config: []byte(`{"argv":["x"]}`)}, look); ok {
		t.Error("acp resolved to an HTTP target")
	}
}
