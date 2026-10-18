# 30. Cost and usage tracking: sources and accounting

[Index](../devagotchi-spec.md) · Read 2026-10-18. Where the numbers come from, how we count them, and the router
option. The data model, graphs, budgets and UI are in [33](33-cost-data-and-ui.md). Providers are the 18 in [29](29-provider-matrix.md).

## Two billing kinds, one ledger
- **API key (usage-billed):** each response carries token counts; cost = tokens × price, or the provider
  states the charge itself. We can be exact per prompt.
- **Subscription (flat plan):** money per prompt is not billed, so we track **tokens** (when the client
  reports them), **quota windows** (percent used, reset time), and an **API-equivalent cost** labeled as an
  estimate, so a user can compare "my $200 plan" with "this would have cost $X at list price".
Both land in one `usage_events` ledger with `billing_kind` and `cost_basis` columns ([33](33-cost-data-and-ui.md)).

## API-key providers: price lists, usage APIs, per-response fields
| Provider | Price source | Org usage / cost API | Per-response fields we read |
|---|---|---|---|
| Anthropic | [pricing](https://platform.claude.com/docs/en/about-claude/pricing) | Admin API `usage_report/messages` (1m/1h/1d buckets, group by model, key, workspace) and `cost_report` (daily USD) ([docs](https://platform.claude.com/docs/en/manage-claude/usage-cost-api)) | `input_tokens`, `output_tokens`, `cache_creation_input_tokens`, `cache_read_input_tokens` ([Messages](https://platform.claude.com/docs/en/api/messages)) |
| OpenAI | [pricing](https://developers.openai.com/api/docs/pricing) | Admin key: `/v1/organization/usage/completions` (1m/1h/1d) and `/v1/organization/costs` (1d) ([admin](https://developers.openai.com/api/docs/guides/admin-apis)) | `input_tokens_details.cached_tokens`, `cache_write_tokens`, `output_tokens_details.reasoning_tokens` ([Responses](https://developers.openai.com/api/reference/resources/responses/methods/create)) |
| xAI | [models](https://docs.x.ai/developers/models) | Management API `POST /v1/billing/teams/{team_id}/usage` time series ([billing](https://docs.x.ai/developers/rest-api-reference/management/billing)) | `usage.cost_in_usd_ticks`; USD = ticks / 10,000,000,000 ([cost tracking](https://docs.x.ai/developers/cost-tracking)) |
| Google Gemini API | [pricing](https://ai.google.dev/gemini-api/docs/pricing) | No per-key cost API found; Cloud Billing export to BigQuery ([export](https://cloud.google.com/billing/docs/how-to/export-data-bigquery)) | `usageMetadata`: prompt, candidates, `cachedContentTokenCount`, `thoughtsTokenCount` ([API](https://ai.google.dev/api/generate-content)) |
| Mistral | [pricing](https://mistral.ai/pricing) | Not found; console only | OpenAI-style `usage` |
| DeepSeek | [pricing](https://api-docs.deepseek.com/quick_start/pricing) | `GET /user/balance` (balance, not history) ([docs](https://api-docs.deepseek.com/api/get-user-balance)) | `prompt_cache_hit_tokens`, `prompt_cache_miss_tokens` ([API](https://api-docs.deepseek.com/api/create-chat-completion)) |
| Perplexity | [pricing](https://docs.perplexity.ai/docs/getting-started/pricing) (tokens plus per-request search fees) | Not found | `usage` plus request counts |
| Meta Model API | billing in [Muse docs](https://dev.meta.ai/docs/muse-code/subscriptions) | Not found | OpenAI/Anthropic-compatible `usage` ([coding agents](https://dev.meta.ai/docs/coding-agents)) |
| OpenRouter | `GET /api/v1/models` pricing per model ([models](https://openrouter.ai/docs/api/api-reference/models/get-models)) | `GET /api/v1/generation?id=` per request ([generation](https://openrouter.ai/docs/api/api-reference/generations/get-request-&-usage-metadata-for-a-generation)) | `usage.cost` and `cost_details.upstream_inference_cost` on every response ([accounting](https://openrouter.ai/docs/cookbook/administration/usage-accounting)) |

Price catalog seed: LiteLLM's `model_prices_and_context_window.json` (per-token input/output/cache prices and
context limits) ([file](https://github.com/BerriAI/litellm/blob/main/model_prices_and_context_window.json)),
merged with OpenRouter's models API, then overridden by any row the user edits. Every price row keeps its
`source_url` and `effective_from`, so a past event is always priced at the rate in force that day.

## Subscriptions: where tokens and quota come from
| Client | Tokens | Quota windows | Cost shown |
|---|---|---|---|
| Claude Code | OTel `claude_code.token.usage`; status line `context_window.*` ([monitoring](https://code.claude.com/docs/en/monitoring-usage)) | status line `rate_limits.five_hour` / `seven_day`: `used_percentage`, `resets_at` (Pro/Max, after first response) ([statusline](https://code.claude.com/docs/en/statusline)) | `cost.total_cost_usd` is a client-side list-price estimate (same page) |
| Codex | app-server `thread/tokenUsage/updated`; OTel `codex.turn.token_usage` ([config](https://learn.chatgpt.com/docs/config-file/config-advanced)) | `account/rateLimits/read` and `account/rateLimits/updated`: `usedPercent`, `windowDurationMins`, `resetsAt` per limit id ([app-server](https://learn.chatgpt.com/docs/app-server)) | estimate from catalog |
| Grok Build | headless JSON per turn ([headless](https://github.com/xai-org/grok-build/blob/main/crates/codegen/xai-grok-pager/docs/user-guide/14-headless-mode.md)) | not found | estimate |
| Antigravity | headless JSON returns token usage per run ([headless](https://antigravity.google/docs/cli/headless/)) | not found | estimate |
| Gemini CLI | OTel `gemini_cli.token.usage` by type ([telemetry](https://google-gemini.github.io/gemini-cli/docs/cli/telemetry.html)) | not found | estimate |
| Copilot | ACP/JSON events | AI Credits since 2026-06-01, token-priced at API rates ([blog](https://github.blog/news-insights/company-news/github-copilot-is-moving-to-usage-based-billing/)); REST billing usage ([docs](https://docs.github.com/en/rest/billing/usage)) | **real** credits, not an estimate |
| Cursor | per-turn events | Team Admin API `filtered-usage-events` with `chargedCents`, hourly ([admin](https://cursor.com/docs/account/teams/admin-api)); none found for individual plans | real for teams |
| Z.ai plan | host harness | official usage-query plugin ([docs](https://docs.z.ai/devpack/extension/usage-query-plugin)); it reads 5-hour and weekly token windows from `/api/monitor/usage/quota/limit` ([audit](https://github.com/joshuadavidthomas/vibeusage/pull/138)) | estimate |
| Muse Code, MiniMax | host or client JSON | plans state 5-hour prompt windows ([Muse](https://dev.meta.ai/docs/muse-code/subscriptions)); MiniMax shows usage in app settings ([docs](https://platform.minimax.io/docs/m-plan/minimax-code)) | estimate |

Where a row says "not found", the graph shows tokens only, plus quota hits we observe (a 429 or a
"limit reached" event from the client), recorded as `quota_snapshots` with `source=observed`.

## Accounting rules (code invariants; the prices themselves are data)
1. **Store tokens by kind, never one total:** input (uncached), cache read, cache write, output, reasoning,
   tool. OpenTelemetry GenAI conventions treat cache-read as a subset of input
   ([GenAI metrics](https://github.com/open-telemetry/semantic-conventions-genai/blob/main/docs/gen-ai/gen-ai-metrics.md));
   we store the subset separately and derive uncached = input − cache read, so no tokens count twice.
2. **Money is integer micro-USD** (`bigint`); xAI ticks and Cursor cents convert on ingest; original value kept
   in `raw`.
3. **`cost_basis`** is one of `provider_charged` (OpenRouter `usage.cost`, xAI ticks, Copilot credits, Cursor
   cents), `computed` (tokens × catalog price), or `client_estimate` (Claude `total_cost_usd`). The UI shows
   the basis next to every number.
4. **Reconcile, don't overwrite.** Daily provider reports (Anthropic, OpenAI, xAI) go to `provider_reports`; a
   gap above `reconcile_tolerance_pct` (preset 2) raises a board note, it does not edit past events.
5. **Attribution** on every event: tenant, subscription, machine, provider, model, and when known task, room,
   seat, routine, and turn. Unknown stays null; we never guess a task.

## Router option
A router gives one key, one usage stream and one budget per virtual key. Options:
- **LiteLLM proxy** (self-hosted, OSS): virtual keys, per-key/team budgets, spend logs at `/spend/logs/v2`
  ([spend](https://docs.litellm.ai/docs/proxy/cost_tracking)). Fits the local edition and our existing
  `router_endpoints` kind `litellm` ([01](01-data-model.md)).
- **OpenRouter** (hosted): widest catalog, cost on every response. Fits users who want one bill.
- **Vercel AI Gateway** ([pricing](https://vercel.com/docs/ai-gateway/pricing)) and **Cloudflare AI Gateway**
  ([docs](https://developers.cloudflare.com/ai-gateway/)): hosted gateways with analytics; candidates for the
  shippin.cloud edition.
**Recommendation:** keep the router optional and pluggable as data (`router_endpoints.kind`). Default the local
edition to direct keys plus our own ledger; offer LiteLLM as the one-click router; accept OpenRouter as a
provider row. Subscriptions never go through a router; they stay in their official client.
