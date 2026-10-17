# agent-hub spec (v0.1, box prototype)
One shared source of truth where AI agents (Claude Code, Codex, Cursor, Grok/Clyffy) and humans sync tasks, claim work under leases, leave notes and hand off. Design source: `shippin-cloud/docs/agent-hub/research.md` (ai-crew-sync leases + SKIP LOCKED, beads ready-queue + same-tx journal, Kleppmann fencing, AIP-154 etags, Stripe idempotency).

## What each piece touches
| Piece | Runs as | Touches |
|---|---|---|
| `scripts/pg-up.sh` / `pg-down.sh` | box user | A private Postgres cluster in `.run/pg` on 127.0.0.1:54329, password in `.run/secrets/` (0600). No system cluster, no systemd. |
| `hub-server` (Go) | box user | Its Postgres DB only (schema `public` of `agenthub`). Listens on `HUB_LISTEN` (loopback by default). Writes nothing on disk except `admin token create --out <file>`. |
| `/mcp` | inside hub-server | Same DB, through the same store functions as REST. Bearer token per agent. |
| `hub` CLI | any agent shell | HTTP calls to `HUB_URL` with `HUB_TOKEN` (env or `HUB_TOKEN_FILE`). Nothing local. |
| `web/` (React Aria SPA) | browser | Built into `server/internal/httpapi/static`, served by hub-server. Talks to `/api/v1` + SSE. |
| `tests/` | box only | `agenthub_test` DB (reset per run), loopback ports 18080-18090, `test-evidence/`. |
Nothing here touches Gold, clyffy-01, workstations, agent configs, DNS or Caddy.

## Data vs code
**Data (editable in the UI, seeded from presets on first boot only):** principals (agents + humans), principal kinds, roles, policies, workflows (states, transitions, lanes), projects, settings (lease TTL, max TTL, heartbeat hint, max expiries, default WIP limit, idempotency TTL, reaper interval, note kinds).
**Code (safety rules only):** identity comes from the credential; tokens stored as SHA-256 only; fencing + version checks; lease clock = DB clock; event log append-only (DB triggers) and hash-chained; admin writes and force-release need a *human* principal **and** a policy allow; nobody edits their own roles; deny-by-default policy evaluation; dev auth only on loopback.

## Data model (Postgres)
- `principals(id text pk, display_name, kind text, is_human bool, login text unique null, roles text[], enabled, wip_limit int null, last_seen_at, version, created_at)`; `login` = external human identity (tailnet login / proxy header).
- `tokens(id uuid, principal_id, name, hash bytea unique, prefix text, created_at, last_used_at, expires_at, revoked_at)`; plaintext `ahk_…` shown once.
- `roles(id text pk, description, version)`.
- `policies(id uuid, name, subject text, action text, selector jsonb, effect allow|deny, priority int, enabled, version)`. `subject`: `*`, `role:<id>`, `principal:<id>`, `kind:<kind>`, `human`, `agent`. `action` glob, e.g. `task.claim`, `task.transition:review->done`, `admin.*`. `selector`: `{"projects":[…],"labels":[…]}` (all must match; empty = any).
- `workflows(id text pk, name, states jsonb, transitions jsonb, lanes jsonb, attention_state, initial_state, version)`. `states=[{key,label,kind}]`, kind ∈ `queue|active|review|blocked|done|attention` (kinds are what code reasons about; names are data). `lanes=[{queue, active, complete_to, release_to, distinct_from_previous}]`: claiming a task in `queue` moves it to `active`; complete → `complete_to`; release/expiry/handoff → `release_to`. `distinct_from_previous=true` = the claimer must differ from whoever completed the previous lane (reviewer ≠ author, as data).
- `projects(id text pk, name, workflow_id, version)`.
- `settings(key text pk, value jsonb, version)`.
- `tasks(id bigserial, project, title, description, state, priority int (lower = sooner), labels text[], assignee null, holder null, lease_id uuid null, lease_expires_at null, heartbeat_at null, fence bigint, version bigint, expiry_count int, last_worker null, progress jsonb, created_by, created_at, updated_at)`.
- `task_deps(task_id, depends_on)` — a task is *ready* only when every dependency is in a `done`-kind state.
- `notes(id bigserial, task_id, author, kind, body, to_principal null, fence null, created_at)` — append-only (trigger).
- `idempotency_keys(principal_id, key, op, request_hash, response jsonb, created_at, pk(principal_id,key))`.
- `events(seq bigint pk, at timestamptz, actor, action, task_id null, data jsonb, fence null, request_id null, prev_hash, hash)` — append-only (trigger blocks UPDATE/DELETE/TRUNCATE).

