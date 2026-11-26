# Conformance suite

`server/conformance` is a black-box test of the agent-hub protocol in `docs/spec.md`. It talks to a running hub
only over REST (`/api/v1`) and MCP (`/mcp`), so any build of the hub (the Go server today, a Rust twin later)
must pass it unchanged before it counts as agent-hub.

## Run it against any hub

The hub needs three principals with the default policies: a human with admin rights and two agents.

```sh
cat > tokens.json <<'JSON'
{"human": "<human admin token>", "agent_a": "<agent token>", "agent_b": "<another agent token>"}
JSON
cd server
HUB_CONFORMANCE_URL=http://127.0.0.1:8080 HUB_CONFORMANCE_TOKENS=../tokens.json go test -count=1 -v ./conformance
```

Every case creates a project of its own (`conf-…`) and force-releases what it held when it ends, so the suite can
run against a hub that already holds other work. It adds projects, tasks, runs and usage rows; run it against a
test hub, not production.

The Go server runs the same suite in `server/hubtest/conformance_test.go`, both in process and through this
external entry point.

## What it checks

| Case | Contract |
| --- | --- |
| errors | `{error:{code,message}}` with 401 `unauthenticated`, 404 `not_found`, 400 `invalid` |
| lease | one holder; re-claim returns the same lease; `held_by_other`; heartbeat; release; fences grow |
| fencing | only humans force-release; the old fence is dead for heartbeat, complete and release |
| claim_next | assigned work first, then priority; blocked and held tasks skipped |
| idempotency | same key and body replays (`Idempotent-Replayed: true`); other body 422 `idempotency_mismatch`; keys per principal |
| versions | stale version 409 `version_conflict`; `If-Match` works |
| review_lane | complete moves build to review; the builder cannot review its own work (`distinct_required`) |
| evidence | stamped with its fence; stale fence refused; rows kept per lease |
| runs | run under the lease; append-only, retry-safe chunks (`duplicate`, `chunk_conflict`); own principal only; sha256 on finish; `run_finished` |
| usage | `external_ref` dedupe; cache reads are part of input; per-task totals |
| events | writes appear in the task's events; `/events/verify` is ok |
| mcp | claim, conflicting claim is an error, get_task, complete |

## Not covered yet

Lease expiry (the shortest lease is 30 s), SSE and `wait_for_updates`, the GitHub webhook, budgets and routing
(they change hub-wide settings), the evidence gate on a lane (it needs a workflow change), and the admin and UI
surfaces. These stay in `server/hubtest`, which also checks the database directly.
