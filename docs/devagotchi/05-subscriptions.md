# 5. Subscriptions, roles, and the two editions
[Index](../devagotchi-spec.md)

## Model
A user owns **many subscriptions** (`subscriptions`). Each is either:
- **`api_key`**: a provider API key stored write-only in `secrets`, used only through a **router endpoint** (LiteLLM-style proxy: virtual keys, per-key budgets and spend, per [LiteLLM docs](https://docs.litellm.ai/docs/proxy/virtual_keys); or OpenRouter BYOK with key priority and fallback, per [OpenRouter BYOK](https://openrouter.ai/docs/use-cases/byok)). The hub hands workers a router *virtual key*, never the provider key.
- **`cli_login`**: a consumer subscription (Claude Pro/Max, ChatGPT Plus/Pro) that is logged in **inside the official CLI on the user's own machine**. The hub stores no credential, only "machine X has CLI Y logged in as label Z". The CLI runs as a **hub worker**: it connects to `/mcp` with its own agent token and claims tasks like any agent ([connect.md](../connect.md)).
A **role binding** maps (role, chat app) → subscription, with priority and fallback: e.g. `builder@claude → "Claude Max (desk)"`, `reviewer@codex → "ChatGPT Pro (laptop)"`, `researcher@grok → "xAI API key via router"`. Roles, chat apps, and providers are preset rows, all editable.

## Provider terms (respected by design)
| Provider | Subscription use | API use | Status |
|---|---|---|---|
| Anthropic | Only through Claude Code / native apps. Third parties may not offer Claude.ai login or route requests through Free/Pro/Max credentials ([legal & compliance](https://code.claude.com/docs/en/legal-and-compliance)) | API key via router | **Verified** (per the research, 2026-10-17) |
| OpenAI | Codex supports Sign in with ChatGPT in its own app/CLI/IDE ([Codex auth](https://developers.openai.com/codex/auth)); third-party reuse of ChatGPT credentials | API key via router | Reuse rule **unverified** (policy page behind a JS wall); treated like Anthropic |
| xAI | No clause found on subscription-vs-API reuse ([x.ai ToS](https://x.ai/legal/terms-of-service)) | API key via router | **Unverified**; default to API key only |
Consequences, enforced in code (code rule: `cli_login` subscriptions can bind only to a worker principal running on the named machine):
- The hub **never** proxies, stores, copies, or replays a subscription token or cookie.
- A `cli_login` worker is launched by the user (or by the bridge under a user-approved `exec` grant whose argv is the official CLI) and talks to the hub over MCP. The chat itself stays inside the vendor's CLI.
- Whether running the official CLI non-interactively as a task worker is "ordinary use" is **unverified** for each vendor; `providers.terms_status` and `terms_note` show this in the UI, and decision D6 asks Jesse to accept it.

## Dispatch
1. A task gets a role label (e.g. `role:reviewer`). Lane rules still apply (`distinct_from_previous`).
2. The hub resolves the bindings for that role, in priority order, skipping disabled, over-budget, or offline (by honest presence) subscriptions.
3. `cli_login` → the task is made claimable only by that worker principal (`assignee`). `api_key` → a router-backed worker (a hub-side runner or a user's agent) claims it with a virtual key scoped to the budget.
4. Fallback is recorded as a note (`kind: decision`, "fell back from X to Y because offline").

## Editions (one design)
| | Personal (local) | Shippin cloud (multi-tenant) |
|---|---|---|
| Hub | Loopback / tailnet, single tenant `local` | Shippin-hosted, `tenant_id` on every row + Postgres row-level security |
| Human auth | as spec.md (tailscale-whois / token / dev) | proxy-header via Zitadel (no Authentik) |
| MCP auth | static bearer per agent | OAuth 2.1 resource server, RFC 9728 metadata, RFC 8707 `resource` ([MCP authorization](https://modelcontextprotocol.io/specification/2025-06-18/basic/authorization)); bearer kept for CLIs |
| Secrets | local key (D5) | per-tenant data key wrapped by a KMS/vault (D5) |
| Router | optional local LiteLLM | per-tenant router config (data) |
| Bridges | dial the local hub | dial the cloud hub; all execution stays on user machines |
| Subscriptions | `cli_login` workers on the user's own devices only | same, never on Shippin servers |
| Plans/limits | none | `tenants.plan` → limits as data (machines, grants, seats) |
The personal edition is the cloud edition with `tenancy_mode=single`. No forked code paths beyond auth adapters and RLS.
