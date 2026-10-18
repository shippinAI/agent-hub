# 31. Installed awareness: other AI tools on the machine

[Index](../devagotchi-spec.md) · Read 2026-10-18. Devagotchi lands on machines that already run Claude, Codex,
Cursor, Desktop Commander and others. It should see them, import what is useful, avoid fighting them, and
share instructions and skills with them. Paths per client are in [32](32-provider-local-state.md).

## Principles
1. **Look, then propose; never silently write.** Detection is read-only. Any change to another tool's config
   is a proposed diff the user approves, applied with the expected-hash, temp+fsync+rename write rule
   ([01](01-data-model.md) invariant 6) and a dated backup next to the file.
2. **Use the client's own command when it has one**, and edit the file only when it has none:
   `claude mcp add-json` ([MCP](https://code.claude.com/docs/en/mcp)), `grok mcp add` ([CLI](https://docs.x.ai/build/cli/reference)),
   `kiro-cli mcp add` ([CLI](https://kiro.dev/docs/reference/cli-commands/)), `agent mcp enable|disable`
   ([Cursor](https://cursor.com/docs/cli/mcp)). The client then keeps its own backups and schema.
3. **Respect managed policy.** If a client has a managed allowlist (Claude Code `allowedMcpServers`,
   `managed-mcp.json` ([settings](https://code.claude.com/docs/en/settings-reference)); Grok `managed_config.toml`
   and `requirements.toml` ([settings](https://docs.x.ai/build/settings)); Devin team allowlists, where "once you
   allowlist even a single MCP server, all non-allowlisted servers will be blocked"
   ([Devin](https://docs.devin.ai/desktop/cascade/mcp))), we show it and propose nothing it would block.
4. **Secrets stay where they are.** Imported env values and headers are kept as **names** only; a value moves
   into our `secrets` table only if the user pastes it, never by copying it from the other tool's file.

## 1. Detect
The bridge runs the detection rules in [32](32-provider-local-state.md) at enrollment, on demand, and on a
schedule setting (`awareness_scan_hours`, preset 24). Results:
- `client_installs(id, machine_id, provider_id, client_kind cli|desktop|ide, version, bin_path, config_paths
  jsonb, signed_in bool null, rule_id, detected_at, last_seen_at)`.
- `mcp_sightings(id, machine_id, client_install_id, name, transport stdio|http|sse, spec_redacted jsonb,
  spec_hash, source_path, source_key, scope user|project|managed, enabled bool, first_seen, last_seen)`.
  `spec_hash` is over command, args, url and env **names**, so the same server in five clients groups as one.

## 2. Read MCP configs (one parser per format; all are data-driven field maps)
| Shape | Clients |
|---|---|
| JSON `mcpServers` | Claude Desktop, Claude Code (`~/.claude.json`, `.mcp.json`), Cursor, Antigravity, Kiro, Devin/Windsurf, Qwen, Cline, Kimi |
| JSON `servers` | VS Code `.vscode/mcp.json` ([VS Code](https://code.visualstudio.com/docs/copilot/customization/mcp-servers)) |
| JSON `mcp_servers` | Muse Code ([config](https://dev.meta.ai/docs/muse-code/extending)) |
| JSON `mcp` (command as array, `type` local or remote) | OpenCode ([MCP](https://opencode.ai/docs/mcp-servers/)) |
| TOML `[mcp_servers.<id>]` | Codex ([ref](https://learn.chatgpt.com/docs/config-file/config-reference)), Grok Build ([settings](https://docs.x.ai/build/settings)) |
| TOML `[[mcp_servers]]` | Mistral Vibe ([MCP](https://docs.mistral.ai/vibe/code/cli/mcp-servers)) |
| YAML `extensions` | Goose ([config](https://github.com/block/goose/blob/main/documentation/docs/guides/config-files.md)) |
| App UI only | Perplexity Mac ([help](https://www.perplexity.ai/help-center/en/articles/11502712-local-and-remote-mcps-for-perplexity)), ChatGPT connectors |
Unknown keys are kept verbatim in `spec_redacted.extra` so a round trip loses nothing. Variable syntaxes are
recorded, not expanded (Devin uses `${env:VAR}` and `${file:path}`, [Devin](https://docs.devin.ai/desktop/cascade/mcp)).

## 3. Import MCP servers into Devagotchi
- The **Installed tools** page lists each distinct `spec_hash` with the clients that use it. "Import" creates
  a Devagotchi MCP registry row (the extension model in [19](19-public-product-spec.md)) that the user can then
  expose through the hub to every seat, so a server configured once in Claude Desktop becomes available to
  Codex, Grok and the rest without retyping.
- Stdio servers imported this way run through the bridge under grants like any extension; remote servers keep
  their URL and get OAuth through our flow, not the other client's tokens.

## 4. Avoid conflicts
| Conflict | What we do |
|---|---|
| **Desktop Commander and Devagotchi both installed** in one client (overlapping `read_file`, `start_process`…) | Flag it; offer to disable DC in that client (client command or `disabled` flag), never delete; show the tool overlap list |
| **Same stdio server in many clients** (N processes, duplicate side effects, port clashes) | Offer to point those clients at one shared HTTP endpoint served by the hub instead |
| **Name collision**: same name, different spec | Import under `name@client`; show both specs side by side |
| **Tool caps**: Cascade allows 100 tools in total ([Devin](https://docs.devin.ai/desktop/cascade/mcp)) | Before adding Devagotchi to a capped client, show the tool count and offer a smaller tool profile |
| **Devagotchi added twice** (user + project scope) | Detect by our server id; keep the narrower scope, propose removing the other |
| Client open while we edit its file | Prefer the client's command; else write, then tell the user the client may need a restart |

## 5. Add Devagotchi to other clients
One action per detected client: "Add Devagotchi to Cursor / Codex / Claude Desktop / …". It renders that
client's shape from a template row (`client_templates`, data) with our hub URL and an install-scoped token
reference, shows the diff, then applies it via rule 2. Removal is the same flow in reverse.

## 6. Share instructions (AGENTS.md)
- **AGENTS.md** is the shared file format; its compatibility list includes Codex, Cursor, Gemini CLI, Copilot
  and others ([agents.md](https://agents.md/)). Claude Code reads AGENTS.md when no CLAUDE.md exists (v2.1.277+)
  and supports `@AGENTS.md` imports inside CLAUDE.md ([memory](https://code.claude.com/docs/en/memory)).
- Devagotchi treats the repo's `AGENTS.md` as the source and never overwrites any instruction file. If a repo
  has only CLAUDE.md, we propose a one-line CLAUDE.md that imports AGENTS.md, plus moving shared rules into
  AGENTS.md, as a diff for the user to accept or skip.
- Hub-owned guidance (roles, room rules) is passed to seats per turn as context, not written into repos.

## 7. Share skills (Agent Skills, SKILL.md)
- Skills are folders with a `SKILL.md` (name, description, instructions, optional scripts)
  ([agentskills.io](https://agentskills.io/)). Cross-tool user locations already overlap: Cursor reads
  `~/.agents/skills`, `~/.cursor/skills`, and also `~/.claude/skills` and `~/.codex/skills`
  ([Cursor](https://cursor.com/docs/context/skills)); Muse Code reads `~/.agents/skills`, `~/.claude/skills`
  and `$CODEX_HOME/skills` ([Muse](https://dev.meta.ai/docs/muse-code/extending)); Claude Code uses
  `~/.claude/skills` ([skills](https://code.claude.com/docs/en/skills)); Kimi uses `$KIMI_CODE_HOME/skills`
  ([data](https://www.kimi.com/code/docs/en/kimi-code-cli/configuration/data-locations.html)).
- Plan: Devagotchi publishes its skills to `~/.agents/skills/devagotchi-*`. For a client that does not read
  that path, we offer a symlink (macOS/Linux) or a managed copy (Windows, where symlinks may need Developer
  Mode), tracked in `skill_links(machine_id, client_install_id, skill_id, method symlink|copy, target_path,
  content_hash)` so updates and removal are clean. Codex's app-server can take extra skill roots per call
  (`skills/list` with `perCwdExtraUserRoots`, [app-server](https://learn.chatgpt.com/docs/app-server)), so
  Codex seats need no copy.
- Imports go the other way too: skills found in other clients' folders appear in our catalog as read-only
  references, and the user can adopt one into the shared folder.

## 8. Sessions from other tools
Some clients already import each other's history (`grok import` reads Claude Code sessions,
[CLI](https://docs.x.ai/build/cli/reference); Antigravity migrates Gemini CLI skills, MCP servers and
configs, [migration](https://antigravity.google/docs/cli/gcli-migration/)). Devagotchi does the same into the
hub through the read surfaces in [32](32-provider-local-state.md), so a room can start from a past session.
