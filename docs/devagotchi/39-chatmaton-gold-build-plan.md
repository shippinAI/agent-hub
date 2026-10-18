# 39. Chatmaton on Gold: build plan (research, recommendation, phases P1–P5)

[Index](../devagotchi-spec.md) · Written 2026-10-18 for Jesse's goal of the same day. Grounded in the code at commit
`0646b44` (Go server in `server/`, UI in `web/`, migrations `001`–`008`, `server/internal/providers`) and in docs
[24](24-chatmaton-spec.md), [29](29-provider-matrix.md)–[38](38-action-feed-learning.md). Facts carry a source link;
design choices are marked **inference**. Nothing here was run on Gold or any other host. Every Gold change below is
a draft gold-policy ticket that still needs Jesse's approval ([14](14-policy-and-review.md)).

## Linked files (each ≤150 lines)
| File | What |
|---|---|
| [39a](39a-research-app-shells.md) | Research: Tauri 2, Wails v3, PWA and React Native, push for approvals, passkey and biometric signing, self-update |
| [39b](39b-research-offline-sync.md) | Research: offline sync for devpulse (ElectricSQL, PowerSync, Zero/Replicache, CRDTs, own Go protocol) |
| [39c](39c-architecture-on-gold.md) | Architecture on Gold, organised by SWEBOK v4 knowledge areas: seats, rooms, endpoints as data, tenant layers, Gold services |
| [39d](39d-p1-chat-page.md) | **P1** chat page: Claude Code, Codex and Grok CLIs in one room over WebSocket. Exact files, endpoints, migration, tests, tickets |
| [39e](39e-p2-approvals-audit-pwa.md) | **P2** approvals and audit, PWA first: Web Push, passkey-signed approvals, action feed (doc 38) |
| [39f](39f-p3-assignment-stats.md) | **P3** assign tasks to subscriptions; stats and costs per provider, prompt and task; Gold service tiles |
| [39g](39g-p4-p5-shells-update-sync.md) | **P4** native shells and signed self-update; **P5** devpulse sync |

## Jesse's goal, restated
1. **Chatmaton** is his private chat app, built as a real compiled app, like the ChatGPT, Grok and Claude apps. He
   signs in to his own subscriptions (18 providers as rows of the provider schema, [34](34-provider-schema.md)). Rooms
   run several providers together with shared state. It patches and updates itself with minisign-signed updates.
2. **Apps on phone, tablet and desktop:** approve changes, review the action feed, assign tasks to a subscription,
   and see stats and costs per provider, prompt and task.
3. **Devpulse:** a local-first client that syncs with the Gold devspace, scoped by tenant layers
   (system > plan > tenant > workspace, [35](35-tenant-defaults.md)).
4. **Gold services** (console, Vaultix, Zitadel, mail, DNS, Tailscale, Cloudflare) can be reached from the app, with
   UI-driven guided flows ([12](12-guides.md), [21](21-a2ui.md)).

## Recommendation, in one paragraph
Keep **one UI**: the React Aria web app that the Go hub already embeds and serves (`server/internal/httpapi/static`).
Ship it as an installable **PWA first** (P2): it works on iPhone, iPad, Android and desktop with no store. Web Push
reaches installed home-screen web apps on iOS 16.4+, with Declarative Web Push from iOS 18.4, and passkeys work in
every target browser. Then wrap the same build in **Tauri 2 shells** (P4) for macOS, Windows, Linux, iOS and Android.
The shells add native push (APNs/FCM), a biometric prompt, deep links, tray notifications and Tauri's minisign
updater on desktop. The desktop shell also hosts the Go devpulse binary as a sidecar. **Wails v3** is the most
Go-native option, but v3 is still in beta and its mobile support has been experimental since June 2026. Revisit it
when v3.0 ships. **React Native is rejected**: React Aria is web-only, so it would need a second component set, which
breaks the React-Aria-only rule. Sources and the scored comparison are in [39a](39a-research-app-shells.md).
Devpulse sync uses a **server-authoritative Go push/pull protocol** (Replicache-style mutation log) over SQLite on
the client. Conflict rules come from the lock modes of doc 35. PowerSync is the fallback if the scope grows
([39b](39b-research-offline-sync.md)).

