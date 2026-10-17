# 6. MCP tools and REST additions
[Index](../devagotchi-spec.md) · Same `/mcp` endpoint, same bearer token, same error envelope and `request_id` idempotency as [spec.md](../spec.md#mcp-post-mcp-streamable-http-stateless-bearer-per-agent). Existing tools are unchanged. Server `instructions` gain one line: "Machine tools need a grant: call request_access, explain why, wait, then use only the granted scope."

## New MCP tools (agents)
| Tool | Args (abridged) | Returns | Writes |
|---|---|---|---|
| `machines_list` | `filter?` | id, label, os, connected, online, last_error, my live grants | – |
| `capabilities_list` | – | capability ids, selector schema, sandbox levels, risk titles | – |
| `request_access` | machine, capability, selector, sandbox_level, duration, reason, task_id? | `{request_id, status}` | access_requests |
| `wait_for_access` | request_id, timeout_s≤25 | status + granted scope (may be narrower) | – |
| `my_grants` | machine? | live grants with expiry and uses | – |
| `exec` | machine, argv[], cwd?, env?, timeout_s?, shell_profile?, stdin? | exit_code, stdout, stderr (capped), truncated, duration | events (`grant.used`) |
| `proc_start` / `proc_read` / `proc_write` / `proc_heartbeat` / `proc_kill` | see [03](03-bridge-protocol.md#ops) | lease id + fence / output since seq | proc_leases, events |
| `fs_read` / `fs_list` / `fs_stat` | machine, path, offset?, limit? | content + sha256 | events |
| `fs_write` | machine, path, content, expect_sha256 \| expect_absent | new sha256 | events |
| `fs_edit` | machine, path, expect_sha256, edits[] | new sha256, applied count | events |
| `fs_move` / `fs_delete` | machine, from/to or path, expect_sha256 | ok | events |
| `role_route` | role, chat_app? | which subscription/worker would take it now (no secrets) | – |
Each machine tool's schema documents its limits (timeouts, caps) in its description, so no hidden caps.

## Not exposed over MCP (human-only, REST + UI)
Approving, narrowing, denying, or revoking grants; editing capabilities/risks/shell profiles; enrolling machines; adding or rotating secrets; creating subscriptions and role bindings; tenant and plan settings. The policy subject `agent` is denied `grant.*`, `secret.*`, `machine.enroll` in code regardless of policy rows.

## REST (`/api/v1`)
- `GET|POST /machines`, `PATCH /machines/{id}`, `POST /machines/{id}/{enroll-code|test|pause|resume}`, `DELETE /machines/{id}` (revokes token + grants).
- `GET /access-requests?status=pending`, `POST /access-requests/{id}/{approve|deny}` (approve body may carry a narrower `selector`, `sandbox_level`, `duration`, `uses_max`; the server rejects widening with `422 scope_wider_than_request`).
- `GET /grants?live=true&machine=`, `POST /grants/{id}/revoke`, `POST /machines/{id}/revoke-all`.
- `GET /proc-leases`, `POST /proc-leases/{id}/kill`.
- `GET|PUT|DELETE /{capabilities|risks|shell-profiles|providers|router-endpoints|chat-apps}[/{id}]`.
- `GET|POST /subscriptions`, `PATCH|DELETE /subscriptions/{id}`, `PUT /subscriptions/{id}/secret` (write-only body; the response is `{fp}` only), `POST /subscriptions/{id}/test` (a one-token router call; returns ok/error, never the key).
- `GET|POST /role-bindings`, `PATCH|DELETE /role-bindings/{id}`.
- `GET /tenants/me` (cloud), plan limits.
- SSE `/stream` adds the event families `access.*`, `grant.*`, `machine.*`, `proc.*`, `subscription.*`.

## Bridge endpoints (machine principals only)
`GET /bridge/v1` (WSS upgrade) · `POST /bridge/v1/poll` · `POST /bridge/v1/result` · `POST /bridge/v1/enroll` (code → token, rate-limited, 5 tries per code) · `GET /bridge/v1/grant/{id}` (state check for the bridge cache).

## CLI `hub` additions
`hub machines` · `hub ask <machine> <capability> --path … --why "…"` · `hub grants` · `hub exec <machine> -- <argv…>` · `hub revoke <grant>` (only works with a human token).
