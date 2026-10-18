# 33. Cost and usage: data model, graphs, budgets, UI

[Index](../devagotchi-spec.md) · Design for [30](30-cost-tracking.md). Hub Postgres, same rules as
[01](01-data-model.md): editable rows carry `version bigint`; every write appends to `events` in the same
transaction. Nothing below is built yet.

## Tables
- `usage_events` (append-only, monthly partitions on `occurred_at`): `id uuid, tenant_id, occurred_at,
  provider_id, model, subscription_id null, router_endpoint_id null, billing_kind api_key|subscription|router,
  machine_id null, task_id null, room_id null, seat_key text null, routine_run_id null, turn_ref text null,
  external_session_id null, input_tokens, cache_read_tokens, cache_write_tokens, output_tokens,
  reasoning_tokens, tool_tokens, requests int, cost_micros bigint null, cost_basis
  provider_charged|computed|client_estimate, price_id null, source otel|stream|app_server|api_response|import|
  provider_report, raw jsonb, dedupe_key text unique`. `dedupe_key` (provider + response or turn id) makes
  re-ingest idempotent. Token columns are `bigint`, default 0, and stay split by kind ([30](30-cost-tracking.md) rule 1).
- `price_catalog`: `id, provider_id, model, unit per_mtok|per_request|per_image, input, cache_read, cache_write,
  output, reasoning null, request null, currency, effective_from, effective_to null, source_url, source
  litellm|openrouter|vendor_page|user, fetched_at, version`. Unique on `(provider_id, model, effective_from)`.
- `quota_snapshots`: `id, subscription_id, observed_at, limit_id, window_label (e.g. "5h", "7d", "month"),
  window_minutes null, used_percent numeric null, used_value null, limit_value null, resets_at null,
  source client_report|provider_api|observed`. Latest row per `(subscription_id, limit_id)` drives meters.
- `provider_reports`: `id, tenant_id, provider_id, report_kind cost|usage, bucket_start, bucket_width,
  group_key jsonb, cost_micros null, tokens jsonb null, fetched_at`. Used only for reconciliation.
- `budgets`: `id, tenant_id, label, scope_kind tenant|subscription|provider|router_key|project|task|room|routine,
  scope_id null, metric cost_micros|tokens|quota_percent, period day|week|month|rolling_5h|per_task|per_run,
  limit_value bigint, thresholds int[] (preset {50,80,100}), on_threshold jsonb (per threshold: notify|ask|
  pause), enabled, version`.
- `budget_alerts`: `id, budget_id, period_start, threshold, value, fired_at, notified jsonb, acked_by null,
  acked_at null`. Unique on `(budget_id, period_start, threshold)`, so an alert fires once per period.
- Rollups: `usage_hourly` and `usage_daily` materialized views grouped by tenant, provider, model,
  subscription, billing kind, task, room; refreshed by the hub's existing job loop (setting
  `usage_rollup_seconds`, preset 300). Graphs read rollups; drill-down reads `usage_events`.

## Ingest paths (all write `usage_events`)
1. **Seat streams**: each Chatmaton adapter emits one event per turn from the client's own usage report
   (Codex `thread/tokenUsage/updated`, headless JSON, ACP updates).
