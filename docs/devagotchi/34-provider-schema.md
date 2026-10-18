# 34. Provider schema: one row shape, few adapters

[Index](../devagotchi-spec.md) · Decision record: [ADR 0002](adr/0002-provider-schema-and-adapters.md). Facts per
provider: [29](29-provider-matrix.md) (surfaces), [32](32-provider-local-state.md) (paths, detection),
[30](30-cost-tracking.md) (usage, quota). The number of providers does not matter: every provider is one data row
of this shape, and code exists only for the five adapters plus the router option.

## Rules
1. **Names:** `snake_case`; ids are `vendor-product` (`anthropic-claude-code`); enums are lower-case words.
2. **No provider-specific code.** Argv, env, paths, JSON paths, regexes and endpoints live in the row. An adapter
   ships best-practice defaults (timeouts, retry, backoff, stream framing) that a row may override.
3. **Swappable.** `adapter` is the preferred one; `fallback_adapters` are tried in order when the first is not
   available on that machine (e.g. ACP via a community bridge, then headless).
4. **Both editions, own sign-in.** Local and hosted (shippin.cloud) both let users sign in with their own
   subscriptions (Jesse, 2026-10-18). `runner` says where the official client runs: `user_device` (bridge on the
   user's machine) or `hosted_workspace` (the user's isolated workspace on shippin.cloud, where the user runs
   the client's own login). The hub never reads, copies or proxies a client credential in either case.
5. **Versioned.** `schema_version` (integer) on every row; newer-than-known rows are refused, older rows are
   converted ([19](19-public-product-spec.md) config versioning).

## Adapters (the only code)
| `adapter.kind` | Talks to | Config keys |
|---|---|---|
| `acp` | Any ACP agent over stdio ([ACP](https://agentclientprotocol.com/get-started/agents)) | `argv`, `env`, `session_load` |
| `codex_app_server` | Codex JSON-RPC app-server ([docs](https://learn.chatgpt.com/docs/app-server)) | `argv`, `env`, `rate_limits` (bool) |
| `headless_json` | One process per turn, JSON or JSONL out | `argv`, `prompt_arg`, `resume_arg`, `output`, `session_id_path`, `text_path`, `usage_path`, `event_map` |
| `openai_compatible` | OpenAI-compatible HTTP (Chat Completions / Responses) | `base_url`, `api`, `secret_ref`, `router` |
| `anthropic_compatible` | Anthropic-compatible Messages HTTP | `base_url`, `secret_ref`, `router` |
`router` (API adapters only): `{ mode: "direct" | "litellm" | "openrouter" | "custom", endpoint_id }`.
**Default `direct`**; LiteLLM is optional and set per tenant or per row ([30](30-cost-tracking.md)).

## Schema sketch (ArkType-style)
```ts
const Os = type("'linux' | 'macos' | 'windows'")
const Detect = type({
  "bins?": "string[]",            // names looked up on PATH, then dirs
  "dirs?": "string[]",            // config dirs; ~ and %APPDATA% expanded by the bridge
  "app_ids?": "string[]",         // macOS bundle id, Windows AppX/Uninstall id, Linux .desktop id
  "version_cmd?": "string[]", "version_regex?": "string",
  "signed_in_cmd?": "string[]"    // the client's own status command; never a token file
})
const Provider = type({
  schema_version: "1", id: /^[a-z0-9]+(-[a-z0-9]+)+$/, vendor: "string", product: "string",
  label: "string", icon_ref: "string", "docs_url?": "string",
  auth_kinds: "('client_login' | 'signin_oauth' | 'api_key' | 'plan_key')[]",
  runner: "('user_device' | 'hosted_workspace' | 'hub')[]",   // hub = API adapters only
  editions: "('local' | 'hosted')[]",
  adapter: { kind: "'acp' | 'codex_app_server' | 'headless_json' | 'openai_compatible' | 'anthropic_compatible'",
             config: "Record<string, unknown>" },             // validated per kind
  "fallback_adapters?": "Adapter[]",
  "host_for?": "'harness'",       // plan keys (Z.ai, MiniMax) run inside another row's client
  models: { source: "'adapter' | 'command' | 'http' | 'static'", "ref?": "string",
            refresh_minutes: "number = 360" },
  usage: { tokens: "'stream' | 'otel' | 'api_response' | 'none'",
           quota: "'adapter' | 'statusline' | 'http' | 'observed' | 'none'", "quota_ref?": "string",
           cost_basis: "'provider_charged' | 'computed' | 'client_estimate'" },
  state: { read: "'adapter' | 'command' | 'http' | 'file' | 'live_only'", "ref?": "string",
           "paths?": "Partial<Record<Os, string[]>>", "parser?": "string" },  // parser = id@version
  detect: "Partial<Record<Os, Detect>>",
  capabilities: { streaming: "boolean", tools: "boolean", mcp_client: "boolean", mcp_host: "boolean",
                  resume: "boolean", fork: "boolean", images: "boolean", reasoning_effort: "boolean",
                  approvals: "boolean" },
  "terms_url?": "string", "terms_note?": "string", version: "number"
})
```
Per-kind `config` schemas are separate types (`AcpConfig`, `HeadlessJsonConfig`, …) chosen by `adapter.kind`.
Postgres: one `providers` table with these columns as `jsonb` where nested; `models` cache in
`provider_models(provider_id, model, label, fetched_at, raw)`. Replaces the `providers` row in [01](01-data-model.md).

## Example row (full)
```json
{ "schema_version": 1, "id": "xai-grok-build", "vendor": "xAI", "product": "Grok Build",
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
     "fork": true, "images": true, "reasoning_effort": true, "approvals": true }, "version": 1 }
```
Sources for this row: [CLI](https://docs.x.ai/build/cli/reference), [sessions](https://docs.x.ai/build/features/sessions).

## The 18 as rows (key fields; full facts and sources in 29/30/32)
| id | auth_kinds | adapter.kind (fallback) | models.source | usage.quota | state.read |
|---|---|---|---|---|---|
| `anthropic-claude-code` | client_login, api_key | headless_json (acp via adapter) | command `/model`; http `/v1/models` | statusline | command `/export`; file parser |
| `anthropic-claude-desktop` | client_login | none (`mcp_host` only) | — | none | live_only |
| `openai-codex` | client_login, signin_oauth, api_key | codex_app_server (acp via adapter) | adapter `model/list` | adapter `account/rateLimits/read` | adapter `thread/turns/list` |
| `xai-grok-build` | client_login, api_key | acp (headless_json) | command `grok models` | observed | command `grok export` |
| `google-antigravity` | client_login | headless_json | command `agy models` | observed | live_only |
| `cursor-cli` | client_login | acp (headless_json) | command `agent models` | observed (team: http admin API) | command `agent ls`; file |
| `github-copilot-cli` | client_login | acp (headless_json) | adapter session config | http billing usage | file `session-state` |
| `cognition-devin` | client_login | acp | adapter session config | observed | live_only |
| `amazon-kiro` | client_login | acp | adapter session config | observed | command `--resume-id` |
| `mistral-vibe` | client_login, api_key | acp (headless_json) | http `/v1/models` | observed | file (opt-in logs) |
| `meta-muse-code` | client_login, api_key | headless_json | static (plan docs) | observed | command `--session-id` |
| `moonshot-kimi-code` | client_login | acp (headless_json) | adapter session config | observed | adapter `session/load` |
| `alibaba-qwen-code` | client_login, api_key | acp (headless_json) | adapter session config | observed | adapter `session/load` |
| `zai-glm-plan` | plan_key | `host_for: harness` + anthropic_compatible | static (plan docs) | http usage-query | host row |
| `minimax-m-plan` | plan_key | `host_for: harness` + anthropic_compatible | static (plan docs) | observed | host row |
| `deepseek-api` | api_key | anthropic_compatible (openai_compatible) | http `/models` | http `/user/balance` | live_only |
| `perplexity-api` | api_key | openai_compatible | static (pricing docs) | none | live_only |
| `opencode-cli` | client_login, api_key | acp (`opencode serve` http) | adapter session config | per backing provider | http `GET /session` |
| `block-goose` | api_key | acp | adapter session config | per backing provider | command `goose session list` |
| `cline-cli` | api_key, client_login | acp (headless_json) | adapter session config | per backing provider | live_only |
Claude Desktop (29 row 1) and Goose (row 17) get their own rows, so 20 rows cover the 18 entries of 29.
"observed" means quota is learned from limit-reached events, since no read surface was found ([30](30-cost-tracking.md)).

## As built (2026-10-18; G-17 done, G-21 schema part)
- Adapter kinds renamed `openai_api` → `openai_compatible`, `anthropic_api` → `anthropic_compatible` (Jesse's task
  wording); the five kinds are the only ones the validator accepts. Ids `opencode`/`goose` became `opencode-cli` and
  `block-goose` so every id matches the `vendor-product` rule.
- Code: `server/internal/providers` (Go validator, per-kind config types, converter, seed); migration `007` (old
  values kept in `providers_legacy_007`, old `models` lists moved to `provider_models`, which adds `position`).
  Seed data: `presets/devagotchi/provider-rows.json` (these 20 rows plus `anthropic-api`, `openai-api`, `xai-api`,
  direct keys) and `provider-models.json`. Web types: `web/src/lib/providerTypes.ts` (enum lists tested against Go).
- Rules the validator adds: `adapter` may be `null` only for an MCP-host-only row (`capabilities.mcp_host`); a
  fallback may not repeat an adapter kind; runner `hub` needs an API adapter; config keys are strict per kind (plus
  overrides `timeout_ms`, `max_retries`, `backoff_ms`; headless adds `framing` json|jsonl); env names that look
  secret must hold a `{{secret:name}}` reference; detect dirs and state paths may not name credential stores;
  `secret_ref` names a stored secret, never a key. Router endpoint existence and kind = mode are checked against
  `router_endpoints`. `disabled_at` turns a row off; rows are not deleted through the API.
- API: `GET/POST /api/v1/providers`, `GET/PUT/PATCH /api/v1/providers/{id}`, `POST …/{id}/disable|enable`,
  `PUT …/{id}/models`, `POST /api/v1/providers/validate`, `GET /api/v1/provider-schema`; MCP `provider_schema`,
  `provider_list`, `provider_get`, `provider_validate`, `provider_create`, `provider_update`, `provider_disable`.
  Errors: 422 `validation_failed`, `current.errors = [{path, message}]`. Writes are human-only plus policy.
