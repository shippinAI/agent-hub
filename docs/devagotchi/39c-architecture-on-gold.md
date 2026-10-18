# 39c. Architecture on Gold, by SWEBOK v4 knowledge area

[Plan](39-chatmaton-gold-build-plan.md) · Organised by the SWEBOK Guide V4.0 knowledge areas (KAs)
([IEEE CS](https://www.computer.org/education/bodies-of-knowledge/software-engineering),
[PDF](https://ieeecs-media.computer.org/media/education/swebok/swebok-v4.pdf)). Each phase file uses the same
headings. Go packages keep their domain names; each one is tagged with its KA here, so the repo layout does not
change (**inference**: moving folders now would churn every import for no gain).

## Software Requirements
| ID | Requirement | Phase |
|---|---|---|
| R-1 | One room seats several providers. Every seat sees the shared transcript, and each keeps its own CLI session | P1 |
| R-2 | Turns stream token by token to every open client; a reconnect loses nothing that was persisted | P1 |
| R-3 | Approve or deny tickets, access requests and room gates from a phone, with a signed, auditable approval | P2 |
| R-4 | Every agent action and script appears in the feed with a plain-English line and term tooltips ([38](38-action-feed-learning.md)) | P2 |
| R-5 | Assign a task to a chosen subscription; see usage and cost per provider, prompt and task | P3 |
| R-6 | Native apps on 5 targets; self-update with minisign; self-patch only through an approved ticket | P4 |
| R-7 | Devpulse works offline and syncs tenant-scoped data with conflict rules from lock modes | P5 |
| R-8 | Gold services (console, Vaultix, Zitadel, mail, DNS, Tailscale, Cloudflare) are reachable with guided flows | P2–P3 |

## Software Architecture
```
 phone / tablet / desktop          Gold (home server, Incus; shippin-cloud/docs/04-build-plan.md)
 ┌──────────────────────┐   HTTPS/WSS   ┌──────────────────────────────────────────────────────────┐
 │ PWA (P2) / Tauri (P4)│──────────────▶│ hub container: hub-server (distroless) + Postgres         │
 │ same web/ React Aria │  endpoint rows│   rooms · seats runtime · chatws · approvals · notify     │
 └──────────────────────┘  (tailnet,LAN,│   providers · changepolicy · actionfeed · syncapi (P5)    │
 ┌──────────────────────┐   public)     │            │ bridge wire (existing, signed calls)          │
 │ devpulse (P5)        │──────────────▶│ seats env: bridge as machine `gold-seats` + pinned CLIs   │
 │ Go + SQLite, Tauri   │               │   claude 2.1.302 · codex 0.163.0 · grok 1.0.62; own logins│
 │ desktop sidecar      │               │ Vaultix · Zitadel · mail · PowerDNS · Caddy · Tailscale   │
 └──────────────────────┘               └──────────────────────────────────────────────────────────┘
```
- **Seats run behind the bridge.** The hub drives the CLIs with the existing `proc.*` ops (`bridge/runner/proc.go`,
  `server/internal/machineops/proc.go`), so CLI credentials stay inside the seats env (T11). Provider rows keep
  `runner: user_device`, so the `runner=hub is API-only` check in `providers/validate.go:89` is unchanged.
- **One authority:** Postgres on Gold. Clients hold caches (the PWA keeps an app-shell cache; devpulse keeps SQLite).
- **Streaming:** browser ⇄ hub over WebSocket (P1, `chatws`). hub ⇄ bridge uses the existing WSS frames.
  Governance updates keep using SSE `/api/v1/stream` (`httpapi/events.go`).

## Software Design (new packages; KA tag in brackets)
| Package | Responsibility | Phase |
|---|---|---|
| `server/internal/rooms` [Design] | Rooms, seats, messages, turns, modes; per-seat context delta | P1 |
| `server/internal/seats` [Construction] | Adapter runtimes: `headless_json`, `codex_app_server` (P1), `acp` (P1.x); normalized events | P1 |
| `server/internal/chatws` [Architecture] | WebSocket tickets, upgrade, per-room fan-out, frames | P1 |
| `server/internal/approvals`, `passkeys`, `notify` [Security] | Unified inbox, WebAuthn signing, push channels | P2 |
| `server/internal/endpoints`, `goldsvc` [Operations] | Endpoint rows and probes; Gold service catalog and health | P2–P3 |
| `server/internal/usage`, `assign` [Economics] | Usage ledger, rollups, task-to-subscription routing | P3 |
| `server/internal/releases` [Config Mgmt] | Update manifests, minisign verification, channels | P4 |
| `server/internal/syncapi`, `devpulse/` [Construction] | Push/pull protocol; the client binary | P5 |

**Room shared state (inference).** The transcript in `room_messages` is the shared state. Each seat keeps its own
CLI session (`room_seats.session_ref`: Claude `session_id`, Codex `threadId`, Grok `sessionId`) and a high-water
mark `last_seen_msg`. On its turn, a seat gets only the messages after its mark, wrapped by the template in
setting `chat.context_template`. The default wraps each message as
`<room-message from="seat:codex" id="…">…</room-message>`, so other seats' text reads as quoted data, never as
instructions. Resume keeps each CLI's own context, so the hub never re-sends the whole transcript.

**Normalized seat events** (one Go type, `seats.Event`): `started{session_ref}`, `delta{item_id,text}`,
`item{kind: reasoning|tool|command|file_change|plan, summary}`, `usage{input,output,cache_read,cache_write,
reasoning,cost_usd,cost_basis}`, `done{stop_reason}`, `error{code,message}`. Each adapter maps its stream to these
(Claude `stream_event`/`result`; Codex `item/agentMessage/delta`/`turn/completed`; Grok `text`/`usage`/`end`).

## Endpoints as data
`access_endpoints(id, kind tailnet|lan|public, url, priority, enabled, probe_path, note, version)` (P2 migration).
Clients fetch the list after sign-in and cache it. They probe in priority order and use the first that answers
`GET /healthz`. Presets: one row of each kind, all editable. Cloudflare (public) is off by default until a Gold
ticket enables the tunnel or route. Router endpoints for LLM APIs stay in `router_endpoints` (migration 003).

## Tenant layers
Chat, notify and sync settings are catalog keys resolved system > plan > tenant > workspace (> user) once G-25
lands ([35](35-tenant-defaults.md)). Until then they are `settings` rows (migration 001) with the same keys, and the
resolver reads `settings` as the system layer. Keys: `chat.*`, `notify.*`, `sync.*`, `updates.*`.

## Gold services (R-8)
`gold_services(id, label, kind console|vaultix|zitadel|mail|dns|tailscale|cloudflare|custom, endpoint_ids text[],
health jsonb, guide_ids text[], deep_link, icon_ref, version)`: rows, never code. Each tile shows health and a
deep link, plus "Guided:" buttons that run a guide ([12](12-guides.md)) through A2UI modals ([21](21-a2ui.md)).
Writes to a service (DNS record, Zitadel app, mail alias) are guide steps that file a gold-policy ticket. They are
never direct API calls from the app.

## Software Security (threats added by this plan; numbering continues [08](08-threat-model.md))
| Threat | Mitigation |
|---|---|
| Cross-seat prompt injection (one model's output steers another) | Other seats' text is wrapped as data. P1 default profile is `read_only`; writes need P2 approvals |
| CLI credential exposure | Credentials live only in the seats env HOME; the hub never reads them; logins are device-code by Jesse |
| WebSocket hijack / CSRF | Single-use 60 s ticket bound to principal and room; Origin check; no cookie-only auth on upgrade |
| Push payload leaks | Payload holds only an id, a sha prefix and a link; details are fetched after auth |
| Forged or replayed approval | Passkey assertion over the subject sha256 plus a nonce; UV required; stored for re-verification |
| Malicious update | Minisign signature verification before swap; key offline; channel pinning; rollback |

## Software Engineering Operations, Maintenance, Configuration Management
- Every Gold change is a gold-policy ticket with a steps file and a gold-rec run ([38](38-action-feed-learning.md)
  ingest). Phase files list the drafts.
- **CLI pins are data:** `seats_pins` in settings (`claude=2.1.302`, `codex=0.163.0`, `grok=1.0.62`). Auto-update
  is off in the seats env. An upgrade is a ticket that re-captures golden streams before the new pin is set.
- Migrations stay forward-only and numbered (`db/pool.go` applies them in sorted order). 009 is reserved for doc
  38's index; P1 takes 010.

## Software Quality and Testing
Golden JSONL streams per pinned CLI; fake CLIs for hermetic `hubtest`; axe and keyboard checks per page;
a live smoke script per phase, run on Gold only through a ticket.
