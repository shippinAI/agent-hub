# 39d. P1: the chat page (Claude Code, Codex and Grok in one room)

[Plan](39-chatmaton-gold-build-plan.md) · Architecture: [39c](39c-architecture-on-gold.md). Closes the
headless and app-server part of G-21 and the rooms part of G-16 ([27](27-gap-tracking.md)).
**Done when:** on Gold, `#/chat` has one room with three seats (Claude Code 2.1.302, Codex 0.163.0, Grok 1.0.62).
One prompt to "All" streams three answers live to two open browser tabs. A reload mid-turn loses nothing that
was persisted. Each answer shows tokens and cost basis, and every turn has an action-feed row.

## Requirements (scope)
- **In:** rooms, seats, messages, turns; modes `parallel` (everyone answers), `round_robin`, `mention` (`@codex`);
  WebSocket streaming; cancel a turn; per-turn usage; seat health (installed, version, signed in).
- **Out (later):** ACP runtime (Grok's preferred adapter, P1.x), roles and routines ([24](24-chatmaton-spec.md)),
  write profiles (P2), cost ledger (P3), and MCP room tools (P1.x).

## How each CLI is driven (facts, then the row change)
| Seat | Mode used | Stream facts | Session |
|---|---|---|---|
| Claude Code | `claude -p --input-format stream-json --output-format stream-json --verbose --include-partial-messages`, one long-lived process, user turns as NDJSON on stdin ([headless](https://code.claude.com/docs/en/headless), [CLI ref](https://code.claude.com/docs/en/cli-reference)) | `stream_event` with `event.delta.type=text_delta`; the final `result` carries text, cost and session | `--resume <session_id>` after a restart |
| Codex | `codex app-server` JSON-RPC over stdio: `initialize` → `initialized` → `thread/start`/`thread/resume` → `turn/start`; `turn/interrupt` to cancel ([app-server](https://learn.chatgpt.com/docs/app-server), [README](https://github.com/openai/codex/blob/main/codex-rs/app-server/README.md)) | `item/agentMessage/delta`, `item/started`, `item/completed`; `turn/completed` carries token usage | `threadId` |
| Grok | `grok --no-auto-update --output-format streaming-json -p <prompt>` (row fallback `headless_json`) ([headless](https://docs.x.ai/build/cli/headless-scripting), [user guide](https://github.com/xai-org/grok-build/blob/main/crates/codegen/xai-grok-pager/docs/user-guide/14-headless-mode.md)) | Typed JSONL: `text`, `thought`, `tool_call`, `usage`, `end` (with `sessionId`, usage) | `-r <sessionId>` |
**Row edits (data only, `presets/devagotchi/provider-rows.json`):** Claude `output` gains `--include-partial-messages`
and `input: stdin_jsonl`. Grok's fallback `output` changes `json` → `streaming-json`, `framing: jsonl`, and adds
`--no-auto-update`. Exact paths are confirmed against golden captures from the pinned versions (ticket
GC-CM-P1-02) before merge. Grok seats use the fallback through a seat-level `adapter_override` until ACP lands.

## Construction: files to add (each ≤150 lines, no barrels)
**Migration:** `server/internal/db/migrations/010_chat_rooms.sql` (009 is reserved for doc 38's index).
- `room_modes(id text pk, label, turn_policy jsonb, termination jsonb, version)`. Seeds: `parallel`,
  `round_robin`, `mention`.
- `rooms(id uuid pk, tenant_id text fk tenants, name, mode_id fk, settings jsonb, created_by fk principals,
  created_at, archived_at, version)`.
- `room_seats(room_id fk, seat text ~ '^[a-z][a-z0-9_-]{0,23}$', provider_id fk providers, machine_id fk machines,
  adapter_override jsonb null, model text null, profile text default 'read_only', position int,
  session_ref text null, last_seen_msg bigint default 0, version, PK(room_id, seat))`.
- `room_messages(id bigserial pk, room_id, seat text null, principal_id text null, role user|assistant|system|error,
  content text, items jsonb, turn_id uuid null, content_sha256 text, created_at)`. Index on `(room_id, id)`.
- `room_turns(id uuid pk, room_id, seat, trigger_msg bigint, status queued|running|done|error|cancelled,
  lease_id uuid null, usage jsonb, cost_usd numeric null, cost_basis text, error text, started_at, ended_at)`.
- `INSERT INTO audit_categories ('seat', 14 days, capture_output=false)` for seat I/O rows.
  Settings rows: `chat.context_template`, `chat.max_parallel_seats`=3, `chat.turn_timeout_ms` (default
  `providers.DefaultTurnTimeoutMS`), `chat.delta_flush_ms`=50, `seats_pins`.
**`server/internal/rooms/`:** `types.go` · `service.go` (create, list, get, archive) · `seats.go` (add, update,
remove; checks the provider row is enabled and its adapter runnable) · `messages.go` (post and list; event
`room.message` holds the sha256, never the content) · `turns.go` (queue, transition, cancel) · `modes.go` (turn
policy from the `room_modes` row) · `context.go` (per-seat delta since `last_seen_msg`, wrapped by the template) ·
`scheduler.go` (runs queued turns, max parallel, timeout; persists results).
**`server/internal/seats/`:** `event.go` (normalized `Event`) · `runtime.go` (`Runtime` interface; picks the adapter
from row, fallbacks and override) · `procio.go` (wraps `machineops.ProcStart`/`ProcOp`: write stdin, read loop on
`after_seq`, heartbeat at ttl/3, kill; splits lines) · `jsonpath.go` (evaluates the `$.a.b[0]` paths allowed by
`providers.jsonPathRe`) · `headless.go` (per-turn and persistent-stdin modes) · `headless_map.go` (`event_map`,
`delta_path` → `Event`) · `codex.go` (JSON-RPC client; answers server approval requests from the profile:
`read_only` = decline) · `codex_map.go` · `profiles.go` · `health.go` (runs the row's `detect.version_cmd` and
`signed_in_cmd` on `gold-seats` through the bridge; never reads token files).
**`server/internal/chatws/`:** `ticket.go` (single-use, 60 s, bound to principal and room) · `socket.go`
(`coder/websocket` accept with Origin check, read/write pumps, ping 20 s) · `fanout.go` (per-room subscribers;
deltas coalesced every `chat.delta_flush_ms`) · `frames.go`.
**Wiring:** `server/internal/devapi/room_routes.go` (new); `devapi/build.go` (construct rooms, seats, chatws;
74 lines today, stays ≤150); `server/internal/actionfeed/category_ctx.go` (new: `WithCategory(ctx,"seat")`, read
in `machineops/feed.go`, so seat reads stay out of `tool`). One `seat.turn` feed event per turn carries
provider, model, tokens, status and a `purpose_plain` line.
**Providers (schema stays v1, keys added as optional):** split `providers/adapters.go` (148 lines) into
`adapters.go` and `adapters_headless.go`; `HeadlessJSONConfig` gains `input` (`argv`|`stdin_jsonl`), `delta_path`,
`done_type`, `profiles map[string][]string`; `CodexAppServerConfig` gains `profiles map[string]json.RawMessage`
(thread/start params such as sandbox and approval policy); `enums.go` adds `HeadlessInputs`; mirror in
`web/src/lib/providerTypes.ts` (kept in step by `enums_ts_test.go`). ADR 0003 records the additive keys.
**Bridge (small, backward compatible):** `bridge/runner/proc.go` adds `split_stderr`; `bridge/procs/start_unix.go`
and `start_windows.go` give stderr its own buffer when it is set. Today stdout and stderr share one writer
(`start_unix.go:58`), which could break a JSONL line in two.
**Presets:** `room-modes.json`; `grant-presets.json` adds `chat-seats` (`proc.interactive` on `gold-seats`, argv[0]
in `claude|codex|grok`, `sandbox_level` from the profile); `settings.json` adds the `chat.*` keys.
**Web (React Aria only):** `web/src/pages/Chat.tsx` · `pages/chat/RoomList.tsx` (`GridList`) ·
`pages/chat/NewRoomModal.tsx` (`Modal`, `TextField`, `CheckboxGroup` of providers with health, `Select` mode) ·
`pages/chat/RoomView.tsx` · `pages/chat/MessageList.tsx` (`<ol>` plus one polite `role=status` region, as in doc 38)
· `pages/chat/MessageItem.tsx` (`Disclosure` for tool and reasoning items) · `pages/chat/Composer.tsx` (`TextArea`,
`ToggleButtonGroup` for All or one seat, send and cancel `Button`s) · `pages/chat/SeatBar.tsx` ·
`pages/chat/TurnUsage.tsx` (`TooltipTrigger` plus a press `Popover` for touch) · `web/src/ui/ProviderChip.tsx` ·
`web/src/lib/chatTypes.ts` · `web/src/lib/roomFrames.ts` (reducer) · `web/src/lib/useRoomSocket.ts` (ticket,
connect, backoff, resume with `after`). Edits: `web/src/App.tsx` (`case 'chat'`), `web/src/pages/Shell.tsx` (nav).

## Endpoints
| Method and path | Purpose |
|---|---|
| `GET/POST /api/v1/rooms` · `GET/PATCH /api/v1/rooms/{id}` | List, create, rename, change mode, archive (`version` checked) |
| `POST /api/v1/rooms/{id}/seats` · `PATCH/DELETE …/seats/{seat}` | Seat a provider (+ machine, model, profile) |
| `GET /api/v1/rooms/{id}/messages?after=&limit=` | Transcript page (resume source) |
| `POST /api/v1/rooms/{id}/messages` | `{content, to:["all"]|[seat…], request_id}` → message plus queued turns |
| `POST /api/v1/rooms/{id}/turns/{turn}/cancel` | Codex `turn/interrupt`; headless: `proc.kill` |
| `POST /api/v1/rooms/{id}/ws-ticket` → `GET /api/v1/rooms/{id}/ws?ticket=` | WebSocket upgrade |
| `GET /api/v1/seats/health?machine=gold-seats` | Per provider: installed, version vs pin, signed in |
**Frames.** Server to client: `hello{head}` `message` `turn{id,seat,status}` `delta{turn,item,text}`
`item{turn,kind,summary}` `usage{turn,…}` `error{turn,code,message}` `snapshot{turn,text}` (on resume). Client to
server: `prompt` (same body as POST) `cancel{turn}` `ping`. Deltas are not stored. The final text is a
`room_messages` row.

## Testing
- **Go unit:** `seats/jsonpath_test.go`; `seats/headless_test.go` and `seats/codex_test.go` replay
  `seats/testdata/{claude-2.1.302,grok-1.0.62,codex-0.163.0}.jsonl` (golden, redacted) through a fake `procio`;
  `rooms/context_test.go` (deltas, wrapping, high-water marks); `rooms/modes_test.go`; `chatws/ticket_test.go`
  (single use, expiry, wrong room, wrong origin); `providers/validate_test.go` cases for the new keys;
  `bridge/procs/procs_test.go` `split_stderr` case.
- **hubtest:** `server/hubtest/rooms_test.go` with the existing fake bridge (`fakebridge_test.go`): 3 seats,
  `parallel` mode, asserts messages, hash-chained `room.message` events, `seat.turn` feed rows, cancel, and resume.
- **UI:** `tests/ui/chat-shots.mjs` (Playwright, the pattern of `actions-shots.mjs`, desktop plus `mobile.mjs`
  viewport, axe, keyboard: composer → send → Escape on popovers); `tests/chat-ui.sh` runner.
- **Live (Gold, through a ticket):** `tests/chat-live.sh` sends one prompt to All and checks 3 answers, usage and
  session refs.

## Gold tickets (drafts; nothing filed)
| Draft id | Change on Gold | Verify |
|---|---|---|
| GC-CM-P1-01 | Seats env: the existing hub container or a sibling Incus container; `seats` user; pinned CLIs; auto-update off (Claude `DISABLE_AUTOUPDATER=1`; Codex and Grok equivalents confirmed from `--help`); persistent HOME volume; bridge enrolled as `gold-seats` | `--version` = pins; bridge Online in `#/dev/machines` |
| GC-CM-P1-02 | Golden capture: one fixed prompt per CLI, output redacted and copied to the box | Fixture sha256s recorded in the ticket |
| GC-CM-P1-03 | Jesse signs in on each CLI with device code (Codex: app-server `chatgptDeviceCode`, [37](37-subscription-sign-in.md); `grok login --device-auth`; Claude `/login`); operator action, no agent sees a credential | `GET /seats/health` = signed in ×3 |
| GC-CM-P1-04 | Deploy: `pg_dump` backup (as `.run/live/backups/providers-pre-007-*.sql`), roll hub-server + bridge, migration 010, approve the `chat-seats` grant | `tests/chat-live.sh` green; `events/verify` ok |
