# 25. Desktop Commander lessons learned

[Index](../devagotchi-spec.md) · Mined from DC's public GitHub issues on 2026-10-18 (API sorted by comments, plus
title searches for windows/timeout/security/blocked/hang). Issue titles are quoted as filed; we did not reproduce each
one. GitHub Discussions were not mined (needs an authenticated API). `#N` =
`https://github.com/wonderwhy-er/DesktopCommanderMCP/issues/N`. "Ours" points to where Devagotchi handles it;
**plan** means not built yet.

## 1. Startup and cold start
| DC issues | What happened | Ours |
|---|---|---|
| #635, #715, #510, #645 | Initialize took 24–26 s; 25–90 s from eager-loading exceljs/pdf-lib/puppeteer; feature-flag network refresh blocked the handshake; slow initialize hit a shared-pool timeout | Static Go binary, 7.9 ms spawn→initialize median ([17](17-standalone-release.md)); no network in `initialize` (rule: no I/O except the config dir before the first reply) |
| #655, #807, #475 | `npx @latest` reinstalls on every start; npx install fails on Windows; binary path mismatch falls back to npx | No npx; installer puts one binary at a fixed path; MCPB/plugins point at that path ([19](19-public-product-spec.md)) |
| #796, #451 | stdio server never answers initialize on Windows; hang on first call of a second session | Parity tests over stdio ([16](16-dc-parity.md)); **plan**: run the parity suite on Windows (gap G-07) |
| #177 | Logs printed to stdout broke JSON-RPC | Rule: stdout is protocol only, logs go to stderr/file; **plan**: a test that fails on any non-JSON stdout line |

## 2. Timeouts and honesty
| DC issues | What happened | Ours |
|---|---|---|
| #447, #671 | `timeout_ms` silently capped; schema requires `timeout_ms` but the description does not say so | T rule: caps reported in the result, never applied silently ([16](16-dc-parity.md)); schema and description generated from one source |
| #711, #837, #839 | Remote results arrived 15–38 s late; heartbeat drops; tool calls time out while "Online" | Honest presence: Online only after a verified exec round-trip ([15](15-live-ops.md)) |
| #661, #798, #633 | Dashboard says Online, tools say offline / "Not connected" | Same: one presence source, measured, shown with its age |

## 3. Security model
| DC issues | What happened | Ours |
|---|---|---|
| #122, #218, #422, #497, #548/#552 | Blocklist bypassed via `git rm`, absolute paths, newline, process substitution, `interact_with_process` | No blocklist as a boundary: OS sandbox + capabilities + argv grants (X03) |
| #219, #420, #403 | Symlink traversal; path traversal with default config | `policy.Gate` resolves symlinks; no roots = refuse all (fail closed) |
| #419 | Config fails open | Rule: fail closed on any policy parse error; **plan**: add an explicit test |
| #384, #180, #682 | Behavioural instructions inside tool descriptions; "context pollution"; ChatGPT flags imperative descriptions as suspicious | Rule: factual descriptions, no imperatives; no injected onboarding messages ([16](16-dc-parity.md)); **plan**: lint tool descriptions |
| #416 | Hard-coded credentials in source | Secrets never in code or presets; secret-pattern lint on guides and presets |
| #375 | ReDoS in Excel/DOCX search | Go `regexp` is RE2 (linear time) ([pkg regexp](https://pkg.go.dev/regexp)); plus size caps |
| #507 | No CSP on the HTML preview iframe | No HTML widgets yet; when MCP Apps land, CSP is required ([21](21-a2ui.md)) |
| #83 | Telemetry concerns | No telemetry at all (N rule) |

## 4. Command-parser false positives
| DC issues | What happened | Ours |
|---|---|---|
| #731, #795, #799, #834 | PowerShell here-strings, Python heredocs, SQL `FORMAT`, hashtables flagged as blocked commands | We do not parse shell text for safety; argv form plus sandbox means no text heuristics to misfire |
| #712 | Non-confirmable safety blocks with no diagnosis | Every denial is `grant_required`/`policy_denied` and names the human command that would allow it |

## 5. Windows specifics
| DC issues | What happened | Ours |
|---|---|---|
| #702 | Console-subsystem exes (ssh.exe) fail silently (exit 255) | ConPTY profiles ([16](16-dc-parity.md) R20); **plan**: Windows run of the PTY tests (G-07) |
| #631 | `read_file` leaks a ReadStream fd, blocking moves on Windows | Files closed on every return path; **plan**: Windows handle-leak test (G-07) |
| #841 | User env vars not inherited by launched Python | Environment is a shell-profile row (data), shown in `get_config`; **plan**: Windows test |
| #833 | spawn EPERM | **Plan**: errors carry the OS error plus a hint id (data) |
| #477 | Update path unclear on Windows | Signed `devagotchi update` with notify default ([19](19-public-product-spec.md)) |

## 6. Release and update discipline
| DC issues | What happened | Ours |
|---|---|---|
| #327 | Beta auto-pushed to stable users through Claude Extensions | Channels are explicit; a host-managed install never self-updates; stable only from the stable index |
| #157 | Regression fixed by pinning an older version | `update --rollback` keeps one `.old`; parity suite gates every release |
| #325 | Protocol version mismatch with Claude Desktop | Official MCP Go SDK; protocol version negotiated and logged; test against each client's version list |

## 7. Remote mode and auth (DC "Remote MCP")
| DC issues | What happened | Ours |
|---|---|---|
| #811, #652, #853, #838 | Connection-pool exhaustion, Realtime transport failures, join rate limits | Bridge uses one outbound HTTPS/WSS to our hub with backoff ([03](03-bridge-protocol.md)); **plan**: soak test |
| #600, #622, #698, #738, #785, #812 | OAuth/device flow failures, a resource-parameter trailing-slash mismatch | Phase 7 OAuth: conformance tests for RFC 9728/8707 with exact `resource` matching, tested from several clients |
| #697 | A second DC instance corrupts a shared state file | **Plan**: one writer per state file (lock + atomic rename), tested with two instances |
| #793 | Paid plan not recognised | No plan checks in the local binary |

## 8. Output size and token use
| DC issues | What happened | Ours |
|---|---|---|
| #82, #112, #584, #256 | Responses too long; long files throw; history output noise | Line windows and byte caps as data; Anthropic asks servers to be token-frugal ([policy](https://support.claude.com/en/articles/13145358-anthropic-software-directory-policy)) |
| #291, #681 | ChatGPT gets only metadata from `read_multiple_files`; `edit_block` exposed without params | Structured content plus text fallback; explicit schemas for every field ([16](16-dc-parity.md)); **plan**: ChatGPT client test once C9 exists |
