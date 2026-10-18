# 16. Desktop Commander parity

What this covers: every tool and feature of Desktop Commander (DC), the Devagotchi tool that replaces it, and what
is better about ours. The same `devagotchi` binary serves these tools in two modes. `devagotchi mcp` is a local MCP
server (stdio or streamable HTTP) for Claude Desktop, Claude Code, Codex, Cursor and others. `devagotchi connect`
is the hub agent; there the hub calls the same tools through the signed `tool` op (`machine_tool` on the hub) and
the `proc.*` lease ops. Install, client config and benchmarks are in [17](17-standalone-release.md).

**Sources.** Repo github.com/wonderwhy-er/DesktopCommanderMCP at commit f508e52 (2026-10-14), npm package 0.3.0:
- `src/server.ts`: the tools/list handler (lines 315-1122) and dispatch (track_ui_event at line 1305).
- `src/tools/schemas.ts`: argument schemas. `src/config.ts:23`: `MAX_PROCESS_WAIT_MS = 50000`.
- README: features, telemetry, Docker, remote device. Issues #635 (cold start), #447 (silent timeout cap),
  #552 (interact bypasses blocklist), #419 (config fails open).

**Test.** The `Row` IDs match subtests in `bridge/parity/*_test.go`. They run against the built binary through the
official MCP Go SDK client: R/X rows over stdio, P5 over HTTP. Every row passed on Linux on 2026-10-18.

## What every row gets

- **G (grant-checked).** Every path goes through `policy.Gate`: symlinks are resolved, roots are case-folded on
  macOS and Windows, and client MCP roots can only narrow access. Errors are `grant_required` or `policy_denied`
  and name the human command that would allow the call.
- **S (sandboxed).** Processes run under bubblewrap (Linux) or Seatbelt (macOS), limited to rw/ro roots, with
  egress only through the proxy allowlist.
- **H (hash-checked).** `expect_sha256` is accepted on write_file, edit_block, move_file and delete_file. Writes
  are atomic and re-read to verify.
- **T (honest timeouts).** A timeout above the cap is reported in the result, never applied silently.
- **N (no telemetry).** Usage stats stay on this machine. **C (config protected).** The model cannot change the
  policy.

## Matrix

