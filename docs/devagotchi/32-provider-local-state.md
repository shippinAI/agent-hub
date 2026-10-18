# 32. Provider local state: paths, transcripts, detection

[Index](../devagotchi-spec.md) · Companion to [29](29-provider-matrix.md). Read 2026-10-18. `~` is `$HOME` on
macOS/Linux and `%USERPROFILE%` on Windows unless a row says otherwise. Every path below is a **data row**
(`providers.detect`), not code, because vendors move them (Windsurf → Devin, Q → Kiro, Gemini → Antigravity).

## Rules
1. **Live capture first.** When Chatmaton drives a seat it stores every turn it sends and every event it gets
   back in the hub (`transcript_items`, below) as it streams. That copy is ours and never depends on a
   vendor's file format.
2. **Import second, through the official read surface** (app-server, ACP `session/load`, `export`, a list
   command, a local HTTP API). Reading the vendor's own files is the last resort, behind a versioned, tolerant
   parser per client, because vendors say the format is internal: Claude Code "changes between versions, so
   scripts that parse these files directly can break" ([sessions](https://code.claude.com/docs/en/sessions)).
3. **Read-only, and never credentials.** Files named `auth.json`, `credentials*`, keychain entries and token
   stores are on a deny list in the bridge; import reads transcript and config paths only, under a grant
   (`fs.read` with the selector below, [04](04-access-flow.md)).
4. **Resume uses the client's own id.** A seat stores `external_session_id`; the next turn passes it back
   (`--resume`, `thread/resume`, ACP `session/load`), so context lives where the vendor expects it.

