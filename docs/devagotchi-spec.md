# Devagotchi spec (v0.1 design, docs only)
Devagotchi (devagotchi.com) **extends** agent-hub; it does not replace it. The hub stays the single source of truth: lease-based claiming with fences, notes, the hash-chained `events` log, MCP at `/mcp`, and the React Aria UI ([spec.md](spec.md), [connect.md](connect.md)). This spec adds machines, grants, and provider bindings to that hub.
Research base: `shippin-cloud/docs/agent-hub/desktop-bridge-research.md` ("the research", researched 2026-10-17). Its sources are cited inline. **Inference** marks a design choice of ours that no source states. **Unverified** marks a claim we could not confirm.

## What will be built, and what each piece touches
| # | Piece | Touches | Does not touch |
|---|---|---|---|
| 1 | Schema additions: `machines`, `shell_profiles`, `capabilities`, `risks`, `access_requests`, `grants`, `proc_leases`, `providers`, `subscriptions`, `role_bindings`, `secrets`, `tenants` | The hub's Postgres DB (new migrations) and store package | Existing tables (except adding the nullable `tenant_id` in phase 7) |
| 2 | Presets for all of the above | `server/internal/presets/*.json`, seeded on first boot only, then edited in the UI | Code paths (no hardcoded hosts, roles, or policies) |
| 3 | MCP and REST additions (`request_access`, `exec`, `fs_*`, `proc_*`, `machines_list`, …) | hub-server `/mcp`, `/api/v1` | Agent configs (agents keep the one `HUB_TOKEN`) |
| 4 | `devagotchi-bridge` daemon (Go, a single signed binary) | Only the machine it is installed on: one config dir, one 0600 token file, an OS sandbox per call, and an outbound HTTPS/WSS connection to the hub | Inbound ports, the system shell profile, telemetry endpoints, and other machines |
| 5 | Access flow (request card, narrowing, grant chip, revoke) | Hub DB, SSE, and the UI | The bridge's own config (the model can never edit grants) |
| 6 | Subscriptions and role bindings | Hub DB, the router (LiteLLM-style) for API keys, and official CLIs on user devices as hub workers | Provider credentials of subscription logins (they never leave the device) |
| 7 | UI screens (desktop and phone) | `web/` (React Aria Components plus our own CSS) | No shadcn or Radix |
| 8 | Cloud edition (Shippin) | The same code with tenancy on, OAuth 2.1 at `/mcp` | The personal edition (local, single tenant, loopback) |
Nothing in this spec is built yet. Writing it touched only `docs/` in this repo, with no hosts changed.

## Files (each ≤150 lines)
1. [Data model](devagotchi/01-data-model.md): tables, data-vs-code rules, and presets
2. [Bridge daemon](devagotchi/02-bridge.md): design, sandbox, and how it improves on Desktop Commander
3. [Bridge protocol](devagotchi/03-bridge-protocol.md): enrollment, the wire, exec, edits, and presence
4. [Access flow](devagotchi/04-access-flow.md): explain, approve or narrow, remind, revoke, expire, audit
5. [Subscriptions and roles](devagotchi/05-subscriptions.md): the two editions and provider terms
6. [MCP tools and REST](devagotchi/06-mcp-tools.md)
7. [UI screens](devagotchi/07-ui.md): desktop and phone
8. [Threat model](devagotchi/08-threat-model.md)
9. [Phased plan](devagotchi/09-phases.md): what each phase touches
10. [Open decisions for Jesse](devagotchi/10-decisions.md)

## Rules that apply everywhere
- Every source, host, policy, role, capability, risk, provider, shell profile, and setting is **editable data**, seeded as presets and never hardcoded. Code holds only the safety invariants (listed in 01).
- UI uses React Aria Components only. No shadcn and no Radix.
- None of our own names contain `core`. No Authentik (human auth stays as in spec.md: tailscale-whois / proxy-header with Zitadel / token / dev).
- Secrets are never printed: not in logs, events, MCP results, the UI, or the CLI. Tokens are shown once, keys are write-only, and events carry only a prefix or fingerprint.
- Source files stay ≤150 lines, with no barrel files.
