# Devagotchi spec (v0.1 design, docs only)
Devagotchi (devagotchi.com) **extends** agent-hub; it does not replace it. The hub stays the single source of truth: lease-based claiming with fences, notes, the hash-chained `events` log, MCP at `/mcp`, and the React Aria UI ([spec.md](spec.md), [connect.md](connect.md)). This spec adds machines, grants, desktop GUI control, guided setup content, and provider bindings to that hub.
Research base: `shippin-cloud/docs/agent-hub/desktop-bridge-research.md` ("the research", 2026-10-17) for the bridge, plus fresh research on 2026-10-17 for GUI control ([11](devagotchi/11-gui-control.md)), guided content ([12](devagotchi/12-guides.md)), and Docker setup ([13](devagotchi/13-docker-setup.md)). Sources are cited inline. **Inference** marks a design choice of ours that no source states. **Unverified** marks a claim we could not confirm from a primary source.

## Who is who
- **Operator**: the person at the machine who approves access and does the steps only a human may do. Can be technical or not; the spec assumes nothing either way.
- **The AI as SME**: the agent acts as the subject-matter expert. It follows Shippin-authored guides ([12](devagotchi/12-guides.md)) to walk the operator through granting access, enrolling machines, installing Docker, and similar tasks. It explains, checks, and does the steps it has a grant for (decision D10, resolved).