2. **OTel receiver**: the hub runs an OTLP endpoint on loopback (setting `otlp_listen`, off by default); the
   user can point Claude Code, Codex and Gemini CLI telemetry at it, which covers sessions Chatmaton did not
   start ([Claude](https://code.claude.com/docs/en/monitoring-usage),
   [Codex](https://learn.chatgpt.com/docs/config-file/config-advanced),
   [Gemini CLI](https://google-gemini.github.io/gemini-cli/docs/cli/telemetry.html)). Setting this up is a
   guided step that edits the client's config only after the user approves the diff ([31](31-installed-awareness.md)).
3. **Router / direct API**: response `usage` on every call we make.
4. **Pollers** (each a data row with interval and secret): quota (`account/rateLimits/read`, Z.ai quota),
   provider reports (Anthropic, OpenAI, xAI admin APIs), Copilot and Cursor billing usage.
5. **Import** of old sessions ([32](32-provider-local-state.md)): tokens when the transcript has them, marked
   `source=import`.

## Budgets and alerts
- Evaluated on ingest (cheap counters for the open period) and on each rollup refresh (authoritative).
- Actions: `notify` (in-app notice + board note), `ask` (the next turn in that scope waits for a human
  approve, using the Requests inbox, [04](04-access-flow.md)), `pause` (the room, routine or task stops taking
  new turns). Only a human may raise a limit or resume after `pause`; an agent token cannot edit `budgets`
  (same rule as grants, [01](01-data-model.md) invariant 1).
- Quota budgets use `quota_percent` on the latest snapshot: e.g. "warn at 80% of Claude 5h", "switch this room's
  seat to its fallback subscription at 95%" (uses `role_bindings.fallback_subscription_id`, [01](01-data-model.md)).
- Per-task and per-run budgets are checked before a routine step starts, from the step's declared caps
  ([24](24-chatmaton-spec.md)).

## Default graphs (presets in a `dashboards` data row; the user can add, remove, reorder)
| Graph | Shows | Default range |
|---|---|---|
| Spend over time | stacked bars by provider, `cost_basis` as pattern (solid charged, hatched estimate) | 30 days, daily |
| Tokens over time | stacked by kind (input, cache read, output, reasoning) | 7 days, hourly |
| Quota windows | one meter per subscription limit: used %, time to reset | live |
| Top tasks / rooms | table of the top 10 by cost and by tokens | 30 days |
| Subscription value | per subscription: plan price vs API-equivalent estimate this period | month |
| Cache hit rate | cache read / input, by model | 7 days |
| Budget status | each budget: current vs limit, next threshold | open period |

## UI (React Aria Components only, plus our CSS; no shadcn or Radix)
- **Usage page**: `Tabs` (Overview, Subscriptions, API keys, Budgets, Prices); filters with `Select` (provider,
  model, subscription), `DateRangePicker`, `ToggleButtonGroup` (hour/day/week)
  ([RAC](https://react-aria.adobe.com/)). Filters sync to the URL.
- **Charts** are our own SVG components in `web/src/ui/charts/` (no new dependency; the app ships only React and
  RAC today). Every chart has: a text summary above it, a "Show as table" `Disclosure` with a RAC `Table` of
  the same data, keyboard focus on data points with a `Tooltip`, and patterns plus color so it reads without
  color (W3C [complex images](https://www.w3.org/WAI/tutorials/images/complex/)).
- **Quota**: RAC `Meter` per window ([Meter](https://react-aria.adobe.com/Meter)) with label "Claude Max · 5h ·
  62% · resets 3:40 PM PT"; the reset time is shown in the user's zone.
- **Budgets**: `Table` of budgets; edit in our `Modal` with `NumberField` (limit), `Select` (period, scope),
  `CheckboxGroup` (thresholds); writes need `If-Match` with the row version.
- **Prices**: `Table` of `price_catalog` with source link and effective date; "refresh from source" shows a diff
  before any row changes.
- **Per-turn chip** in Chatmaton tabs: tokens and cost for the last turn, with the basis ("charged",
  "computed", "estimate") in a `Tooltip`.
- **Phone**: the overview becomes cards (spend today, quota meters, alerts); charts collapse to the table view.

## API and MCP
- REST: `GET /api/v1/usage?group_by=&from=&to=` (rollups), `GET /api/v1/usage/events` (paged),
  `GET/PUT /api/v1/budgets/{id}`, `GET /api/v1/quota`, `GET/PUT /api/v1/prices/{id}`.
- MCP (read-only for agents): `usage_summary` and `quota_status`, so an agent can check headroom before a long
  task. No MCP tool writes budgets or prices.
