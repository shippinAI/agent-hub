package hubtest

import (
	"strings"
	"testing"
)

func TestProvidersMCP(t *testing.T) {
	e := setup(t)
	agent := e.mcpSession("claude-code")
	out, isErr := callTool(t, agent, "provider_list", map[string]any{})
	if l, _ := out["providers"].([]any); isErr || len(l) < 20 {
		t.Fatalf("provider_list: %v", out)
	}
	out, _ = callTool(t, agent, "provider_get", map[string]any{"id": "openai-codex"})
	if out["adapter"].(map[string]any)["kind"] != "codex_app_server" {
		t.Fatalf("provider_get: %v", out)
	}
	bad := newRow("example-mcp")
	bad["models"] = map[string]any{"source": "command"}
	out, _ = callTool(t, agent, "provider_validate", map[string]any{"row": bad})
	if out["ok"] != false || !strings.Contains(strFmt(out), "models.ref") {
		t.Fatalf("provider_validate: %v", out)
	}
	if out, isErr = callTool(t, agent, "provider_create", map[string]any{"row": newRow("example-mcp")}); !isErr || !strings.Contains(strFmt(out), "humans_only") {
		t.Fatalf("agent provider_create: %v", out)
	}
	human := e.mcpSession("jesse")
	if out, isErr = callTool(t, human, "provider_create", map[string]any{"row": bad}); !isErr || !strings.Contains(strFmt(out), "validation_failed") {
		t.Fatalf("invalid provider_create: %v", out)
	}
	if out, isErr = callTool(t, human, "provider_create", map[string]any{"row": newRow("example-mcp")}); isErr {
		t.Fatalf("provider_create: %v", out)
	}
	out, isErr = callTool(t, human, "provider_update", map[string]any{"id": "example-mcp", "version": 1, "fields": map[string]any{"label": "Via MCP"}})
	if isErr || out["label"] != "Via MCP" {
		t.Fatalf("provider_update: %v", out)
	}
	if out, isErr = callTool(t, human, "provider_disable", map[string]any{"id": "example-mcp", "disabled": true}); isErr || out["disabled"] != true {
		t.Fatalf("provider_disable: %v", out)
	}
}
