package providers

import (
	"encoding/json"
	"strings"
	"testing"
)

// grok is the full example row from doc 34.
const grok = `{ "schema_version": 1, "id": "xai-grok-build", "vendor": "xAI", "product": "Grok Build",
  "label": "Grok", "icon_ref": "icons/xai", "auth_kinds": ["client_login", "api_key"],
  "runner": ["user_device", "hosted_workspace"], "editions": ["local", "hosted"],
  "adapter": { "kind": "acp", "config": { "argv": ["grok", "agent", "stdio"], "session_load": true } },
  "fallback_adapters": [{ "kind": "headless_json", "config": { "argv": ["grok", "-p"],
     "output": ["--output-format", "json"], "resume_arg": "--resume", "session_id_path": "$.sessionId" } }],
  "models": { "source": "command", "ref": "grok models", "refresh_minutes": 360 },
  "usage": { "tokens": "stream", "quota": "observed", "cost_basis": "client_estimate" },
  "state": { "read": "command", "ref": "grok export", "paths": { "linux": ["~/.grok/sessions"] } },
  "detect": { "linux": { "bins": ["grok"], "dirs": ["~/.grok"], "version_cmd": ["grok", "version"] } },
  "capabilities": { "streaming": true, "tools": true, "mcp_client": true, "mcp_host": false, "resume": true,
     "fork": true, "images": true, "reasoning_effort": true, "approvals": true } }`

// with returns the grok row with one dotted path set (or removed when v is nil).
func with(t *testing.T, path string, v any) []byte {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(grok), &m); err != nil {
		t.Fatal(err)
	}
	keys := strings.Split(path, ".")
	cur := m
	for _, k := range keys[:len(keys)-1] {
		cur = cur[k].(map[string]any)
	}
	if v == nil {
		delete(cur, keys[len(keys)-1])
	} else {
		cur[keys[len(keys)-1]] = v
	}
	b, _ := json.Marshal(m)
	return b
}

func hasErr(errs Errs, path string) bool {
	for _, e := range errs {
		if e.Path == path {
			return true
		}
	}
	return false
}

func TestDocExampleRowIsValid(t *testing.T) {
	p, errs := Parse([]byte(grok))
	if len(errs) > 0 {
		t.Fatalf("doc 34 example row: %v", errs)
	}
	if p.Adapter.Kind != "acp" || len(p.FallbackAdapters) != 1 || p.Models.RefreshMinutes != 360 {
		t.Fatalf("decoded wrong: %+v", p)
	}
}

func TestFieldErrors(t *testing.T) {
	cases := []struct {
		name, path string
		v          any
		want       string
	}{
		{"bad id", "id", "Grok", "id"},
		{"id without product", "id", "grok", "id"},
		{"missing label", "label", "", "label"},
		{"unknown auth kind", "auth_kinds", []string{"cookie"}, "auth_kinds[0]"},
		{"no runner", "runner", []string{}, "runner"},
		{"hub runner on a CLI adapter", "runner", []string{"hub"}, "runner"},
		{"bad edition", "editions", []string{"cloud"}, "editions[0]"},
		{"sixth adapter kind", "adapter", map[string]any{"kind": "grok_native", "config": map[string]any{}}, "adapter.kind"},
		{"old adapter name", "adapter", map[string]any{"kind": "openai_api", "config": map[string]any{}}, "adapter.kind"},
		{"acp without argv", "adapter", map[string]any{"kind": "acp", "config": map[string]any{}}, "adapter.config.argv"},
		{"unknown config key", "adapter", map[string]any{"kind": "acp", "config": map[string]any{"argv": []string{"x"}, "argz": 1}}, "adapter.config.argz"},
		{"literal secret in env", "adapter", map[string]any{"kind": "acp", "config": map[string]any{"argv": []string{"x"}, "env": map[string]string{"XAI_API_KEY": "xai-123"}}}, "adapter.config.env.XAI_API_KEY"},
		{"bad models source", "models.source", "guess", "models.source"},
		{"command models without ref", "models.ref", nil, "models.ref"},
		{"refresh too large", "models.refresh_minutes", 99999, "models.refresh_minutes"},
		{"bad quota", "usage.quota", "unlimited", "usage.quota"},
		{"bad state read", "state.read", "scrape", "state.read"},
		{"credential path", "state.paths", map[string]any{"linux": []string{"~/.grok/auth.json"}}, "state.paths.linux[0]"},
		{"credential dir", "detect", map[string]any{"linux": map[string]any{"dirs": []string{"~/.codex/auth.json"}}}, "detect.linux.dirs[0]"},
		{"bad os", "detect", map[string]any{"amiga": map[string]any{"bins": []string{"x"}}}, "detect"},
		{"bad regex", "detect", map[string]any{"linux": map[string]any{"version_regex": "("}}, "detect.linux.version_regex"},
		{"missing capability", "capabilities.approvals", nil, "capabilities.approvals"},
		{"bad docs url", "docs_url", "ftp://x", "docs_url"},
		{"removed terms_status", "terms_status", "verified", "terms_status"},
		{"bad host_for", "host_for", "proxy", "host_for"},
	}
	for _, c := range cases {
		_, errs := Parse(with(t, c.path, c.v))
		if len(errs) == 0 {
			t.Errorf("%s: no error", c.name)
			continue
		}
		if c.want != "" && !hasErr(errs, c.want) {
			t.Errorf("%s: want error at %s, got %v", c.name, c.want, errs)
		}
	}
}

func TestDuplicateFallbackKind(t *testing.T) {
	_, errs := Parse(with(t, "fallback_adapters", []any{map[string]any{"kind": "acp", "config": map[string]any{"argv": []string{"g"}}}}))
	if !hasErr(errs, "fallback_adapters[0].kind") {
		t.Fatalf("duplicate adapter kind accepted: %v", errs)
	}
}

func TestMCPHostOnlyRowNeedsNoAdapter(t *testing.T) {
	b := with(t, "adapter", nil)
	b2 := []byte(strings.Replace(string(b), `"mcp_host":false`, `"mcp_host":true`, 1))
	var m map[string]any
	_ = json.Unmarshal(b2, &m)
	delete(m, "fallback_adapters")
	b2, _ = json.Marshal(m)
	if _, errs := Parse(b2); len(errs) > 0 {
		t.Fatalf("mcp host row: %v", errs)
	}
	if _, errs := Parse(b); !hasErr(errs, "adapter") {
		t.Fatalf("missing adapter accepted on a non-host row: %v", errs)
	}
}
