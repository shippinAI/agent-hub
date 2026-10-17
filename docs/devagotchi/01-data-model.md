# 1. Data model
[Index](../devagotchi-spec.md) · Extends the tables in [spec.md](../spec.md#data-model-postgres). Every new table has `version bigint` (optimistic concurrency, `If-Match`) and every write appends to `events` in the same transaction (hash chain unchanged).

## Data vs code
**Data (UI-editable, preset-seeded):** machines, shell profiles, capabilities, risks, risk-to-capability links, grant duration options, providers, router endpoints, subscriptions, role bindings, tenants, plan limits, and settings below. Policies gain new action names (`machine.*`, `grant.*`, `exec`, `fs.*`, `subscription.*`); those names are data rows in `capabilities`.
**Code (invariants only):**
1. Only a **human** principal may approve, narrow, or revoke a grant, and never for their own agent token (same rule as "nobody edits their own roles"). No MCP tool writes `grants`.
2. Deny wins: deny policy > grant > nothing (deny-by-default). Order: deny, then ask, then allow ([Claude Code permissions](https://code.claude.com/docs/en/permissions)).
3. The bridge rechecks the grant on every call (it does not cache allow decisions longer than `grant_cache_seconds`, max 30, as a code clamp).
4. Secrets are write-only: stored encrypted, never returned by any API, and logged only as `fp` (first 8 hex of SHA-256).
5. A machine's presence is set only by a completed executed call (see 03).
6. Edits require an expected hash; writes are temp+fsync+rename.

## New tables
- `machines(id text pk, tenant_id, principal_id → principals (kind machine), label, os text, arch, bridge_version, sandbox_kind text, shell_profile_ids text[], enrolled_by, enrolled_at, last_exec_ok_at null, last_exec_err_at null, last_error text null, connected bool, disabled bool, version)`. `connected` = socket up; **online** is derived = `last_exec_ok_at > now() - online_window_seconds`.
- `shell_profiles(id text pk, os, label, argv_prefix jsonb, env_allow text[], cwd_default, pty bool, version)`. Presets: `bash` `["/bin/bash","-lc"]`, `zsh`, `pwsh` `["pwsh","-NoProfile","-Command"]`, `cmd`, `direct` (no shell, argv runs as given; the default). Fixes DC shell drift ([#4](https://github.com/wonderwhy-er/DesktopCommanderMCP/issues/4), [#443](https://github.com/wonderwhy-er/DesktopCommanderMCP/issues/443)).
- `capabilities(id text pk, label, description, selector_schema jsonb, sandbox_levels text[], default_duration, version)`. Presets: `fs.read`, `fs.write`, `exec`, `proc.interactive`, `net.egress`, `gui` (off; decision D7).
- `risks(id text pk, title, plain_text, severity low|med|high, mitigation, source_url null, version)` and `capability_risks(capability_id, risk_id, when_selector jsonb)`; e.g. `net.egress` to a broad domain → risk "data can leave via that site" ([Claude sandboxing](https://code.claude.com/docs/en/sandboxing)).
- `access_requests(id uuid, tenant_id, requester principal, task_id null, machine_id, capability, selector jsonb, sandbox_level, duration jsonb, reason text, risk_snapshot jsonb, status pending|approved|narrowed|denied|cancelled|expired, decided_by null, decided_at, grant_id null, created_at, expires_at)`.
- `grants(id uuid, tenant_id, principal_id, machine_id, capability, selector jsonb, sandbox_level, task_id null, expires_at null, uses_max null, uses int, approved_by, approved_at, risk_snapshot jsonb, request_id → access_requests, revoked_at null, revoked_by null, revoke_reason, version)`. Ended grants are kept (history); events reference `grant_id`.
- `proc_leases(id uuid, machine_id, grant_id, holder principal, task_id null, argv_fp text, pid_hint int, fence bigint (from fence_seq), lease_expires_at, heartbeat_at, state running|exited|killed|lost, exit_code null, started_at, ended_at)`.
- `providers(id text pk, kind api|subscription_cli, label, models jsonb, router_endpoint_id null, cli_name null, terms_url, terms_status verified|unverified, terms_note, version)`.
- `router_endpoints(id text pk, label, url, kind litellm|openrouter|direct, secret_id null, version)`.
- `secrets(id uuid, tenant_id, owner principal, label, ciphertext bytea, fp text, created_at, rotated_at, revoked_at)`; storage choice is decision D5.
- `subscriptions(id uuid, tenant_id, owner principal (human), provider_id, label, kind api_key|cli_login, secret_id null, worker_principal_id null, machine_id null, budget jsonb null, enabled, version)`. A `cli_login` row holds **no credential**: it names the machine whose official CLI is logged in.
- `role_bindings(id uuid, tenant_id, owner, role_id → roles, chat_app text, subscription_id, priority int, fallback_subscription_id null, enabled, version)`; e.g. role `reviewer` in chat app `codex` → subscription "Jesse ChatGPT Pro (laptop)".
- `chat_apps(id text pk, label, provider_ids text[], connect_template text, version)`. Presets: `claude`, `codex`, `grok`, `cursor`.
- `tenants(id text pk, name, edition personal|cloud, plan text, version)`. The personal edition has one row `local`; `tenant_id` defaults to it (decision D8).

## Settings presets (new keys)
`online_window_seconds` 120 · `bridge_poll_seconds` 25 · `grant_default_duration` `"task"` · `grant_duration_options` `["once","task","1h","8h","until_revoked"]` · `grant_cache_seconds` 10 · `proc_lease_ttl_seconds` 300 · `exec_timeout_default_seconds` 120 · `exec_timeout_max_seconds` 3600 · `exec_output_cap_bytes` 1048576 · `fs_read_cap_bytes` 4194304 · `request_expiry_minutes` 30 · `reminder_style` `"chip+banner"` · `unsandboxed_allowed` false (D1).