## How the chat runs (P1 core idea, **inference** grounded in the code)
- The hub image stays distroless (`deploy/docker/hub-server.Dockerfile`). The pinned CLIs and their logins live in a
  **seats** environment on Gold: the existing Gold hub container, or a sibling container. A normal **bridge**
  (`bridge/`) is enrolled there as machine `gold-seats`.
- The hub drives each CLI through the bridge's **existing** process ops: `proc.start`, `proc.write` (stdin) and
  `proc.read` (long-poll with `after_seq`, up to 20 s) in `bridge/runner/proc.go`, called through
  `server/internal/machineops/proc.go`. Grants, policy, call signing and the action feed therefore all apply with no
  bridge change. The hub never sees a client credential, which keeps threat T11 of [08](08-threat-model.md) and
  rule 4 of [34](34-provider-schema.md).
- A new `server/internal/seats` package implements the `headless_json` and `codex_app_server` runtimes. A new
  `server/internal/rooms` package holds the shared room state in Postgres. A new `server/internal/chatws` package
  streams normalized frames to the browser over WebSocket (`github.com/coder/websocket`, already in `server/go.mod`).

## Phase map
| Phase | Ships | Closes / advances | Size |
|---|---|---|---|
| P1 | `#/chat`: one room seating Claude Code 2.1.302, Codex 0.163.0 and Grok 1.0.62 (pinned), WebSocket streaming, per-turn usage | G-21 (headless + app-server part), G-16 (rooms part) | ~2 weeks |
| P2 | Approvals inbox, action feed (doc 38), PWA install, Web Push, passkey-signed approvals, endpoints as data | G-15 (partial), new G-28/G-29 | ~2 weeks |
| P3 | Assign a task to a subscription; usage ledger; stats per provider, prompt and task; Gold service tiles | G-22, G-27 (status part) | ~2 weeks |
| P4 | Tauri 2 shells (5 targets), native push and biometric, minisign self-update for shells and Go binaries | G-09, G-10 | ~3 weeks |
| P5 | Devpulse: SQLite client, Go sync protocol, tenant-layer scoped pull, offline writes | G-25 (resolver part) | ~3 weeks |
Sizes are **inference** for one agent team with reviews. Every phase ships alone behind a setting flag
(`features.<phase>`, data in `settings`).

## Rules applied in every phase (from [devagotchi-spec](../devagotchi-spec.md))
React Aria Components only (no shadcn, no Radix). Files ≤150 lines. No barrel files. No `core` in names. Defaults
live in adapters or preset JSON, and every value is editable data. Endpoints (tailnet, LAN, Cloudflare/public) are
rows. Secrets are never printed: tokens are shown once, and Vaultix or `secretbox` hold the values. Every write goes
through `store.Write`, so it lands in the hash-chained `events` log.

## Open decisions for Jesse (3)
1. **Canonical hub domain for passkeys (P2).** WebAuthn binds a passkey to one RP ID, which must be a domain, never
   an IP. Option A: one name (e.g. `hub.<your-domain>`) that resolves on LAN and tailnet through split DNS and
   publicly through Cloudflare. Option B: the tailnet MagicDNS name plus a Related Origins list. A is simpler.
   **Proposed: A.**
2. **iOS distribution for the native shell (P4).** Apple Developer Program (TestFlight or ad-hoc, needed for APNs
   and the passkey associated domain), or stay PWA-only on iPhone and iPad. **Proposed: PWA until P4 shows a need.**
3. **P1 seat permissions.** Read-only/plan mode for all three CLIs (`profile: read_only`), or write access inside
   a scratch workspace on `gold-seats` (`profile: scratch_write`). **Proposed: read_only for P1. Writes come with the
   P2 approvals.**

## Not done here
No ticket was filed, no host was touched, and no board task was created. Ticket drafts are listed per phase. Ticket
IDs (`GC-CM-Pn-NN`) are placeholders until filed. The code comments in `008_action_audit.sql` still cite
"39-action-feed.md"; doc 38 already marks those references as stale, and this file does not take over that name.
