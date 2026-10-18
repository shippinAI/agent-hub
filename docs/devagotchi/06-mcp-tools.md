# 6. MCP tools and REST additions
[Index](../devagotchi-spec.md) · Same `/mcp` endpoint, same bearer token, same error envelope and `request_id` idempotency as [spec.md](../spec.md#mcp-post-mcp-streamable-http-stateless-bearer-per-agent). Existing tools are unchanged. Server `instructions` gain one line: "Machine and GUI tools need a grant: call request_access, explain why, wait, then use only the granted scope. For setup tasks, find a guide with guides_search and follow it as the subject-matter expert."

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
| `gui_session_start` / `gui_session_end` | machine, monitors?, ttl_s | session id + fence, display size and scale | gui_sessions, events |
| `gui_screenshot` | session, region?, max_edge? | image (MCP image content) + sha256; never stored unless retention says so | gui_artifacts (hash row), events |
| `gui_input` | session, fence, actions[] (move, click, drag, scroll, type, key) | per-action ok + target app/window | events |
| `gui_tree` / `gui_window` / `gui_app` | session, window?/op/app id | accessibility tree / window list, focus, move / app launch, quit | events |
| `guides_search` / `guide_get` | query, os?, tags? / guide id, version? | matching guides / the published version's steps for this machine | – |
| `guide_run_start` / `guide_run_step` | guide id, machine, task_id? / run id, step id, outcome, note? | run id + first step / next step (branches resolved) | guide_runs, events |
| `role_route` | role, chat_app? | which subscription/worker would take it now (no secrets) | – |
Each machine tool's schema documents its limits (timeouts, caps) in its description, so no hidden caps.

## MCP resources and prompts
Published guides are also exposed as MCP **resources** (`guide://<id>@<version>`, with `listChanged` when a new version publishes) and each guide as an MCP **prompt** the operator can pick in their client ("Set up Docker on my laptop"). Resources are application-driven and prompts user-controlled in MCP ([resources](https://modelcontextprotocol.io/specification/2025-06-18/server/resources), [prompts](https://modelcontextprotocol.io/specification/2025-06-18/server/prompts)); the `guide_*` tools exist because not every client surfaces either (**inference**).

## Not exposed over MCP (human-only, REST + UI)
Approving, narrowing, denying, or revoking grants; enabling the unsandboxed policy; editing grant presets or the default preset; acknowledging resume alerts; editing, publishing, or importing guides; editing capabilities/risks/shell profiles; enrolling machines; adding or rotating secrets; creating subscriptions and role bindings; tenant and plan settings. The policy subject `agent` is denied `grant.*`, `preset.*`, `policy.unsandboxed.*`, `guide.publish`, `guide.edit`, `secret.*`, `machine.enroll` in code regardless of policy rows.

## REST (`/api/v1`)
- `GET|POST /machines`, `PATCH /machines/{id}`, `POST /machines/{id}/{enroll-code|test|pause|resume}`, `DELETE /machines/{id}` (revokes token + grants).
- `GET /access-requests?status=pending`, `POST /access-requests/{id}/{approve|deny}` (approve body may carry a narrower `selector`, `sandbox_level`, `duration`, `uses_max`; the server rejects widening with `422 scope_wider_than_request`).
- `GET /grants?live=true&machine=`, `POST /grants/{id}/revoke`, `POST /machines/{id}/revoke-all`.
- `GET /proc-leases`, `POST /proc-leases/{id}/kill`.
- `GET|POST /grant-presets`, `PATCH|DELETE /grant-presets/{id}`, `PUT /settings/grant_default_preset` (full-access presets need body `confirm` = preset id and a live unsandboxed policy).
- `GET|PUT /sandbox-policies[/{machine}]` (enable/disable; body carries `acknowledged_risk_ids`, must cover every risk row of `unsandboxed`).
- `GET /resume-alerts` (alerts due for this UI session), `POST /resume-alerts/{id}/{continue|revoke|pause}`.
- `GET /gui-sessions?live=true`, `POST /gui-sessions/{id}/end`, `GET /gui-artifacts/{id}` (only while retained; human principals only).
- `GET|POST /guides`, `GET|PATCH /guides/{id}`, `GET|POST /guides/{id}/versions`, `POST /guides/{id}/versions/{v}/{publish|retire}`, `GET /guides/{id}/diff?from=&to=`, `POST /guides/import` (signed bundle), `GET /guide-updates` (pending Shippin versions), `POST /guide-updates/{id}/{accept|skip}`, `GET /guide-runs?machine=&guide=`.
- `GET|PUT|DELETE /{capabilities|risks|shell-profiles|providers|router-endpoints|chat-apps}[/{id}]`.
- `GET|POST /subscriptions`, `PATCH|DELETE /subscriptions/{id}`, `PUT /subscriptions/{id}/secret` (write-only body; the response is `{fp}` only), `POST /subscriptions/{id}/test` (a one-token router call; returns ok/error, never the key).
- `GET|POST /role-bindings`, `PATCH|DELETE /role-bindings/{id}`.
- `GET /tenants/me` (cloud), plan limits.
- SSE `/stream` adds the event families `access.*`, `grant.*`, `machine.*`, `proc.*`, `gui.*`, `guide.*`, `resume.*`, `policy.*`, `subscription.*`.

## Bridge endpoints (machine principals only)
`GET /bridge/v1` (WSS upgrade) · `POST /bridge/v1/poll` · `POST /bridge/v1/result` · `POST /bridge/v1/enroll` (code → token, rate-limited, 5 tries per code) · `GET /bridge/v1/grant/{id}` (state check for the bridge cache).

## CLI `hub` additions
`hub machines` · `hub ask <machine> <capability> --path … --why "…"` · `hub grants` · `hub exec <machine> -- <argv…>` · `hub revoke <grant>` (only works with a human token) · `hub gui stop <machine>` · `hub guides` · `hub guide run <id> <machine>`.
