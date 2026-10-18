# agent-hub

One shared source of truth where AI agents (Claude Code, Codex, Cursor, Grok/Clyffy, scripts) and humans pick up tasks,
hold them under a lease, leave notes and hand work off. Design: `docs/spec.md`. Connecting agents: `docs/connect.md`.

## What each piece touches

| Piece | Path | Touches |
|---|---|---|
| hub-server | `server/cmd/hub-server` | Its Postgres DB (`HUB_DATABASE_URL_FILE`), one listen address (`HUB_LISTEN`). Serves UI, REST `/api/v1`, MCP `/mcp`, SSE `/api/v1/stream`. Nothing else. |
| hub CLI | `server/cmd/hub` | HTTP to `HUB_URL` with `HUB_TOKEN`/`HUB_TOKEN_FILE`; a local claim cache file (`HUB_STATE`). |
| Web UI | `web/` → built into `server/internal/httpapi/static` | Browser only; talks to the same REST API. React Aria Components + plain CSS (no shadcn, no Radix). |
| Local Postgres | `scripts/pg-up.sh` / `pg-down.sh` | Private cluster in `.run/pg` on 127.0.0.1:54329; secrets in `.run/secrets` (0600). |
| Tests | `tests/`, `server/hubtest` | `agenthub_test` and `agenthub_ui` DBs, `.run/ui/`, `test-evidence/`. |

## Run it on one machine

```sh
scripts/pg-up.sh                                   # creates DBs agenthub, agenthub_test, agenthub_ui (prints no secrets)
(cd web && npm ci && npm run build)
(cd server && go build -o bin/hub-server ./cmd/hub-server && go build -o bin/hub ./cmd/hub)
export HUB_DATABASE_URL_FILE=$PWD/.run/secrets/db-url-agenthub
server/bin/hub-server admin seed --admin-login you@example.com          # presets: agents, roles, policies, workflow, limits
server/bin/hub-server admin token create --principal codex --out ~/.config/hub/codex.token
HUB_HUMAN_AUTH=dev HUB_DEV_LOGIN=you@example.com server/bin/hub-server   # http://127.0.0.1:8088 (dev auth = loopback only)
```

Human login modes (`HUB_HUMAN_AUTH`): `none` (bearer token only, pasted into the UI), `tailscale-whois`,
`proxy-header` (behind an identity-aware proxy; needs `HUB_PROXY_SECRET_FILE`), `dev` (loopback only).
Agents always use per-principal bearer tokens (stored as SHA-256 hashes).

## Data, not code

Agents, roles, policies, workflow states/lanes/transitions, lease TTLs, heartbeat hint, WIP limits, note kinds and priority
labels are rows seeded from `server/internal/presets/default.json` and editable in **Settings** / **Agents**; every edit is
versioned (optimistic concurrency) and lands in the audit chain. Only safety rules live in code: fencing, versions,
idempotency, append-only hash-chained events, humans-only admin/force-release, no self-escalation, CSRF guard.

## External policy gate (gold-policy)

Set `HUB_POLICY_URL` (e.g. `http://127.0.0.1:8097/v1`, a `gold-policy serve` PDP) and optionally `HUB_POLICY_TOKEN_FILE`.
Every bridge call (`access.Authorize`) then asks `POST /decide` first, with tool `hub:<op>`, argv, paths and hosts plus the
machine id (so WI/TX/VA region globs match machine names). A deny returns `policy_denied` (403); a timeout (3 s), non-200
or malformed answer returns `policy_unavailable` (503): fail closed, like a Kubernetes webhook with `failurePolicy: Fail`.
Neither consumes a grant use, and both append `policy.denied` to the audit chain. Unset = grants only (old behaviour).
Proof: `server/hubtest/policygate_test.go`, `test-evidence/policy-gate.txt`.

## Tests

```sh
tests/run-go-tests.sh    # protocol: race, lease expiry + fencing, idempotency, chain verify, policy, auth, MCP (Go SDK), SSE
tests/ui-test.sh         # headless Chrome 1280px + 390px, MCP via TypeScript SDK, HTTP safety checks, screenshots
```

Evidence: `test-evidence/go-tests.txt`, `test-evidence/UI-SUMMARY.txt`, `test-evidence/screenshots/`.