## What will be built, and what each piece touches
| # | Piece | Touches | Does not touch |
|---|---|---|---|
| 1 | Schema additions: `machines`, `shell_profiles`, `capabilities`, `risks`, `grant_presets`, `access_requests`, `grants`, `proc_leases`, `gui_sessions`, `gui_artifacts`, `guides`, `guide_versions`, `guide_runs`, `providers`, `subscriptions`, `role_bindings`, `secrets`, `tenants` | The hub's Postgres DB (new migrations) and store package | Existing tables (except adding the nullable `tenant_id` in phase 7) |
| 2 | Presets for all of the above, including the Shippin guide bundle | `server/internal/presets/*.json`, seeded on first boot only, then edited in the UI | Code paths (no hardcoded hosts, roles, policies, or guide text) |
| 3 | MCP and REST additions (`request_access`, `exec`, `fs_*`, `proc_*`, `gui_*`, `guides_*`, `machines_list`, …) | hub-server `/mcp`, `/api/v1` | Agent configs (agents keep the one `HUB_TOKEN`) |
| 4 | `devagotchi-bridge` daemon (Go, a single signed binary) | Only the machine it is installed on: one config dir, one 0600 token file, an OS sandbox per call, and an outbound HTTPS/WSS connection to the hub | Inbound ports, the system shell profile, telemetry endpoints, and other machines |
| 5 | GUI helper (`devagotchi-bridge gui-agent`, runs in the operator's desktop session) | Screen capture, input injection, and window/app control on that machine's desktop, only under a `gui.*` grant, through the OS permission gates (TCC, UIPI, portals) | Elevated/secure desktops (UAC, login), other users' sessions, the exec sandbox user |
| 6 | Access flow (request card, narrowing, grant presets, unsandboxed policy, reminders, revoke) | Hub DB, SSE, and the UI | The bridge's own config (the model can never edit grants) |
| 7 | Guides: versioned content, runner, admin editor, update flow | Hub DB (`guides*`), UI, MCP resources/prompts | Guide text in code; secrets in guides |
| 8 | Docker/Podman setup guides per OS | Guide content plus bridge calls under grants on the chosen machine | Privileged installs by the bridge (operator does elevated steps) |
| 9 | Subscriptions and role bindings | Hub DB, the router (LiteLLM-style) for API keys, and official CLIs on user devices as hub workers | Provider credentials of subscription logins (they never leave the device) |
| 10 | UI screens (desktop and phone) | `web/` (React Aria Components plus our own CSS) | No shadcn or Radix |
| 11 | Cloud edition (Shippin) | The same code with tenancy on, OAuth 2.1 at `/mcp` | The personal edition (local, single tenant, loopback) |
Nothing in this spec is built yet. Writing it touched only `docs/` in this repo, with no hosts changed.

## Files (each ≤150 lines)
1. [Data model](devagotchi/01-data-model.md): tables, data-vs-code rules, presets
2. [Bridge daemon](devagotchi/02-bridge.md): design, sandbox, Windows + WSL, improvements over Desktop Commander
3. [Bridge protocol](devagotchi/03-bridge-protocol.md): enrollment, the wire, exec, edits, GUI ops, presence
4. [Access flow](devagotchi/04-access-flow.md): explain, approve or narrow, presets, unsandboxed policy, remind, revoke, audit
5. [Subscriptions and roles](devagotchi/05-subscriptions.md): the two editions and provider terms
6. [MCP tools and REST](devagotchi/06-mcp-tools.md)
7. [UI screens](devagotchi/07-ui.md): desktop and phone
8. [Threat model](devagotchi/08-threat-model.md)
9. [Phased plan](devagotchi/09-phases.md): each phase opens with what it touches
10. [Decisions](devagotchi/10-decisions.md): resolved and open
11. [Desktop GUI control](devagotchi/11-gui-control.md): per-OS APIs, permissions, grant scopes, risks, audit, screenshots
12. [Guides](devagotchi/12-guides.md): Shippin-authored versioned content, the AI as SME, editor, updates
13. [Docker setup](devagotchi/13-docker-setup.md): Docker Desktop vs Engine, WSL2, Podman preset
14. [Policy gate and reviewers](devagotchi/14-policy-and-review.md): versioned tenant policies, decision log, change tickets, reviewer agents
15. [Live ops](devagotchi/15-live-ops.md): supervisor, reachability, Online semantics, gold-policy hub mode, install bundles
16. [Desktop Commander parity](devagotchi/16-dc-parity.md): every DC tool mapped to ours, improvements, gaps left
17. [Standalone mode, release, benchmarks](devagotchi/17-standalone-release.md): subcommands, install, client config, sizes, numbers
18. [Public product research](devagotchi/18-public-product-research.md): client listing rules, update practice
19. [Public product spec](devagotchi/19-public-product-spec.md): packaging channels, signed updates, extensions, config versions
20. [Turnkey environment setup](devagotchi/20-env-setup.md): WSL, Docker, Podman as test/apply/verify components
21. [A2UI surfaces](devagotchi/21-a2ui.md): protocol, our catalog, React Aria mapping, a11y
22. [Website](devagotchi/22-website.md): Next.js 16 vs TanStack Start, recommendation
23. [Chatmaton research](devagotchi/23-chatmaton-research.md): which subscriptions and CLIs can be driven
24. [Chatmaton spec](devagotchi/24-chatmaton-spec.md): editions, tabs, rooms, roles, routines, approvals
25. [Desktop Commander lessons](devagotchi/25-dc-lessons.md): DC issues by category and how we avoid each
26. [Connectivity layers](devagotchi/26-connectivity.md): HTTPS first, then an owned WireGuard mesh
27. [Gap tracking](devagotchi/27-gap-tracking.md): register G-01…G-27, board tasks, ADRs ([ADR 0001](devagotchi/adr/0001-track-gaps-as-tasks-and-adrs.md))
28. [Public gaps, phases, open questions](devagotchi/28-public-gaps-and-phases.md)
29. [Provider matrix](devagotchi/29-provider-matrix.md): the 18 subscriptions, surface per client, model list sources
30. [Cost and usage sources](devagotchi/30-cost-tracking.md): price lists, usage APIs, quota windows, accounting rules, router option
31. [Installed awareness](devagotchi/31-installed-awareness.md): detect other AI tools, import MCP servers, avoid conflicts, share AGENTS.md and skills
32. [Provider local state](devagotchi/32-provider-local-state.md): config, MCP and session paths, read surfaces, detection per OS
33. [Cost data model and UI](devagotchi/33-cost-data-and-ui.md): ledger tables, budgets and alerts, default graphs, React Aria screens
34. [Provider schema](devagotchi/34-provider-schema.md): one row shape for every provider, five adapters, router option ([ADR 0002](devagotchi/adr/0002-provider-schema-and-adapters.md))
35. [Tenant defaults](devagotchi/35-tenant-defaults.md): Shippin console layers, lock modes, inheritance and overrides
36. [Vaultix](devagotchi/36-vaultix.md): rebranded Infisical, Doppler-style additions, A2UI secret surfaces
37. [Subscription sign-in](devagotchi/37-subscription-sign-in.md): ChatGPT/Codex and Grok official flows, hub and console use

## Rules that apply everywhere
- Every source, host, policy, role, capability, risk, grant preset, guide, provider, shell profile, and setting is **editable data** in the UI, seeded as presets and never hardcoded. Code holds only the safety invariants (listed in 01).
- UI uses React Aria Components only. No shadcn and no Radix.
- None of our own names contain `core`. No Authentik (human auth stays as in spec.md: tailscale-whois / proxy-header with Zitadel / token / dev).
- Secrets are never printed: not in logs, events, MCP results, the UI, the CLI, guides, or screenshots we store. Tokens are shown once, keys are write-only, and events carry only a prefix or fingerprint.
- Source files stay ≤150 lines, with no barrel files.
