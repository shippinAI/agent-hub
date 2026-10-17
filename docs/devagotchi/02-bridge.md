# 2. Bridge daemon (`devagotchi-bridge`)
[Index](../devagotchi-spec.md) · One per machine (Windows, macOS, Linux). It is a hub principal of kind `machine`. AI clients never talk to it directly: they call the hub's `/mcp`, and the hub routes the call to the machine (research, "Recommended architecture").

## Improvements over Desktop Commander (DC)
| Requirement | Design | DC failure it answers |
|---|---|---|
| Outbound-only | The bridge dials the hub (WSS, falling back to HTTPS long-poll ≤`bridge_poll_seconds`). It opens no listening socket and no inbound ports, so it works behind NAT. | Remote presence/connect issues [#661](https://github.com/wonderwhy-er/DesktopCommanderMCP/issues/661), [#811](https://github.com/wonderwhy-er/DesktopCommanderMCP/issues/811) |
| OS-enforced sandbox | Every exec and fs call runs inside the OS sandbox for the grant's `sandbox_level`: Linux bubblewrap, macOS Seatbelt (`sandbox-exec` profile generated from the grant), Windows a dedicated low-privilege local user + ACLs + firewall rules (the Codex model). Paths are enforced by the kernel, not by string checks. | Advisory `allowedDirectories` and bypasses [#791](https://github.com/wonderwhy-er/DesktopCommanderMCP/issues/791), [#403](https://github.com/wonderwhy-er/DesktopCommanderMCP/issues/403), [#552](https://github.com/wonderwhy-er/DesktopCommanderMCP/issues/552), [#730](https://github.com/wonderwhy-er/DesktopCommanderMCP/issues/730) ([DC README](https://github.com/wonderwhy-er/DesktopCommanderMCP#readme)) |
| Argument lists | `exec` takes `argv: string[]`. A shell is used only if the grant names a `shell_profile`, and then the script is one argv element. There is no blocklist parsing, so heredocs and here-strings cannot be misread. | False positives [#731](https://github.com/wonderwhy-er/DesktopCommanderMCP/issues/731), [#795](https://github.com/wonderwhy-er/DesktopCommanderMCP/issues/795), [#822](https://github.com/wonderwhy-er/DesktopCommanderMCP/issues/822) |
| Processes as leases | Long or interactive processes get a `proc_lease` with a fence and TTL. The agent heartbeats it, and the reaper kills orphans (the process tree / job object). | Hangs, timeout caps [#447](https://github.com/wonderwhy-er/DesktopCommanderMCP/issues/447), [#451](https://github.com/wonderwhy-er/DesktopCommanderMCP/issues/451) |
| Hash-checked atomic edits | `fs_edit` = exact-match replace with `expect_sha256` of the whole file; `fs_write` = temp file in the same dir + fsync + rename; fds are always closed (Windows `FILE_SHARE_DELETE`). It returns the new hash. Fuzzy matching is never used. | [#97](https://github.com/wonderwhy-er/DesktopCommanderMCP/issues/97), [#433](https://github.com/wonderwhy-er/DesktopCommanderMCP/issues/433) (empty files), [#631](https://github.com/wonderwhy-er/DesktopCommanderMCP/issues/631) |
| No telemetry | It makes no network calls except to the configured hub URL and no background downloads. Logs go to stderr/file only, never stdout. | [#180](https://github.com/wonderwhy-er/DesktopCommanderMCP/issues/180), [#797](https://github.com/wonderwhy-er/DesktopCommanderMCP/issues/797), [#177](https://github.com/wonderwhy-er/DesktopCommanderMCP/issues/177) |
| Honest presence | A machine is **online** only if a real call completed within `online_window_seconds`. A socket ping sets only `connected`. The UI shows both. | [#633](https://github.com/wonderwhy-er/DesktopCommanderMCP/issues/633), [#837](https://github.com/wonderwhy-er/DesktopCommanderMCP/issues/837) |
| Pinned releases | Signed static binary per OS/arch with a pinned version. Updates need user consent (no `npx @latest`), and a single-instance lock per machine. | [#655](https://github.com/wonderwhy-er/DesktopCommanderMCP/issues/655), [#327](https://github.com/wonderwhy-er/DesktopCommanderMCP/issues/327), [#697](https://github.com/wonderwhy-er/DesktopCommanderMCP/issues/697) |
| Console apps on Windows | Uses ConPTY for `pty:true` profiles, hidden windows otherwise (no focus stealing). | [#702](https://github.com/wonderwhy-er/DesktopCommanderMCP/issues/702), [#852](https://github.com/wonderwhy-er/DesktopCommanderMCP/issues/852) |
| Config out of model reach | The bridge has no "set config" tool. Grants live in the hub and are only changed by humans in the UI. | DC `set_config_value` ([README](https://github.com/wonderwhy-er/DesktopCommanderMCP#readme)) |

## Sandbox levels (data rows, mapped per OS in code)
- `read-only`: read the selector paths, no network, no writes.
- `workspace-write`: read and write inside the selector paths, network only to the grant's `net.egress` hosts.
- `unsandboxed`: exists only if `unsandboxed_allowed` is true (D1). It gets a high-severity risk row and a red chip.
Mapping is inferred from [Claude Code sandboxing](https://code.claude.com/docs/en/sandboxing) (Seatbelt; bubblewrap+socat) and [Codex sandboxing](https://developers.openai.com/codex/concepts/sandboxing) / [Codex Windows](https://developers.openai.com/codex/windows). Known limit, stated in the UI: a domain-allowlist proxy does not inspect TLS, so domain fronting can escape it (Claude docs).
**Unverified:** that macOS `sandbox-exec` stays usable on future macOS releases (it is deprecated but still used by Claude Code). A fallback is decision D9.

## Local layout (all the bridge touches)
- Linux `~/.config/devagotchi/`, macOS `~/Library/Application Support/Devagotchi/`, Windows `%ProgramData%\Devagotchi\` (service) holding `bridge.toml` (hub URL, machine id, shell profile ids, log level) and `token` (0600 / ACL owner-only).
- Logs: the same dir, `bridge.log`, rotated, with secrets redacted (`ahk_`, `sk-`, and known prefixes from the `secret_patterns` setting).
- Service: systemd **user** unit / launchd agent / Windows service, installed only by an explicit `devagotchi-bridge install` that the user runs.
- The sandbox user on Windows is created at install with user consent and removed at uninstall.

## Non-goals (v0.1)
No GUI automation (D7). No file sync. No remote desktop. No running as root/Administrator for jobs (the service may need elevation only to create the sandbox user on Windows).