## Per provider
| # | Client | Config + MCP | Sessions / transcripts | Official read surface |
|---|---|---|---|---|
| 1 | Claude Code | `~/.claude/settings.json`; MCP in `~/.claude.json` and project `.mcp.json` ([MCP](https://code.claude.com/docs/en/mcp)); `CLAUDE_CONFIG_DIR` moves it | `~/.claude/projects/<cwd-with-dashes>/<session>.jsonl`, subagents under `<session>/subagents/`, 30-day default (`cleanupPeriodDays`) ([dir](https://code.claude.com/docs/en/claude-directory)) | `/export`, `--resume <id>` ([sessions](https://code.claude.com/docs/en/sessions)) |
| 1b | Claude Desktop | `claude_desktop_config.json` in `~/Library/Application Support/Claude/`, `%APPDATA%\Claude\`, or the MSIX path `%LOCALAPPDATA%\Packages\Claude_pzs8sxrjxfjjc\LocalCache\Roaming\Claude\` ([issue](https://github.com/anthropics/claude-code/issues/26073)) | Desktop and Cowork code sessions also land in `~/.claude/projects` (own retention key `desktopSessionCleanupPeriodDays`, [sessions](https://code.claude.com/docs/en/sessions)) | none for chats; MCP host only |
| 2 | Codex | `~/.codex/config.toml`, `[mcp_servers.<id>]` ([ref](https://learn.chatgpt.com/docs/config-file/config-reference)); `CODEX_HOME` | `~/.codex/sessions/YYYY/MM/DD/rollout-*.jsonl`, compressed to `.jsonl.zst` after 7 days; `session_index.jsonl`, `history.jsonl` ([termdeck](https://termdeck.io/guides/where-codex-cli-stores-sessions)) | app-server `thread/list`, `thread/turns/list`, `thread/items/list` ([docs](https://learn.chatgpt.com/docs/app-server)) |
| 3 | Grok Build | `~/.grok/config.toml`, project `.grok/config.toml` (MCP, plugins, permissions only) ([settings](https://docs.x.ai/build/settings)); `GROK_HOME` | `~/.grok/sessions/` (SQLite), keyed by working directory ([headless](https://github.com/xai-org/grok-build/blob/main/crates/codegen/xai-grok-pager/docs/user-guide/14-headless-mode.md)) | `grok sessions list/search`, `grok export`, ACP ([sessions](https://docs.x.ai/build/features/sessions)) |
| 4 | Antigravity CLI | `~/.gemini/antigravity-cli/settings.json`; MCP `~/.gemini/config/mcp_config.json` or `.agents/mcp_config.json` ([MCP](https://antigravity.google/docs/cli/mcp/)) | Workspace → conversation cache `~/.gemini/antigravity-cli/cache/last_conversations.json` ([resume](https://antigravity.google/docs/cli/commands/resume/)); transcript files not documented | `agy --conversation <id>`; JSON per turn (`conversation_id`, tokens) |
| 4b | Gemini CLI (enterprise) | `~/.gemini/settings.json` | `~/.gemini/tmp/<project_hash>/chats/` with tokens per turn ([sessions](https://geminicli.com/docs/cli/session-management/)) | `gemini --resume` |
| 5 | Cursor CLI | `~/.cursor/mcp.json`, project `.cursor/mcp.json` ([MCP](https://cursor.com/docs/cli/mcp)) | `~/.cursor/projects/<p>/agent-transcripts/<id>/<id>.jsonl`; chat metadata `~/.cursor/chats/<hash>/<id>/store.db` ([guide](https://jazzyalex.github.io/agent-sessions/guides/cursor-agent-local-history.html)) | `agent ls`, `agent --resume <id>` ([params](https://cursor.com/docs/cli/reference/parameters)) |
| 6 | Copilot CLI | `~/.copilot/`, MCP `~/.copilot/mcp-config.json`; `COPILOT_HOME` ([dir](https://docs.github.com/en/copilot/reference/copilot-cli-reference/cli-config-dir-reference)) | `~/.copilot/session-state/<id>/` | session data guide ([chronicle](https://docs.github.com/en/copilot/how-tos/copilot-cli/use-copilot-cli/chronicle)), `--resume` |
| 7 | Devin Desktop / CLI | `~/.config/devin/config.json` and `mcp_config.json` (Windows `%APPDATA%\devin\`); legacy `~/.codeium/windsurf/mcp_config.json` stays readable ([MCP](https://docs.devin.ai/desktop/cascade/mcp), [config](https://docs.devin.ai/cli/reference/configuration/config-file)) | not documented | ACP `devin acp` |
| 8 | Kiro CLI | `~/.kiro/settings/mcp.json`, project `.kiro/settings/mcp.json` ([config](https://kiro.dev/docs/cli/chat/configuration/)) | SQLite under `~/.kiro/`, per directory ([sessions](https://kiro.dev/docs/cli/chat/session-management/)) | `kiro-cli chat --resume-id <id>`, ACP |
| 9 | Mistral Vibe | `~/.vibe/config.toml` (project `.vibe/` wins), `[[mcp_servers]]` ([config](https://docs.mistral.ai/vibe/code/cli/configuration)) | `~/.vibe/logs` when `log_interactions = true`; ACP log opt-in `~/.vibe/logs/acp/messages.jsonl` ([logger](https://github.com/mistralai/mistral-vibe/blob/f71bfd3b/vibe/acp/acp_logger.py)) | ACP |
| 10 | Muse Code | `~/.config/muse/settings.json` (`schema_version`, `mcp_servers`) ([config](https://dev.meta.ai/docs/muse-code/configuration)) | Kept forever unless `session_retention_days` is set (same page) | `muse exec --session-id <uuid>` |
| 11 | Kimi Code | `~/.kimi-code/config.toml`, `~/.kimi-code/mcp.json`; `KIMI_CODE_HOME` ([data](https://www.kimi.com/code/docs/en/kimi-code-cli/configuration/data-locations.html)) | `~/.kimi-code/sessions/<workDirKey>/<sessionId>/` ([sessions](https://www.kimi.com/code/docs/en/kimi-code-cli/guides/sessions)) | ACP session resume |
| 12 | Qwen Code | `~/.qwen/settings.json`, project `.qwen/settings.json`, `mcpServers` ([settings](https://qwenlm.github.io/qwen-code-docs/en/users/configuration/settings/)) | Recorded when `general.chatRecording` is on (default) | `--resume`, `--continue`, ACP |
| 13–15 | Z.ai, MiniMax, DeepSeek | The host harness's files (rows 1, 17); MiniMax wizard edits `~/.claude/settings.json` ([CLI](https://platform.minimax.io/docs/m-plan/minimax-cli)) | The host harness's sessions | The host harness's surface |
| 16 | Perplexity Mac | Connectors set in the app UI; needs the PerplexityXPC helper ([help](https://www.perplexity.ai/help-center/en/articles/11502712-local-and-remote-mcps-for-perplexity)) | Cloud only | none (API for seats) |
| 17 | OpenCode | `~/.config/opencode/opencode.json`, `mcp` key ([MCP](https://opencode.ai/docs/mcp-servers/)) | `~/.local/share/opencode/` ([troubleshooting](https://opencode.ai/docs/troubleshooting/)) | `opencode serve`: `GET /session` ([server](https://opencode.ai/docs/server/)) |
| 17b | Goose | `~/.config/goose/config.yaml`, `extensions` ([config](https://github.com/block/goose/blob/main/documentation/docs/guides/config-files.md)) | `sessions.db` | `goose session list --format json` |
| 18 | Cline CLI | `~/.cline` (`CLINE_DIR`); MCP `~/.cline/data/settings/cline_mcp_settings.json` ([ref](https://docs.cline.bot/cli/cli-reference)) | under `~/.cline/data` | ACP |

"Not documented" means we found no vendor page; for those rows Chatmaton relies on live capture (rule 1).

## Detection (each a data rule, run by the bridge under `fs.read` + `exec` of `--version` only)
| Signal | macOS | Windows | Linux |
|---|---|---|---|
| CLI binary | `PATH` lookup, then known install dirs (`~/.local/bin`, Homebrew, npm global) | `PATH` + `where.exe`; WinGet/npm global dirs | `PATH`, `~/.local/bin`, npm global |
| Version | `<bin> --version` with a 5 s timeout, parsed by a per-provider regex | same | same |
| Config dir | Exists and readable (table above) | same, also `%APPDATA%` / MSIX virtualized paths | same, honoring `XDG_CONFIG_HOME` |
| Desktop app | `/Applications/*.app` and `~/Applications`, bundle id | Uninstall registry keys and `Get-AppxPackage` for MSIX | `.desktop` files in `/usr/share/applications`, `~/.local/share/applications`, Flatpak/Snap lists |
| Signed in | The client's own status command when it has one (`/status`, `grok inspect --json`, `codex` app-server `account/read`); never by opening token files | same | same |
| WSL | Each distro is its own machine row ([01](01-data-model.md)); detection runs inside the distro | — | — |

Detection output is a `client_installs` row per machine (see [31](31-installed-awareness.md)). A row whose
signal is missing shows "not found" with the rule used, so the user can fix the path in the UI.

## Hub tables for combined history
- `seat_sessions(id, room_id, seat_key, provider_id, machine_id, external_session_id, surface, started_at,
  last_turn_at, imported bool, version)`; `seat_key` is the participant key in `rooms.participants` ([24](24-chatmaton-spec.md)).
- `transcript_items(id, seat_session_id, seq, role user|assistant|tool|system|event, content jsonb, usage_event_id
  null, source live|import, source_ref text, created_at)`. Live rows come from the stream; import rows carry the
  file or API cursor they came from, so a re-import is idempotent (unique on `seat_session_id, source_ref`).
- A room's shared view is a query over `transcript_items` by room, ordered by time, labeled with each seat's
  provider icon and model ([24](24-chatmaton-spec.md)).
