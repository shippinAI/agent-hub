# 23. Chatmaton research: which subscriptions and CLIs can be driven

[Index](../devagotchi-spec.md) · Chatmaton is the multi-model chat and rooms product on the hub, in two editions
(hosted on shippin.cloud, and local). This file lists what each vendor allows. Read 2026-10-18. **Verified** means a
primary vendor page states it; **unverified** means we found no primary statement. This is not legal advice;
Jesse decides (D6, [10](10-decisions.md)).

## Rules that apply to all rows (inference from the sources below)
- Drive only the vendor's **official, unmodified** CLI or app-server, as a child process on the user's machine,
  through its documented headless interface. Never read its token files, never call its private endpoints.
- Superseded 2026-10-18 (Jesse): both editions let users sign in with their own subscriptions; in the hosted
  edition the official client runs in the user's own workspace ([34](34-provider-schema.md)).
- API keys go direct by default; LiteLLM or OpenRouter is optional ([30](30-cost-tracking.md)).

## Candidates

| # | Provider · surface | How to drive it | Subscription use by another app | Status |
|---|---|---|---|---|
| 1 | Anthropic · Claude Code CLI | `claude -p`, `--output-format stream-json`, `--input-format stream-json` ([headless](https://code.claude.com/docs/en/headless)) | User may sign in to the **unmodified** binary with their own plan, also when a platform hosts it; third parties may **not** offer Claude.ai login, route requests through Free/Pro/Max credentials, or collect tokens; hosting it in a product needs the Commercial Terms and no reselling ([legal](https://code.claude.com/docs/en/legal-and-compliance)) | Verified rules; whether a local orchestrator launching it counts as "ordinary, individual usage" is **unverified** |
| 2 | Anthropic · Claude Desktop | Not drivable; it is an MCP **host**: it calls Devagotchi | n/a | Verified ([MCPB](https://claude.com/docs/connectors/building/mcpb)) |
| 3 | Anthropic · API | API key via router | Product builders "should use API key authentication" ([legal](https://code.claude.com/docs/en/legal-and-compliance)) | Verified |
| 4 | OpenAI · Codex CLI | `codex exec` ([non-interactive](https://learn.chatgpt.com/docs/non-interactive-mode)); `codex app-server` JSON-RPC over stdio for rich clients ([app-server](https://developers.openai.com/codex/app-server.md)) | ChatGPT-account auth in CI is for trusted private runners ([CI auth](https://developers.openai.com/codex/auth/ci-cd-auth/)) | Verified mechanism |
| 5 | OpenAI · Sign in with ChatGPT | OAuth into *our* app; Go/Plus/Pro users may let the app use their plan, with per-app weekly limits | Launched with a limited partner set; **commercial** apps apply via an interest form; open-source developers can use the developer docs ([help](https://help.openai.com/en/articles/20001542-using-your-chatgpt-plan-in-other-apps-and-sites), [partners](https://learn.chatgpt.com/docs/sign-in-with-chatgpt), [cookbook](https://developers.openai.com/cookbook/articles/sign-in-with-chatgpt)) | Verified; replaces the old "unverified" OpenAI row in 05 |
| 6 | xAI · Grok Build CLI | `grok -p` headless and full ACP "to build your own bots and agent orchestration apps" ([launch](https://x.ai/news/grok-build-cli)) | SuperGrok / X Premium+ sign-in; no clause on third-party orchestration found | Mechanism verified; ToS reuse **unverified** |
| 7 | xAI · API | API key; server-side remote MCP ([remote MCP](https://docs.x.ai/developers/tools/remote-mcp)) | API terms | Verified |
| 8 | Google · Antigravity CLI | `agy -p`, `--output-format stream-json`, `--input-format stream-json` ([headless](https://antigravity.google/docs/cli/headless/)) | A forum answer says launching the official `agy` as a child process with cached credentials is supported and uses the same entitlements; extracting tokens or calling backends is not ([forum](https://discuss.ai.google.dev/t/is-external-orchestration-of-antigravity-cli-headless-mode-supported-with-account-based-usage/183051/1)) | Mechanism verified; policy from a forum reply whose staff status is **unverified** |
| 9 | Google · Gemini CLI | Headless (`-p`); replaced by Antigravity for unpaid/Google One users on 2026-06-18 ([geminicli.com](https://geminicli.com/docs/extensions/releasing/)) | Terms per auth type ([ToS page](https://geminicli.com/docs/resources/tos-privacy/)) | Partly verified |
| 10 | Cursor · Cursor CLI | `agent -p`, `--output-format json`, `CURSOR_API_KEY` ([headless](https://cursor.com/docs/cli/headless)) | Uses the Cursor subscription | Mechanism verified; ToS for orchestration **unverified** |
| 11 | GitHub · Copilot CLI | `copilot -p`, `--allow-tool` ([CLI](https://docs.github.com/en/copilot/concepts/copilot-surfaces/copilot-cli), [tools](https://docs.github.com/en/copilot/how-tos/copilot-cli/use-copilot-cli/allowing-tools)) | Counts against the user's Copilot plan | Mechanism verified |
| 12 | AWS · Kiro CLI | `kiro-cli chat --no-interactive`, `KIRO_API_KEY` (paid plans) ([headless](https://kiro.dev/docs/cli/headless/)) | API keys for paid Kiro tiers | Mechanism verified |
| 13 | Mistral · Vibe CLI | `vibe --prompt ... --output json` ([CLI](https://docs.mistral.ai/vibe/code/cli/work-with-cli)) | Le Chat Pro includes Vibe ([Vibe 2.0](https://mistral.ai/news/mistral-vibe-2-0/)) | Mechanism verified |
| 14 | Moonshot · Kimi Code CLI | `kimi -p`, `--output-format stream-json` ([kimi command](https://www.kimi.com/code/docs/en/kimi-code-cli/reference/kimi-command)) | Plus+ membership; third-party agents use a Kimi Code API key ([overview](https://www.kimi.com/en/help/kimi-code/membership-guide)) | Verified |
| 15 | Alibaba · Qwen Code | `qwen -p`; free OAuth tier ended 2026-04-15; use API key or Coding Plan ([auth](https://qwenlm.github.io/qwen-code-docs/en/users/configuration/auth/)) | API / Coding Plan key | Verified |
| 16 | Z.ai · GLM Coding Plan | Anthropic-compatible endpoint used from Claude Code etc. | Only in Z.ai's **officially supported tools**; others risk bans ([usage policy](https://docs.z.ai/devpack/usage-policy)) | Verified: restricted |
| 17 | Sourcegraph · Amp | `amp -x` execute mode; can use a ChatGPT plan; Claude plans only via Claude Code mode ([the dial](https://ampcode.com/docs/the-dial)) | Amp is an OpenAI Sign-in partner ([partners](https://learn.chatgpt.com/docs/sign-in-with-chatgpt)) | Verified |
| 18 | Factory · Droid | `droid exec --auto low, medium or high` ([exec](https://docs.factory.com/droid-exec/overview)) | Factory account | Mechanism verified |
| 19 | JetBrains · Junie CLI | headless with `--auth` API key; JetBrains AI subscription via account ([headless](https://junie.jetbrains.com/docs/junie-headless.html)) | API key for automation | Verified |
| 20 | Augment · Auggie | `auggie --print`; print mode may need an enterprise agreement ([automation](https://docs.augmentcode.com/cli/automation/overview)) | Session auth | Verified: may be gated |
| 21 | OpenCode (open source) | `opencode run`; ChatGPT plan via Sign in with ChatGPT ([providers](https://opencode.ai/docs/providers/)) | Removed Claude Pro/Max login after Anthropic's legal request ([v1.3.0](https://github.com/anomalyco/opencode/releases/tag/v1.3.0), [PR #18186](https://github.com/anomalyco/opencode/pull/18186)) | Verified, and a warning |
| 22 | Ollama (local models) | Local HTTP API, no subscription | n/a | Verified ([FAQ](https://docs.ollama.com/faq)) |

## Integration protocols worth using (instead of scraping terminals)
- **ACP** (Agent Client Protocol): registry lists Gemini CLI natively and Claude/Codex via adapters
  ([agents](https://agentclientprotocol.com/get-started/agents), [registry](https://agentclientprotocol.com/get-started/registry)).
  Grok Build supports ACP ([launch](https://x.ai/news/grok-build-cli)).
- **Codex app-server**: JSON-RPC with threads, turns, approvals, streamed items ([app-server](https://developers.openai.com/codex/app-server.md)).
- **stream-json** (Claude Code, Antigravity, Kimi): NDJSON events in and out.
- Prefer ACP where an official agent exists; fall back to the vendor's stream-json; never PTY-scrape a TUI.

## Not researched / unverified
Perplexity, DeepSeek, MiniMax, Meta, Devin, Warp, Goose, Cline, Kilo Code: not checked in this pass. Devin, Warp and
Kilo Code appear on OpenAI's plan-usage partner list ([partners](https://learn.chatgpt.com/docs/sign-in-with-chatgpt)).
