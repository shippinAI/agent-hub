# 17. Standalone mode, release and benchmarks

There is one binary, `devagotchi`, per OS and CPU. The subcommand picks the role:

| Command | What it does |
|---|---|
| `devagotchi mcp [--root DIR]... [DIR...]` | Local MCP server on stdio, for desktop AI clients |
| `devagotchi mcp --http [--addr 127.0.0.1:7676]` | Streamable HTTP at `/mcp`; bearer token in `<config-dir>/http-token` (0600, never printed) |
| `devagotchi connect` (alias `run`) | Hub agent: signed calls under revocable grants ([03](03-bridge-protocol.md)) |
| `devagotchi enroll` | Enroll with a hub (the code is read from stdin) |
| `devagotchi doctor` | Self-check: OS, policy, config-dir permissions, a real sandbox in/out write test, PTY, process list, hub status |
| `devagotchi version` | Version, commit, build date, os/arch, Go version |
| `devagotchi config path\|show\|init --root DIR\|set KEY VALUE` | Human-only policy editing (`policy.toml`) |
| `devagotchi grant list\|add --cap C [--path P] [--host H] [--ttl 8h]\|revoke ID` | Human-only local grants |
| `devagotchi install\|uninstall --mode connect\|mcp-http [--enable\|--disable]` | Per-user service: systemd user unit, LaunchAgent, or a logon scheduled task. No root or admin |

Config dir: `~/.config/devagotchi` (Linux), `~/Library/Application Support/Devagotchi` (macOS), or
`%APPDATA%\Devagotchi` (Windows). Override it with `DEVAGOTCHI_CONFIG_DIR`.

With no roots, every path is refused (fail closed). Windows has no OS sandbox yet ([16](16-dc-parity.md) gap 5), so
processes there need `devagotchi config set sandbox_level unsandboxed`. That is a deliberate, human-made choice.

## Install (one line per OS; per-user, no sudo; verifies SHA256SUMS)

```sh
curl -fsSL "$BASE/install.sh" | DEVAGOTCHI_BASE_URL="$BASE" sh        # Linux, macOS -> ~/.local/bin
```
```powershell
$env:DEVAGOTCHI_BASE_URL="$BASE"; irm "$BASE/install.ps1" | iex          # Windows -> %LOCALAPPDATA%\Programs\Devagotchi
```

`$BASE` is wherever `dist/release/` is served. To install offline from a local copy, use `sh install.sh --from DIR`
or `install.ps1 -From DIR`.

## Client config

Claude Desktop (`claude_desktop_config.json`) and Cursor (`~/.cursor/mcp.json`):
```json
{ "mcpServers": { "devagotchi": { "command": "/home/you/.local/bin/devagotchi",
  "args": ["mcp", "--root", "/home/you/code"] } } }
```
On Windows, set `"command"` to `"C:\\Users\\you\\AppData\\Local\\Programs\\Devagotchi\\devagotchi.exe"`.

Claude Code:
```sh
claude mcp add --scope user devagotchi -- devagotchi mcp --root ~/code
claude mcp add --transport http devagotchi http://127.0.0.1:7676/mcp \
  --header "Authorization: Bearer $(cat ~/.config/devagotchi/http-token)"     # HTTP mode
```

Codex (`~/.codex/config.toml`). Codex's default `tool_timeout_sec` is 60, so raise it if you use long `timeout_ms`:
```toml
[mcp_servers.devagotchi]
command = "devagotchi"
args = ["mcp", "--root", "/home/you/code"]
tool_timeout_sec = 600
# HTTP instead: url = "http://127.0.0.1:7676/mcp"  and  bearer_token_env_var = "DEVAGOTCHI_TOKEN"
```

Cursor in HTTP mode: `{"mcpServers":{"devagotchi":{"url":"http://127.0.0.1:7676/mcp","headers":{"Authorization":"Bearer ${env:DEVAGOTCHI_TOKEN}"}}}}`.

## Release build

Run `scripts/release.sh`. It sets `CGO_ENABLED=0`, `-trimpath`, and `-ldflags "-s -w -buildid= -X buildinfo.Version/Commit/Date"`,
and writes `dist/release/` with `SHA256SUMS` and `sizes.txt`. Targets are data (`TARGETS=...`).

Sizes for 0.2.0, in bytes (gzip -9 in brackets): linux/amd64 16,969,852 (6.6 MB); linux/arm64 15,597,692 (5.9 MB);
darwin/amd64 16,946,864 (6.6 MB); darwin/arm64 15,629,970 (6.0 MB); windows/amd64 17,058,304 (6.7 MB);
windows/arm64 15,439,872 (5.9 MB). Most of the size comes from the MCP SDK and pdfcpu (PDF page edits). For
comparison, DC 0.3.0 installs 672 npm packages totalling 259 MB of node_modules and needs Node.js.

## Benchmarks (2026-10-18 at about 02:05 EDT, 8-vCPU Xeon box, Linux)

Method:
- Both servers were driven by the same official TypeScript MCP client (`tests/mcp-node/bench.mjs`, SDK 1.32.1).
- Each server had 20 cold spawns, then 5 runs of each operation; the tables show medians.
- The corpus was a 20,000-line file plus 1,000 small files.
- DC 0.3.0 was installed with `npm --ignore-scripts`, using system ripgrep. It ran with telemetry off, under
  `bwrap --unshare-net`, so nothing could leave the box.

| Measure | devagotchi 0.2.0 | Desktop Commander 0.3.0 |
|---|---|---|
| Spawn → initialize done | **7.9 ms** (max 20.6) | 399 ms (max 442); #635 reports 24–26 s via `npx` cold |
| tools/list | 1.7 ms (28 tools) | 9.2 ms (25 tools) |
| read_file 1000 lines | 2.5 ms | 23 ms |
| list_directory (depth 2) | 1.6 ms | 12.1 ms |
| start_search content (1,000 files) | 21.7 ms, complete | 13 ms, still running at reply (ripgrep) |
| start_process `echo hi` | 13.2 ms (inside bubblewrap) | 8.8 ms (no sandbox) |
| start_process `sleep 75; echo done`, timeout_ms 80000 | **75.0 s, finished, output "done"** | 50.0 s, "Process is running"; silent clamp to `MAX_PROCESS_WAIT_MS = 50000` (#447) |
| `devagotchi version` / `doctor` | 3.5 ms / 24 ms | n/a |

Our search is slower than ripgrep, but it returns the complete result set in that reply. Our process start costs
about 4 ms more because each process gets its own bubblewrap sandbox.

## Tests

- `go test -race ./...` in `bridge/` covers unit tests for every package plus `tools` end to end (sandbox
  confinement, PTY prompts, timeouts).
- `bridge/parity` covers every matrix row through the official Go MCP client over stdio, plus HTTP with a token
  check.
- The benchmark run doubles as a TypeScript-client compatibility check.
- `scripts/release.sh` cross-compiles all six targets.
