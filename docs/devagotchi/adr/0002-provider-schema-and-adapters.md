# ADR 0002: One provider schema, five swappable adapters, own sign-in in both editions

- **Status:** Accepted (Jesse, 2026-10-18).
- **Context:** Chatmaton drives many AI subscriptions and APIs ([29](../29-provider-matrix.md)). Their clients
  differ in surface (ACP, Codex app-server, headless JSON, HTTP APIs), paths, quota and history
  ([30](../30-cost-tracking.md), [32](../32-provider-local-state.md)), and vendors rename and move them often.
  Earlier docs limited the hosted edition to API keys and kept a terms badge per provider.
- **Decision:**
  1. Every provider is a data row of one schema ([34](../34-provider-schema.md)): id, vendor, product, label, icon,
     auth kinds, runner, editions, adapter + config, fallback adapters, model-list source, usage and quota source,
     state read path, detection rules per OS, capabilities. The provider count does not matter.
  2. Code exists only for five adapters: `acp`, `codex_app_server`, `headless_json`, `openai_compatible`,
     `anthropic_compatible`, plus a router option on the API adapters. Adapters carry best-practice defaults that rows
     can override; a row can name fallbacks.
  3. Router default is `direct` keys; LiteLLM is optional and configurable per tenant or row; OpenRouter is a
     provider row.
  4. Both editions let users sign in with their own subscriptions. Local: the client runs on the user's machine.
     Hosted: the client runs on the user's machine via the bridge, or in the user's isolated workspace on
     shippin.cloud, where the user runs the client's own login. The hub never reads, copies or proxies a
     client credential. No terms badge; `terms_note` is plain info.
- **Consequences:** A new vendor is a new row, not a release, unless it needs a sixth adapter (that needs a new
  ADR). Paths and argv break when vendors change them, so rows are versioned and the UI shows the failing rule.
  Hosted workspaces need per-user isolation, encrypted home volumes and no cross-tenant mounts
  ([08](../08-threat-model.md) T11). Work: G-17, G-21–G-24 in [27](../27-gap-tracking.md).
- **Alternatives rejected:** one adapter per vendor (grows with the count, hardcodes vendor facts); API keys
  only in hosted (Jesse rejected, 2026-10-18); proxying subscription tokens through the hub (exposes
  credentials, and the official client is the documented interface).