| Row | DC tool | Devagotchi | Improvement |
|---|---|---|---|
| R01 | get_config | get_config | Effective roots, grants, limits, sandbox kind; no secrets; `telemetry: false` |
| R02 | set_config_value | set_config_value (always denied) | C: DC lets the model rewrite allowedDirectories/blockedCommands; we return the `devagotchi config set` line for a human |
| R03 | read_file (offset/length, negative tail) | read_file | G; line windows plus sha256 for later H edits |
| R04 | read_file isUrl | read_file isUrl | Needs net.fetch plus url_hosts; each redirect hop is checked; private IPs refused unless named; 30 s/10 MiB caps are data |
| R05 | read_file image | read_file image | Image content block; size-capped |
| R08/R09/R10 | read_file Excel/DOCX/PDF | read_file sheet/range, DOCX outline, PDF pages+images | Pure Go, no Chrome download (DC #797) |
| R06 | read_multiple_files | read_multiple_files | Per-file errors; no all-or-nothing failure |
| R07 | write_file rewrite/append | write_file | H, atomic, `expect_absent`; DC's 50-line chunk advice is a note, not a cap |
| R08/R09 | write_file Excel/DOCX | write_file .xlsx (JSON grid, `=` formulas) / .docx (markdown) | Standard library only |
| R10 | write_pdf create/modify | write_pdf | Markdown→PDF with no browser; modify writes a new outputPath and refuses to overwrite by default |
| R11 | create_directory | create_directory | G |
| R12 | list_directory depth, [DENIED] | list_directory | Same markers; nested cap `nested_dir_limit` is data |
| R13 | move_file | move_file | G on both ends; refuses to overwrite unless asked; works across filesystems |
| R14 | get_file_info | get_file_info | Adds sha256, birth time (statx), lineCount |
| R15 | edit_block text/Excel/DOCX + near-miss diff | edit_block | Exact match only, with `expected_replacements` and a character-level near-miss diff; H; CRLF-aware |
| R16 | start_search files/content | start_search | Built-in walker (no ripgrep download); .gitignore/.ignore; Excel cells; first page in the reply |
| R17 | get_more_search_results | get_more_search_results | Negative offset gives the tail |
| R18 | list_searches | list_searches | — |
| R19 | stop_search | stop_search | Results kept 5 minutes |
| R20 | start_process | start_process | S; argv form (no shell parsing); PTY profiles, ConPTY on Windows; T (DC clamps silently to 50 s) |
| R21 | interact_with_process | interact_with_process | Prompt detection from data patterns; same grant as start (DC #552) |
| R22 | read_process_output | read_process_output | Line windows (DC semantics); state `waiting_for_input` |
| R23 | list_sessions | list_sessions | — |
| R24 | force_terminate | force_terminate | Kills the whole tree (process group / Job Object) |
| R25 | list_processes | list_processes | Needs proc.list; Windows has no CPU/RSS sampling (gap, below) |
| R26 | kill_process | kill_process | Own sessions need proc.interactive; foreign pids need proc.kill; pid ≤ 1, self and parent refused |
| R27 | get_usage_stats | get_usage_stats | N: local file only |
| R28 | get_recent_tool_calls | get_recent_tool_calls | Redacted, output-capped, 0600 |
| R29 | give_feedback_to_desktop_commander | same name | Returns `feedback_url`; opens and sends nothing |
| X01 | — | delete_file | Requires expect_sha256 |
| X02 | — | run_command | One-shot argv with stdout/stderr caps; killed on timeout and the result says so |
| X03 | — (DC blocklist) | sandbox | A write outside the roots fails at the OS level, whatever the command |
| X04 | start_process node:local | run_code | python/node/R runners are policy data; no temp file |
| P1 | tools/list | 28 tools with annotations | readOnly/destructive/openWorld hints; structured content |
| P2 | prompts (empty since get_prompts was removed) | `devagotchi_onboarding` + `prompts/*.md` | Prompts are data in the config dir |
| P3 | MCP-UI resources | `devagotchi://config`, `devagotchi://history/recent` | Plain JSON resources |
| P4 | allowedDirectories | policy roots ∩ client MCP roots | MRTR roots on protocol 2026-07-28; ListRoots on older protocols |
| P5 | HTTP via remote device | `devagotchi mcp --http` | Loopback by default, bearer token file (0600, never printed), DNS-rebinding guard |

## Features beyond the tool list

| DC feature | Devagotchi |
|---|---|
| Telemetry on by default (track_ui_event, capture) | None. Not ported; there is no setting because there is nothing to send |
| blockedCommands blocklist | Replaced by the OS sandbox plus capabilities (X03); in hub mode, argv prefixes from the grant |
| Docker isolation mode | OS sandbox by default; Docker optional ([13](13-docker-setup.md)) |
| Remote device | Hub mode: `devagotchi connect`, signed calls, revocable grants ([03](03-bridge-protocol.md)) |
| Onboarding, `--no-onboarding` | Onboarding prompt only on request; no injected messages |
| fileReadLineLimit / fileWriteLineLimit | `[limits]` in policy.toml (all limits are data) |
| npx cold start | Single static binary; see [17](17-standalone-release.md) |

## Gaps left (deliberate or not built)

1. **MCP-UI HTML widgets** (file preview, config editor): not ported. Plain text, image and JSON resources only.
2. **.xls (BIFF) reading**: refused with a clear error. .xlsx/.xlsm work.
3. **PDF styling**: no HTML/CSS/SVG. Core fonts cover Windows-1252, so other characters print as `?`.
4. **DOCX create**: no embedded images.
5. **Windows sandbox**: the restricted-token or AppContainer sandbox is not built, so `sandbox: none`. Processes
   are refused unless the policy says `unsandboxed`. File tools are still grant-checked.
6. **Windows list_processes**: CPU and RSS are reported as 0.
7. **macOS and Windows** were built (cross-compiled and vetted) but not run in this pass.
8. **track_ui_event**: dropped, because it is UI telemetry.
