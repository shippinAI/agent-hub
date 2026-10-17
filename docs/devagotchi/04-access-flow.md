# 4. Access flow (plain-language "SME" flow)
[Index](../devagotchi-spec.md) · Written for a non-technical owner (SME = small business owner / subject-matter expert, not a security engineer; the interpretation is decision D10). Precedents: Android rationale-before-ask and graceful denial ([Android](https://developer.android.com/training/permissions/requesting)); a granted scope can be narrower than the requested one ([RFC 6749 §3.3](https://datatracker.ietf.org/doc/html/rfc6749#section-3.3)); deny-by-default grants ([Tailscale grants](https://tailscale.com/kb/1324/grants)); accept/decline/cancel and no secrets in prompts ([MCP elicitation](https://modelcontextprotocol.io/specification/2025-06-18/client/elicitation)). The combined flow below is **inference** from those sources.

## Steps
1. **Agent asks.** It calls `request_access{machine, capability, selector, sandbox_level, duration, reason, task_id?}`. `reason` is required (≥20 chars) and shown verbatim, labelled "the agent says". The hub attaches the matching `risks` rows as a `risk_snapshot`, so later edits to the risk catalog do not rewrite history.
2. **User is told.** The request goes out over SSE to the UI (a badge, an optional browser notification, and a card on the phone). If the client supports MCP elicitation, the hub may also elicit, but the hub card is the source of truth (D3). The agent's call returns `pending` with `request_id`, and the agent waits with `wait_for_access` (long-poll ≤25 s) or works on something else.
3. **Card shows, in this order:** who is asking (agent + chat app + subscription), for which task (link), *what* in plain words ("Run commands in ~/code/shop, no internet, until this task ends"), *why* (agent's reason), *risks* (each with severity, plain text, and mitigation), and the technical detail collapsed (capability, selector JSON, sandbox level).
4. **User decides:** **Approve**, **Approve narrower** (edit paths to a subfolder, command prefix, hosts, sandbox level down, duration shorter, uses max), **Deny** (optional note to the agent), or **Not now** (stays pending until `request_expiry_minutes`). Widening beyond the request is not allowed; the agent must ask again. Approving needs a human principal + policy `grant.approve` (code rule 1).
5. **Grant row created**, with `access_request.status = approved|narrowed`. The agent's wait returns the *granted* scope so it can adapt. Denied → the agent gets `access_denied` + note and must degrade gracefully (Android precedent).
6. **Persistent reminder.** While any grant is live, a header chip shows "3 live grants · 1 high risk". It cannot be dismissed, only cleared by grants ending. The machine page and Grants page list each grant with risks and a countdown. A grant marked `unsandboxed` or high severity also shows a banner (`reminder_style` setting).
7. **One-click revoke.** Every grant row and the chip menu have **Revoke** (confirmation is optional via setting `revoke_confirm`, default off, since revoking is safe). Revoke sets `revoked_at`, pushes `revoke{grant_id}` to the bridge, and kills calls running under it. Bulk **Revoke all on this machine** and **Pause machine** (sets `disabled`) are also available.
8. **Expiry.** `once` = consumed by first use; `task` = ends when the task's lease ends (complete, release, expiry, or force-release); `Nh` = wall-clock; `until_revoked` = needs a policy allow `grant.indefinite` (preset: off for agents of kind `cli`/`service`). The reaper ends grants and appends `grant.expired`.
9. **Audit.** Hash-chained events: `access.requested|approved|narrowed|denied|cancelled|expired`, `grant.used` (one per call: op + argv/path fingerprint + exit code), `grant.revoked|expired`, `machine.paused`. The Audit page filters by grant, machine, and agent; each grant page shows its full use history.

## Selector shape (data, validated by `capabilities.selector_schema`)
```json
{"paths":["~/code/shop"],"argv_prefix":[["npm","test"],["git"]],
 "hosts":["registry.npmjs.org"],"env_allow":["CI"],"max_runtime_s":600}
```
Matching: the path is canonicalised on the bridge (realpath, case-fold rules per OS) **and** enforced by the sandbox. The argv prefix is compared element-wise, never as a string.

## Rules
- The model can never create, widen, or extend a grant; no MCP tool writes `grants`. Fixes DC `set_config_value`.
- Requests never ask for secrets; a secret goes in through the Subscriptions screen only (MCP elicitation rule).
- Risk text is data (`risks`), with presets written in plain English and editable per tenant.
- Repeated requests: after `deny_cooldown_minutes` (preset 10) for the same capability and selector, a new request is shown collapsed ("asked again") to avoid prompt fatigue.
