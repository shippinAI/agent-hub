# 29. Provider matrix: the 18 subscriptions Chatmaton drives

[Index](../devagotchi-spec.md) · Read 2026-10-18. Supersedes the terms columns of [23](23-chatmaton-research.md).
Local paths, transcripts and detection per provider are in [32](32-provider-local-state.md); cost and quota in
[30](30-cost-tracking.md); alignment with other installed tools in [31](31-installed-awareness.md).

## Decision this file applies (Jesse, 2026-10-18)
The local edition is free, non-profit and for personal use. The user signs in to each official client
themselves; Chatmaton drives that client and reads and combines its state. Terms are not shown as a blocker
and no row carries an "unverified" badge. We still never read or copy a client's credential files
([08](08-threat-model.md) T11): we start the signed-in client and talk to its documented interface.

## How the 18 were chosen
1. One row per **subscription a person pays for** (or a free open harness they bring their own plan to).
2. The vendor ships an **official client that can be driven**: a headless mode, ACP (Agent Client Protocol,
   [agents list](https://agentclientprotocol.com/get-started/agents)), an app-server, or an SDK.
3. Products that were renamed count once under the current name: Windsurf is now Devin Desktop
   ([blog](https://devin.ai/blog/windsurf-is-now-devin-desktop)); Amazon Q Developer CLI is now Kiro CLI
   ([Kiro](https://kiro.dev/docs/upgrade-guides/migrating-from-q/)); consumer Gemini CLI moved to Antigravity CLI
   on 2026-06-18 ([Google](https://developers.googleblog.com/en/an-important-update-transitioning-gemini-cli-to-antigravity-cli/)).
4. Left out as rows, kept as data the user can add (a `providers` row, [01](01-data-model.md)): Amp, Factory,
   Augment, Junie, Warp, Zed, Jules. Reason: not in Jesse's list and none adds a new integration kind.

## Integration kinds (the order Chatmaton prefers)
| Kind | Meaning | Why first/last |
|---|---|---|
| `app_server` | Long-lived JSON-RPC with threads, turns, models, rate limits | Richest: history, usage and quota without parsing files |
| `acp` | ACP over stdio (`session/new`, `session/prompt`, streamed updates) | One adapter covers many vendors |
| `headless` | One process per turn, JSONL out, resume by session id | Works everywhere; history via the client's own export |
| `api` | HTTPS API with the user's key (direct or via router) | Exact token and cost data; not a subscription |
| `mcp_host` | The app hosts MCP servers but cannot be driven | Devagotchi is a tool inside it, not a seat in a room |

## The 18
| # | Vendor · subscription | Official client(s) | Surface Chatmaton uses | Model list source |
|---|---|---|---|---|
| 1 | Anthropic · Claude Pro/Max | Claude Code CLI; Claude Desktop (MCP host, Cowork) | `headless`: `claude -p --output-format stream-json` ([docs](https://code.claude.com/docs/en/headless)); Desktop as `mcp_host` | `/model` in the CLI; API `GET /v1/models` ([docs](https://platform.claude.com/docs/en/api/models/list)) for key users |
| 2 | OpenAI · ChatGPT Plus/Pro | Codex CLI, Codex app, ChatGPT desktop | `app_server`: `thread/*`, `turn/start`, `model/list`, `account/rateLimits/read` ([docs](https://learn.chatgpt.com/docs/app-server)) | `model/list` (per account) |
| 3 | xAI · SuperGrok / X Premium+ | Grok Build CLI (`grok`) | `acp`: `grok agent stdio`; `headless`: `grok -p --output-format json` ([CLI](https://docs.x.ai/build/cli/reference)) | `grok models` |
| 4 | Google · AI Pro/Ultra | Antigravity CLI (`agy`), Antigravity IDE, Gemini app | `headless`: `agy -p --output-format json`, returns `conversation_id` and tokens ([docs](https://antigravity.google/docs/cli/headless/)); no native ACP yet ([issue](https://github.com/google-antigravity/antigravity-cli/issues/31)) | `agy models` |
| 5 | Cursor · Pro/Ultra | Cursor IDE, Cursor CLI (`agent`) | `acp`: `agent acp` ([docs](https://cursor.com/docs/cli/acp)); `headless`: `agent -p` | `agent models` / `--list-models` ([params](https://cursor.com/docs/cli/reference/parameters)) |
| 6 | GitHub · Copilot Pro/Pro+ | Copilot CLI, VS Code, JetBrains | `acp`: `copilot --acp --stdio` ([docs](https://docs.github.com/en/copilot/reference/copilot-cli-reference/acp-server)); `headless`: `copilot -p --output-format json` ([ref](https://docs.github.com/en/copilot/reference/copilot-cli-reference/cli-command-reference)) | `--model` / `/model` in the CLI |
| 7 | Cognition · Devin Desktop (ex-Windsurf) | Devin Desktop, Devin CLI (`devin`) | `acp`: `devin acp` ([commands](https://docs.devin.ai/cli/reference/commands)) | `--model` in ACP; app picker |
| 8 | Amazon · Kiro (ex-Q Developer) | Kiro IDE, Kiro CLI (`kiro-cli`, old `q`) | `acp`: `kiro-cli acp` ([docs](https://kiro.dev/docs/cli/acp/)) | `/model` in chat |
| 9 | Mistral · Vibe Pro (ex-Le Chat Pro) | Mistral Vibe CLI, Le Chat apps | `acp`: `vibe-acp` ([setup](https://github.com/mistralai/mistral-vibe/blob/main/docs/acp-setup.md)); `headless`: `vibe --prompt --output json` ([CLI](https://docs.mistral.ai/vibe/code/cli/work-with-cli)) | Vibe config; API `GET /v1/models` |
| 10 | Meta · Muse Code plans | Muse Code CLI (`muse`) | `headless`: `muse exec --json`, resume with `--session-id` ([extending](https://dev.meta.ai/docs/muse-code/extending)); subscription works only in this CLI ([plans](https://dev.meta.ai/docs/muse-code/subscriptions)) | Muse settings; Model API for keys ([coding agents](https://dev.meta.ai/docs/coding-agents)) |
| 11 | Moonshot · Kimi Code | Kimi Code CLI (`kimi`) | `acp`: `kimi acp` ([docs](https://www.kimi.com/code/docs/en/kimi-code-cli/reference/kimi-acp)); `headless`: `kimi --print --output-format stream-json` ([example](https://github.com/MoonshotAI/kimi-cli/blob/8283d785/examples/kimi-cli-stream-json/main.py)) | Kimi config |
| 12 | Alibaba · Qwen Code | Qwen Code CLI (`qwen`) | `acp`: `qwen --acp` ([settings](https://qwenlm.github.io/qwen-code-docs/en/users/configuration/settings/)); `headless`: `qwen -p --output-format stream-json` ([docs](https://qwenlm.github.io/qwen-code-docs/en/users/features/headless/)) | Qwen settings |
| 13 | Z.ai · GLM Coding Plan | ZCode; plan key also used in Claude Code / OpenCode ([tools](https://docs.z.ai/devpack/tool/others)) | Via a host harness (row 1 or 17) with `ANTHROPIC_BASE_URL=https://api.z.ai/api/anthropic` ([docs](https://docs.z.ai/devpack/tool/claude)) | GLM names in the plan docs |
| 14 | MiniMax · M Plan | MiniMax Code (desktop, `mcode` CLI), `mmx` CLI ([docs](https://platform.minimax.io/docs/m-plan/minimax-code)) | Via a host harness: `mmx agent setup` writes the plan key into Claude Code and others ([CLI](https://platform.minimax.io/docs/m-plan/minimax-cli)) | `/provider` in `mcode`; plan page |
| 15 | DeepSeek · pay-as-you-go | Web/mobile chat; no coding CLI | `api` only, or a host harness through its Anthropic-compatible endpoint ([docs](https://api-docs.deepseek.com/guides/anthropic_api/)) | API `GET /models` |
| 16 | Perplexity · Pro/Max | Perplexity apps; Mac app hosts local MCP ([help](https://www.perplexity.ai/help-center/en/articles/11502712-local-and-remote-mcps-for-perplexity)) | `mcp_host` for Devagotchi; Chatmaton seat only via `api` (Sonar/Agent API, [pricing](https://docs.perplexity.ai/docs/getting-started/pricing)) | API docs |
| 17 | Open harness · OpenCode (and Goose) | `opencode`, `goose` | OpenCode: `opencode serve` HTTP API and `opencode acp` ([server](https://opencode.ai/docs/server/)); Goose: `goose acp` ([docs](https://goose-docs.ai/docs/gdk/acp/)) | The harness's configured providers |
| 18 | Open harness · Cline | Cline CLI and IDE extensions | `acp`: `cline --acp` ([docs](https://docs.cline.bot/usage/acp)); headless with `--json` ([CLI](https://docs.cline.bot/usage/cli-overview)) | Cline provider settings |

Rows 13–15 and 17–18 are how a person who pays Z.ai, MiniMax or DeepSeek gets a seat: the plan key goes into a
harness the user already runs, and Chatmaton drives the harness. The seat's label still shows the real
provider and model ([24](24-chatmaton-spec.md) tab chips), taken from the harness's own model report.

## What this means for the build
- **Three adapters cover 15 rows**: ACP (rows 3, 5–9, 11, 12, 17, 18), Codex app-server (2), headless JSONL
  (1, 4, 10). Rows 13–15 reuse a harness adapter. Row 16 is API plus `mcp_host`.
- **Claude and Codex reach ACP only through adapters** (Zed's [claude-agent-acp](https://github.com/zed-industries/claude-agent-acp),
  [codex-acp](https://github.com/agentclientprotocol/codex-acp), per the [ACP list](https://agentclientprotocol.com/get-started/agents)).
  We use their own surfaces (headless stream, app-server) and keep the adapters as a fallback.
- **Model lists are never hardcoded.** Each adapter reads the client's own list (`model/list`, `grok models`,
  `agy models`, `agent models`, ACP session config) into `providers.models` with `fetched_at`, refreshed on
  connect and on a schedule setting (`model_list_refresh_minutes`, preset 360).
- **Each provider row is data**: `kind`, `surface`, launch argv template, env template, detection rules
  ([32](32-provider-local-state.md)), icon ref and label. A new vendor is a new row plus, at most, an adapter.

## Schema change implied
`providers` drops `terms_status` and keeps an optional `terms_url` and free-text `terms_note`, shown as plain
info, never as a badge ([01](01-data-model.md)). New columns: `surface text[]` (preference order),
`launch jsonb`, `detect jsonb`, `models_fetched_at`.
