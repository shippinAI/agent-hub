package devapi

import (
	"context"
	"encoding/json"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"agenthub/internal/mcptools"
	"agenthub/internal/store"
)

type providerListIn struct {
	EnabledOnly bool `json:"enabled_only,omitempty" jsonschema:"hide disabled rows"`
}

type providerIDIn struct {
	ID string `json:"id" jsonschema:"provider id, e.g. anthropic-claude-code"`
}

type providerRowIn struct {
	Row map[string]any `json:"row" jsonschema:"a full provider row (schema_version 1, see provider_schema)"`
}

type providerUpdateIn struct {
	ID      string         `json:"id"`
	Version int64          `json:"version" jsonschema:"the row's current version (from provider_get)"`
	Fields  map[string]any `json:"fields" jsonschema:"top-level fields to replace (merge), or the whole row when replace is true"`
	Replace bool           `json:"replace,omitempty"`
}

type providerDisableIn struct {
	ID       string `json:"id"`
	Disabled bool   `json:"disabled" jsonschema:"true to disable, false to enable again"`
}

func rawFields(m map[string]any) map[string]json.RawMessage {
	out := map[string]json.RawMessage{}
	for k, v := range m {
		out[k], _ = json.Marshal(v)
	}
	return out
}

// providerTools: the provider catalog over MCP. Reads and validate are open to every principal; create,
// update and disable are human-only plus policy (rows carry argv the bridge runs), same as REST.
func (a *API) providerTools(s *mcp.Server) {
	mcptools.Tool(s, "provider_schema", "The provider row vocabulary: adapter kinds, router modes, auth kinds, sources and adapter defaults.",
		func(ctx context.Context, p *store.Principal, in struct{}) (any, error) { return providerSchema(), nil })
	mcptools.Tool(s, "provider_list", "List provider rows (one per AI client or API; doc 34).",
		func(ctx context.Context, p *store.Principal, in providerListIn) (any, error) {
			l, err := a.Providers.List(ctx, in.EnabledOnly)
			return map[string]any{"providers": l}, err
		})
	mcptools.Tool(s, "provider_get", "One provider row with its cached model list.",
		func(ctx context.Context, p *store.Principal, in providerIDIn) (any, error) {
			return a.Providers.Get(ctx, in.ID)
		})
	mcptools.Tool(s, "provider_validate", "Check a provider row against the schema without saving. Returns ok and errors [{path, message}].",
		func(ctx context.Context, p *store.Principal, in providerRowIn) (any, error) {
			return a.Providers.Check(ctx, rawFields(in.Row))
		})
	mcptools.Tool(s, "provider_create", "Create a provider row (humans only). Fails with validation_failed and per-field errors.",
		func(ctx context.Context, p *store.Principal, in providerRowIn) (any, error) {
			body := rawFields(in.Row)
			delete(body, "version")
			return a.Providers.Save(ctx, p, "", "", body, false)
		})
	mcptools.Tool(s, "provider_update", "Update a provider row (humans only): merge fields, or replace the whole row; send the current version.",
		func(ctx context.Context, p *store.Principal, in providerUpdateIn) (any, error) {
			body := rawFields(in.Fields)
			body["version"], _ = json.Marshal(in.Version)
			return a.Providers.Save(ctx, p, "", in.ID, body, !in.Replace)
		})
	mcptools.Tool(s, "provider_disable", "Disable (or re-enable) a provider row (humans only). Rows are never deleted.",
		func(ctx context.Context, p *store.Principal, in providerDisableIn) (any, error) {
			return a.Providers.SetDisabled(ctx, p, "", in.ID, in.Disabled)
		})
}