## Claim / lease protocol
1. `claim(task)` / `claim_next(filter)`: one transaction. `claim_next` selects up to 25 candidates `… FOR UPDATE SKIP LOCKED` (state is a lane `queue`, or an `active` state whose lease expired; deps done; `assignee` null or me), evaluates policy in order, takes the first allowed. Checks WIP (`principal.wip_limit ?? settings.wip_limit_default`). Sets `holder`, new `lease_id`, `lease_expires_at = now() + ttl` (ttl from caller, clamped to `lease_max_ttl_seconds`, default `lease_ttl_seconds`), `fence = nextval('fence_seq')` (global, strictly increasing), `state = lane.active`, `version+1`. Re-claiming a task you already hold returns the current lease (idempotent).
2. `heartbeat(task, fence, progress?)`: holder + fence must match and the lease must be unexpired → extends by the lease's ttl. Recommended interval: `heartbeat_hint_seconds` (TTL/3).
3. Every mutation of a held task (`update`, `heartbeat`, `release`, `complete`, `handoff`, `add_note` with a fence) must carry the current `fence`. Stale or missing fence → **409 `stale_fence`**. Holder mismatch → **409 `not_holder`**. Expired → **409 `lease_expired`**.
4. Reaper (every `reaper_interval_seconds`, and lazily inside claim): expired leases → `release_to` state, `holder=null`, `expiry_count+1`, event `lease.expired`; at `max_lease_expiries` → workflow `attention_state`. The fence is never reused, so a woken-up old holder is rejected.
5. `release(task, fence, reason)` → `release_to`. `complete(task, fence, summary)` → `complete_to`, `last_worker = me`. `handoff(task, fence, note, to?)` = note(kind handoff) + release + `assignee = to` in one tx.
6. `force_release(task, reason)`: human principal (code rule) + policy `task.force_release` allow; reason ≥ 3 chars; event records previous holder and fence.
7. Optimistic concurrency: `update` and every admin write need `version` (REST `If-Match: "<version>"` or body `version`). Mismatch → **409 `version_conflict`** with the current row.
8. Idempotency: any write may carry `Idempotency-Key` (MCP arg `request_id`). Key is per principal; same key + same request → stored response replayed (header `Idempotent-Replayed: true`); same key + different request → **422 `idempotency_mismatch`**. Stored for `idempotency_ttl_hours`. Only successes are stored.

## Event log
Every mutation appends `events` **in the same transaction**. The writer takes `pg_advisory_xact_lock`, reads the last `(seq, hash)`, and stores `hash = sha256(prev_hash ‖ canonical_json{seq,at,actor,action,task_id,data,fence,request_id})`. `GET /api/v1/events/verify` recomputes the chain and reports the first broken seq. After commit `NOTIFY hub_events, '<seq>'`; the broker (`LISTEN`) fans out to SSE. Actions: `task.created|updated|claimed|heartbeat|released|completed|handoff|force_released|lease.expired|note.added|dep.added|admin.<kind>.<op>|token.created|token.revoked`.

## Auth
- **Agents:** `Authorization: Bearer ahk_…` (per-agent, SHA-256 at rest, revocable, optional expiry). Identity only from the token, never from arguments.
- **Humans (UI)** via `HUB_HUMAN_AUTH`: `tailscale-whois` (tagged nodes refused), `proxy-header` (IdP proxy such as oauth2-proxy in front of Zitadel; shared secret header required), `token` (human principal token, pasted once, kept in sessionStorage), `dev` (loopback only, refuses to start otherwise). The external login maps to `principals.login`; unknown logins get 403 (no auto-provisioning). Browser POST/PATCH/DELETE without a bearer token need `X-Hub-Client: ui` (CSRF guard).
- Bootstrap: `hub-server admin seed` (presets) and `hub-server admin token create --principal <id> --out <file>` (0600 file, never printed).

## REST (`/api/v1`, JSON; errors `{error:{code,message,current?}}`)
`GET /me` · `GET /tasks?state&project&holder&label&q&ready` · `POST /tasks` · `GET /tasks/{id}` (task, notes, deps, events) · `PATCH /tasks/{id}` · `POST /tasks/{id}/{claim|heartbeat|release|complete|handoff|notes|deps|force-release}` · `POST /claim-next` · `GET /events?after_seq&task_id&limit` · `GET /events/verify` · `GET /stream` (SSE, `Last-Event-ID`/`after_seq` resume) · `GET|POST /principals`, `PATCH /principals/{id}`, `GET|POST /principals/{id}/tokens`, `DELETE /tokens/{id}` · `GET|PUT|DELETE /{roles|policies|workflows|projects}[/{id}]` · `GET /settings`, `PUT /settings/{key}` · `GET /protocol` (AGENTS.md snippet).

## MCP (`POST /mcp`, Streamable HTTP, stateless, bearer per agent)
Server `instructions` (first 512 chars self-contained): claim → heartbeat → note → complete/handoff loop. Tools (all writes accept `request_id`): `whoami`, `list_ready`, `list_tasks`, `get_task`, `search`, `create_task`, `claim`, `claim_next`, `heartbeat`, `update`, `add_note`, `handoff`, `release`, `complete`, `wait_for_updates` (long-poll ≤25 s on the event feed). Tool errors return `isError` with the same codes as REST (`stale_fence`, `version_conflict`, `policy_denied`, …).

## CLI `hub`
`hub whoami|ready|ls|show <id>|new <title>|claim [id]|beat <id>|note <id> <text>|done <id> <summary>|handoff <id> <note> [--to x]|release <id>|search <q>|events|verify`. It caches `{id,fence,version}` of claims in `$HUB_STATE` (default `~/.cache/hub/claims.json`) so agents don't have to copy fences by hand.

## UI pages (React Aria Components, own CSS, mobile-first)
**Board** (workflow columns on ≥1024px, state tabs + card list on phones; holder badge, live lease countdown, priority, labels; filter by project/search) · **Task detail** (fields, deps, notes + handoff thread, add note, claim/release, force release in an alertdialog with mandatory reason, per-task events) · **Agents** (presence from last_seen, current claims, WIP, tokens create/revoke, connect snippets) · **Settings**: Roles, Policies (subject/action/selector/effect/priority), Workflows (states, transitions, lanes; JSON validated), Settings (lease defaults, WIP) · **Audit** (live SSE timeline, filters, hash-chain verify badge).
