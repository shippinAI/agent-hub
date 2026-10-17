# 10. Open decisions for Jesse
[Index](../devagotchi-spec.md) · Each has a proposed default, used by the spec until answered.

| # | Decision | Options | Proposed default |
|---|---|---|---|
| D1 | Allow an `unsandboxed` grant level? | never / per-grant with a red banner (Codex `danger-full-access` style) | Off (`unsandboxed_allowed=false`), toggle per tenant |
| D2 | Windows: native only, or WSL as a separate machine? | native / native + WSL machine | Native first; WSL enrolled as its own Linux machine |
| D3 | Approval channel | hub card only / MCP elicitation only / both | Both, hub card canonical |
| D4 | Default grant duration | `task` / wall-clock / once | `task` (dies with the lease) |
| D5 | Secret store | Postgres + pgcrypto with a local key file / OS keychain on the bridge / external vault (cloud) | Personal: local key file 0600 + pgcrypto; cloud: KMS-wrapped per-tenant keys |
| D6 | Accept that Claude/ChatGPT subscriptions run only as on-device official CLIs (hub workers), never proxied? Non-interactive worker use is unverified as "ordinary use" | accept / API keys only | Accept, with a visible "unverified" badge |
| D7 | Desktop GUI automation (Windows-MCP style) in scope? | no / later phase | No for v0.1 |
| D8 | Tenancy in the personal edition | `tenant_id` from day one / add in phase 7 | New tables get it in phase 1; existing tables in phase 7 |
| D9 | macOS sandbox fallback if `sandbox-exec` goes away | Endpoint Security / VM / refuse sandboxed grants | Refuse (read-only only) until decided |
| D10 | What "SME" means for the access flow copy | small-business owner (plain language) / subject-matter expert reviewer | Plain language for non-technical owners, technical detail collapsed |
| D11 | Audit anchoring | none / periodic hash export to a file or git / external timestamping | Daily export of the head hash to a file the user keeps |
| D12 | Bridge distribution | GitHub releases / devagotchi.com downloads / package managers | Signed releases on devagotchi.com + checksums |
| D13 | Router | LiteLLM (self-host) / OpenRouter BYOK / both as data | Both as `router_endpoints` presets; LiteLLM default for personal |
| D14 | Cloud pricing/plan limits for Shippin | – | Out of scope here; plan limits stay data |

## Unverified items carried in this spec
- OpenAI policy on third-party reuse of ChatGPT sign-in (policy page behind a JS wall).
- xAI subscription-vs-API reuse (no clause found in the ToS updated 2026-10-12).
- Whether running official CLIs non-interactively as workers counts as ordinary use (all vendors).
- Long-term availability of macOS `sandbox-exec`.
- Goose / Open Interpreter / LibreChat comparisons (pages did not render for the research); Reddit sentiment not covered (blocked).
