# 3. Bridge protocol
[Index](../devagotchi-spec.md) · Transport: the bridge opens `wss://HUB/bridge/v1` (falling back to `POST /bridge/v1/poll` long-poll + `POST /bridge/v1/result`). Auth is `Authorization: Bearer ahk_…` for the machine principal. Messages are JSON, one per frame, with `v:1`.

## Enrollment
1. A human clicks **Machines → Add machine** and picks a label, OS, and shell profiles. The hub creates principal `machine:<id>` and a one-time **enrollment code** (10 min, single use, stored hashed).
2. The user runs `devagotchi-bridge enroll --hub HUB` on the machine and pastes the code at the prompt (not as an argv flag, so it stays out of shell history). The bridge sends the code plus OS/arch/version/sandbox self-test results, and receives a long-lived machine token that it writes to `token` (0600). The token is never echoed.
3. The hub records `machine.enrolled` (code prefix only). If the sandbox self-test fails, the machine is enrolled with `sandbox_kind=none` and every grant on it is capped at decision D1.

## Frames
Hub → bridge `call`:
```json
{"v":1,"type":"call","call_id":"uuid","grant_id":"uuid","grant_version":7,"fence":4182,
 "op":"exec","args":{"argv":["git","status"],"cwd":"~/code/x","timeout_s":120,"env":{}},
 "deadline":"2026-10-17T23:10:00Z"}
```
Bridge → hub: `ack` (call accepted, sandbox built), then `chunk` (`stream: stdout|stderr`, seq, ≤64 KiB, capped at `exec_output_cap_bytes`), then `result` `{call_id, ok, exit_code|error, duration_ms, out_sha256, truncated}`.
Control: `heartbeat` (proc leases), `cancel{call_id}`, `revoke{grant_id}` (pushed on revoke; the bridge kills running calls under that grant), and `ping`/`pong` (sets `connected` only).

## Per-call checks (in this order, fail closed)
1. Hub: is the caller's token valid, does policy allow (`exec`, `fs.*`), and is there a live grant matching machine + capability + selector + sandbox level (not revoked, not expired, uses < max, task lease still held if `duration=task`)? A check that fails returns `grant_required` with a prefilled `request_access` draft.
2. Hub signs the call (HMAC with a per-machine key derived at enrollment) over `call_id, grant_id, grant_version, op, args_sha256, deadline`.
3. Bridge: verifies the signature and deadline, then re-fetches the grant state if its cache is older than `grant_cache_seconds`, so a revocation is seen within that window. It builds the sandbox from the grant's selector, not from call args.
4. Bridge: executes. Result → hub → `events` (`exec.ran` with argv fingerprint + exit code, never output bodies) → MCP result to the caller.

## Ops
- `exec{argv, cwd, env, timeout_s, shell_profile?, stdin?}`: one-shot. The timeout is honoured up to `exec_timeout_max_seconds` and documented in the tool schema (unlike DC [#447](https://github.com/wonderwhy-er/DesktopCommanderMCP/issues/447), [#671](https://github.com/wonderwhy-er/DesktopCommanderMCP/issues/671)).
- `proc_start{argv,…, ttl_s}` → `proc_lease{id, fence}`; `proc_write{id, fence, data}`, `proc_read{id, after_seq}`, `proc_heartbeat`, `proc_kill`. A lease that expires means the reaper sends `cancel` and the bridge kills the tree. `interact` goes through the same grant check (DC [#552](https://github.com/wonderwhy-er/DesktopCommanderMCP/issues/552)).
- `fs_read{path, offset, limit}` → `{content|base64, sha256, size, mtime}`; `fs_list`; `fs_stat`.
- `fs_write{path, content, expect_sha256|expect_absent}` → `{sha256}`; mismatch → `hash_mismatch` with the current hash.
- `fs_edit{path, expect_sha256, edits:[{old, new, count?}]}`: every `old` must match exactly `count` (default 1) times, or nothing is written (`edit_no_match` / `edit_ambiguous`).
- `fs_move`, `fs_delete`: `fs.write` capability, delete only with `expect_sha256`.

## Presence
- `connected`: socket up / poll seen in the last `2×bridge_poll_seconds`.
- `online`: `last_exec_ok_at` within `online_window_seconds`. An idle machine shows "connected, not verified". The UI's **Test** button runs the `probe` op (an argv `["devagotchi-bridge","--probe"]` inside the sandbox, granted implicitly) to verify it. This fixes DC "shows Online while calls fail" ([#661](https://github.com/wonderwhy-er/DesktopCommanderMCP/issues/661)).
- `last_error` shows the last failure in plain text.

## Errors (MCP `isError`, same envelope as REST)
`grant_required`, `grant_revoked`, `grant_expired`, `machine_offline`, `machine_busy`, `sandbox_unavailable`, `hash_mismatch`, `edit_no_match`, `edit_ambiguous`, `timeout`, `output_truncated` (not an error, a flag), `stale_fence`, `policy_denied`.
