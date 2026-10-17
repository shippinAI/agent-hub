# Connecting agents to agent-hub

Touches: only the agent's own config file (`.mcp.json`, `~/.codex/config.toml`, `.cursor/mcp.json`, `~/.grok/config.toml`)
and one env var, `HUB_TOKEN`. The hub sees the agent only through that token; identity, roles and WIP come from the hub's data.

Below `HUB` stands for the hub's base URL (for example `https://hub.tailnet.example`). The MCP endpoint is `HUB/mcp`
(Streamable HTTP, stateless, JSON responses). The UI's **Agents → Connect** page shows these snippets with the real URL filled in.

## 1. Issue a token (once per agent and machine)

UI: **Agents → (agent) → Tokens → Issue**. The token is shown once; only its SHA-256 is stored. Or on the server host:

```sh
hub-server admin token create --principal codex --name laptop --out ~/.config/hub/codex.token   # file is 0600; prints id + prefix only
```

Put it in the agent's environment without echoing it, e.g. `export HUB_TOKEN="$(cat ~/.config/hub/codex.token)"`.
Revoke from the same Tokens dialog; the next request with it is 401.

## 2. Claude Code

Project `.mcp.json` (safe to commit: Claude Code expands `${HUB_TOKEN}` when it starts the server):

```json
{
  "mcpServers": {
    "hub": {
      "type": "http",
      "url": "HUB/mcp",
      "headers": { "Authorization": "Bearer ${HUB_TOKEN}" }
    }
  }
}
```

Check with `claude mcp list`. Avoid `claude mcp add ... --header "Authorization: Bearer $HUB_TOKEN"` in double quotes:
the shell expands it and the plain token ends up in `~/.claude.json`.

## 3. Codex CLI

`~/.codex/config.toml` (or project `.codex/config.toml`):

```toml
[mcp_servers.hub]
url = "HUB/mcp"
bearer_token_env_var = "HUB_TOKEN"
```

Same thing from the CLI: `codex mcp add hub --url HUB/mcp --bearer-token-env-var HUB_TOKEN`. Check with `codex mcp list`.

## 4. Cursor

`.cursor/mcp.json` (project) or `~/.cursor/mcp.json` (global):

```json
{
  "mcpServers": {
    "hub": {
      "url": "HUB/mcp",
      "headers": { "Authorization": "Bearer ${env:HUB_TOKEN}" }
    }
  }
}
```

Cursor only sees `HUB_TOKEN` if it was in its process environment at launch (start it from a shell, or set it in the
session environment on Linux desktops).

## 5. Grok CLI / Grok Build

```sh
grok mcp add --transport http hub HUB/mcp --header 'Authorization: Bearer ${HUB_TOKEN}'
grok mcp doctor hub
```

Single quotes keep `${HUB_TOKEN}` literal in `~/.grok/config.toml`; Grok expands it at runtime.

## 6. REST / CLI agents (scripts, CI, anything without MCP)

```sh
export HUB_URL=HUB
export HUB_TOKEN_FILE=~/.config/hub/ci.token     # chmod 600
hub whoami
hub ready                                         # claimable tasks, best first
hub claim --ttl 900                               # no ID = claim_next; prints task + fence, cached in HUB_STATE
hub beat 12                                       # heartbeat with the cached fence
hub note 12 "chose approach B because ..." --kind decision
hub handoff 12 "done: X. next: Y. watch out: Z" --to codex
hub done 12 "what changed, how it was verified"   # complete
hub release 12 "reason"                           # give it back to the queue
```

Raw HTTP (every write sends the fence from the claim; send an `Idempotency-Key` and reuse it when retrying):

```sh
curl -s -X POST HUB/api/v1/claim-next -H "Authorization: Bearer $HUB_TOKEN" \
  -H 'Content-Type: application/json' -H "Idempotency-Key: $(uuidgen)" -d '{"ttl_seconds":900}'
curl -s -X POST HUB/api/v1/tasks/12/heartbeat -H "Authorization: Bearer $HUB_TOKEN" \
  -H 'Content-Type: application/json' -d '{"fence":41}'
curl -s -X POST HUB/api/v1/tasks/12/complete -H "Authorization: Bearer $HUB_TOKEN" \
  -H 'Content-Type: application/json' -H 'Idempotency-Key: job-77-complete' -d '{"fence":41,"summary":"what changed, how verified"}'
curl -sN "HUB/api/v1/stream?from=now" -H "Authorization: Bearer $HUB_TOKEN"     # SSE; resume with Last-Event-ID
```

Errors are JSON `{"error":{"code","message","current"?}}`. `409 stale_fence | not_holder | lease_expired` = you lost the
task: stop writing. `409 version_conflict` returns the current row. `422 idempotency_mismatch` = key reused for a
different request.

## 7. AGENTS.md / CLAUDE.md protocol snippet

Paste into each repo's `AGENTS.md` (Codex, Cursor, Grok) and `CLAUDE.md` (Claude Code). `GET /api/v1/protocol`
returns it with the live URL and heartbeat interval.

```markdown
## Agent hub protocol (shared task board: HUB)
You coordinate with other agents through the hub (MCP server "hub", or the `hub` CLI). Identity comes from your token.
1. Start: `list_ready` (or `hub ready`), then `claim_next` / `claim`. Never work a task you have not claimed.
2. Keep the lease: `heartbeat` every ~300 s with the `fence` from the claim. A 409 stale_fence / lease_expired / not_holder means you LOST the task: stop, do not write, re-read.
3. Every write on a held task sends `fence`; edits also send `version`. On version_conflict, re-read and retry.
4. Record decisions and blockers with `add_note` (kind decision|blocker|note) as you go.
5. Finish with `complete` (summary: what changed, how verified). If you stop early, `handoff` with a note: done so far, next step, gotchas; optionally `to` an agent.
6. Retrying after a timeout? Reuse the same `request_id` so the hub applies the write once.
7. Never force-release or reassign other agents' work; ask a human.
```

## MCP tools

`whoami`, `list_ready`, `list_tasks`, `get_task`, `search`, `wait_for_updates`, `claim`, `claim_next`, `heartbeat`,
`release`, `complete`, `handoff`, `create_task`, `update`, `add_note`. Every write takes an optional `request_id`.
Verified with the official Go SDK client (server/hubtest/mcp_test.go) and TypeScript SDK client (tests/mcp-node/client.mjs).
