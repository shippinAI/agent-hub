package providers

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func with2(t *testing.T, row []byte, key string, v any) []byte {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(row, &m); err != nil {
		t.Fatal(err)
	}
	m[key] = v
	b, _ := json.Marshal(m)
	return b
}

func TestSchemaVersion(t *testing.T) {
	if _, errs := Parse(with(t, "schema_version", 2)); !hasErr(errs, "schema_version") || !strings.Contains(errs[0].Message, "newer") {
		t.Fatalf("newer row not refused: %v", errs)
	}
	for _, v := range []any{0, -1, "1", 1.5} {
		if _, errs := Parse(with(t, "schema_version", v)); !hasErr(errs, "schema_version") {
			t.Errorf("schema_version %v accepted", v)
		}
	}
	if _, errs := Parse(with(t, "schema_version", nil)); !hasErr(errs, "schema_version") {
		t.Error("missing schema_version accepted")
	}
}

func TestOlderRowsAreConverted(t *testing.T) {
	old := upgrades
	defer func() { upgrades = old }()
	upgrades = map[int]func(map[string]any) map[string]any{1: func(m map[string]any) map[string]any { m["label"] = "upgraded"; return m }}
	out, err := convertTo([]byte(`{"schema_version":1,"label":"x"}`), 2)
	if err != nil || !strings.Contains(string(out), `"label":"upgraded"`) || !strings.Contains(string(out), `"schema_version":2`) {
		t.Fatalf("v1 -> v2: %s %v", out, err)
	}
	if _, err := convertTo([]byte(`{"schema_version":1}`), 3); err == nil {
		t.Fatal("missing v2 -> v3 converter not reported")
	}
}

// Every shipped seed row validates, and the doc 34 table's rows are all there.
func TestSeedRowsValidate(t *testing.T) {
	b, err := os.ReadFile("../presets/devagotchi/provider-rows.json")
	if err != nil {
		t.Fatal(err)
	}
	var f SeedFile
	if err := json.Unmarshal(b, &f); err != nil {
		t.Fatal(err)
	}
	ids := map[string]bool{}
	for i, raw := range f.Rows {
		p, errs := Parse(raw)
		if len(errs) > 0 {
			t.Errorf("seed row %d (%s): %v", i, p.ID, errs)
		}
		if ids[p.ID] {
			t.Errorf("duplicate seed id %s", p.ID)
		}
		ids[p.ID] = true
	}
	for _, id := range []string{"anthropic-claude-code", "anthropic-claude-desktop", "openai-codex", "xai-grok-build",
		"google-antigravity", "cursor-cli", "github-copilot-cli", "cognition-devin", "amazon-kiro", "mistral-vibe",
		"meta-muse-code", "moonshot-kimi-code", "alibaba-qwen-code", "zai-glm-plan", "minimax-m-plan", "deepseek-api",
		"perplexity-api", "opencode-cli", "block-goose", "cline-cli"} {
		if !ids[id] {
			t.Errorf("doc 34 row %s missing from the seed", id)
		}
	}
}
