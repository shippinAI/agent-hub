# 39f. P3: assign tasks to subscriptions; stats and costs

[Plan](39-chatmaton-gold-build-plan.md) · Data model from [30](30-cost-tracking.md) and [33](33-cost-data-and-ui.md);
connections from [37](37-subscription-sign-in.md). Closes G-22 and the status part of G-27.
**Done when:** from the phone, Jesse opens a board task, picks "Codex (ChatGPT Pro)" and it runs. The Stats page
then shows that run's tokens and cost under provider, prompt and task. Codex's rate-limit window is visible.

## Requirements (scope)
- **In:** assign a hub task to a provider seat (optionally a model and profile); run it as a one-seat room linked
  to the task; usage ledger fed by every turn; price catalog; quota snapshots (Codex `account/rateLimits/read`,
  observed limits for the others); stats grouped by provider, model, prompt (message), task and room; budgets with
  alerts that use P2 push; subscription connection status.
- **Out:** automatic routing ("cheapest seat that can do X") as a default. It ships as an optional routing rule
  (data), off by default.

## Design
- **Assignment is a task field plus a room.** `task_assignments` links a task to `{provider_id, model, profile,
  machine_id}`. "Run" creates a room (`rooms.settings.task_id`) seating one seat, posts the task description
  through the P1 context template, and moves the task to `in_progress` with the existing claim and lease rules
  (`httpapi/tasks.go`). The result is a note on the task. A review lane with `distinct_from_previous` can seat a
  different provider as reviewer ([24](24-chatmaton-spec.md)).
- **Usage ledger.** Each `seat.turn` writes `usage_events` (doc 33 columns: tenant, occurred_at, provider_id,
  model, subscription_id, room_id, turn_id, task_id, message_id, tokens by kind, `cost_usd`, `cost_basis`
  `provider_charged|computed|client_estimate`). Grok's headless `total_cost_usd` and Claude's `result` cost are
  `client_estimate`. Computed cost uses `price_catalog` rows (data, with `effective_from`). The prompt dimension is
  the triggering `room_messages.id`.
- **Rollups.** SQL views over monthly partitions (`usage_daily_by_provider`, `…_by_task`, `…_by_prompt`). There are
  no materialized views until data volume shows a need.
- **Quota.** `quota_snapshots` from Codex rate limits (adapter), and observed `limit reached` events from Claude
  and Grok streams. The UI says "observed" and never guesses a reset time (doc 37 rule).
- **Gold services v2.** Tiles gain guided actions: Vaultix "add secret" (A2UI modal, [36](36-vaultix.md)), Zitadel
  "new OIDC app", mail "add alias", DNS "add record". Each guide ends in a gold-policy ticket that is approved in
  P2's inbox.

## Construction: files
**Migration `012_usage_assign.sql`.** `usage_events` (partitioned by month on `occurred_at`; partitions created by
the hub like `auditkeep`), `price_catalog`, `quota_snapshots`, `budgets`, `budget_alerts` (doc 33 shapes),
`task_assignments(task_id fk, provider_id fk, model, profile, machine_id, assigned_by, room_id null, created_at,
version)`, and `subscription_connections` (doc 37 shape; status from `seats/health.go` and adapter results).
**Server.** `internal/usage/`: `ingest.go` (from `seat.turn`), `cost.go` (price lookup, basis), `query.go` (group
by), `partitions.go`, `quota.go`. `internal/assign/`: `assign.go`, `run.go` (create room, link task, post),
`result.go` (note on the task, state change). `internal/budgets/`: `eval.go`, `alerts.go` (P2 `notify`).
`internal/goldsvc/guides.go`. Routes: `devapi/usage_routes.go`, `devapi/assign_routes.go`. Presets:
`price-catalog.json` (sourced per doc 30, with `source_url` on each row), `dashboards.json` (default graphs, doc 33).
**Web.** `web/src/pages/Stats.tsx`; `pages/stats/UsageTable.tsx` (React Aria `Table`, sortable);
`pages/stats/GroupBySelect.tsx`; `pages/stats/RangePicker.tsx` (`DateRangePicker`); `pages/stats/BarList.tsx` and
`pages/stats/Spark.tsx` (plain SVG with a table fallback; no chart library); `pages/stats/QuotaMeter.tsx`
(`Meter`); `pages/tasks/AssignDialog.tsx` (`Modal`, `Select` provider and model from `provider_models`,
`RadioGroup` profile); edit `pages/TaskDetail.tsx` (an "Assign" button and the linked room);
`pages/settings/Budgets.tsx`; `pages/dev/Connections.tsx`.

## Endpoints
| Method and path | Purpose |
|---|---|
| `POST /api/v1/tasks/{id}/assign` · `POST /api/v1/tasks/{id}/run` · `DELETE /api/v1/tasks/{id}/assign` | Assign, run, unassign |
| `GET /api/v1/usage?group=provider\|model\|task\|prompt\|room&from=&to=` | Rollups (tenant scoped) |
| `GET /api/v1/usage/turns/{turn}` | One turn's ledger rows |
| `GET /api/v1/quota` · `POST /api/v1/quota/refresh` | Snapshots; refresh asks the adapter (Codex) |
| `GET/PUT /api/v1/price-catalog` · `GET/POST/PATCH /api/v1/budgets` | Data editors |
| `GET /api/v1/subscription-connections` | Status per provider (never tokens) |
MCP: `usage_summary` (read-only) and `task_assign` (needs the policy `task.assign`).

## Testing
Go: cost basis selection; price `effective_from` boundaries; partition creation; group-by sums equal turn sums;
budget `pause` stops a seat from starting a turn; assignment run creates exactly one room and one note.
hubtest: `assign_test.go` (fake seat stream with usage → ledger → `/usage?group=task`). UI:
`tests/ui/stats-shots.mjs` (axe; the table fallback is read by screen readers; keyboard sort).

## Gold tickets (drafts)
| Draft id | Change | Verify |
|---|---|---|
| GC-CM-P3-01 | Deploy migration 012 and the new build; backfill `usage_events` from `room_turns.usage` (an idempotent script reviewed in the ticket) | Sums match `room_turns` |
| GC-CM-P3-02 | Gold service health probes: allow-list the probe URLs (console, Vaultix, Zitadel, mail, PowerDNS API, Tailscale local API read-only, Cloudflare API read-only token stored in Vaultix) | Tiles green; tokens never shown |
