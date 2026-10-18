# 4. Access flow (the AI as SME guides the operator)
[Index](../devagotchi-spec.md) · **SME** means the AI acts as the subject-matter expert: it explains what it needs, why, and the risks, and walks the operator through each decision and step, usually by following a guide ([12](12-guides.md)). It does **not** mean the operator is non-technical (D10, resolved). Copy is plain language first with full technical detail one click away, so it suits both a newcomer and an engineer. Precedents: Android rationale-before-ask and graceful denial ([Android](https://developer.android.com/training/permissions/requesting)); a granted scope can be narrower than the requested one ([RFC 6749 §3.3](https://datatracker.ietf.org/doc/html/rfc6749#section-3.3)); deny-by-default grants ([Tailscale grants](https://tailscale.com/kb/1324/grants)); accept/decline/cancel and no secrets in prompts ([MCP elicitation](https://modelcontextprotocol.io/specification/2025-06-18/client/elicitation)). The combined flow below is **inference** from those sources.

## Steps
1. **Agent asks.** It calls `request_access{machine, capability, selector, sandbox_level, duration, reason, task_id?, guide_run_id?, preset?}`. `reason` is required (≥20 chars) and shown verbatim, labelled "the agent says". If `preset` is given, the request expands to that preset's items, each still shown. The hub attaches the matching `risks` rows as a `risk_snapshot`, so later edits to the risk catalog do not rewrite history.
2. **Operator is told.** The request goes out over SSE to the UI (a badge, an optional browser notification, and a card on the phone). If the client supports MCP elicitation, the hub may also elicit, but the hub card is the source of truth (D3). The agent's call returns `pending` with `request_id`; the agent waits with `wait_for_access` (long-poll ≤25 s) or keeps explaining the next guide step.
3. **Card shows, in this order:** who is asking (agent + chat app + subscription), for which task or guide step (link), *what* in plain words ("Run commands in ~/code/shop, no internet, until this task ends"), *why* (agent's reason), *risks* (each with severity, plain text, and mitigation), and the technical detail collapsed (capability, selector JSON, sandbox level, preset).
4. **Operator decides:** **Approve**, **Approve narrower** (edit paths to a subfolder, command prefix, hosts, apps/windows for GUI, sandbox level down, duration shorter, uses max), **Deny** (optional note to the agent), or **Not now** (pending until `request_expiry_minutes`). Widening beyond the request is not allowed; the agent must ask again. Approving needs a human principal + policy `grant.approve` (01 invariant 1).
5. **Grant row created**, with `access_request.status = approved|narrowed`. The agent's wait returns the *granted* scope so it can adapt. Denied → the agent gets `access_denied` + note and degrades gracefully (Android precedent), e.g. by switching the guide to "operator does it" steps.
6. **Persistent reminder.** While any grant is live, a header chip shows "3 live grants · 1 high risk · GUI active". It cannot be dismissed, only cleared by grants ending. The machine page and Grants page list each grant with risks and a countdown. `unsandboxed`, `gui.*`, full-access, or high-severity grants also show a banner (`reminder_style`).
7. **One-click revoke.** Every grant row and the chip menu have **Revoke** (confirmation optional via `revoke_confirm`, default off, since revoking is safe). Revoke sets `revoked_at`, pushes `revoke{grant_id}` to the bridge, kills calls, and ends GUI sessions under it. Bulk **Revoke all on this machine** and **Pause machine** (sets `disabled`) are also available, as is the on-machine GUI stop hotkey ([11](11-gui-control.md)).
8. **Expiry.** `once` = consumed by first use; `task` = ends when the task's lease ends (complete, release, expiry, or force-release); `Nh` = wall-clock; `until_revoked` = needs policy allow `grant.indefinite` (preset: off for agents of kind `cli`/`service`). The reaper ends grants and appends `grant.expired`.
9. **Audit.** Hash-chained events: `access.requested|approved|narrowed|denied|cancelled|expired`, `grant.used` (one per call: op + argv/path fingerprint + exit code; GUI actions per [11](11-gui-control.md#audit)), `grant.revoked|expired`, `machine.paused`, `policy.unsandboxed.enabled|disabled`, `preset.default_changed`, `resume.alert_shown|acknowledged`. The Audit page filters by grant, machine, agent, and guide run.

## Grant length and presets (D4, resolved)
- The default duration is `task` (dies with the lease), via the default preset `task-scoped`. The default is **configurable**: Settings → Access → *Default preset* picks any `grant_presets` row, and each preset's duration is editable. Changing it appends `preset.default_changed`.
- A preset fills the card; the operator can still narrow every item. A preset never skips the card.
- **`experimental-full-access`** may be set as the default only if the unsandboxed policy is enabled (it has `requires_policy=unsandboxed`). Selecting it shows the full risk list and needs typed confirmation of the preset name.

## Full access and resume alerts
Applies to every live grant that is `unsandboxed` or came from a `full_access` preset (01 invariant 7):
- **Every time a session is resumed** the operator sees a blocking alert dialog (`alertdialog`) listing the machines, scope, age, and risks, with **Continue**, **Revoke now**, and **Pause machine**. "Resumed" means: a UI sign-in, any page load or reload, a tab or phone app returning after `resume_alert_idle_minutes` hidden or idle, and every agent session resuming under that grant (a new MCP session, reconnect, or task claim by that principal). Agent-side resumes alert the human through SSE and push; the agent's call proceeds only while the grant is live.
- Each alert writes a `resume_acks` row and `resume.alert_shown` / `resume.acknowledged` events. The alert cannot be disabled; only its wording is data.
- The persistent banner stays on every screen while such a grant is live.

## Unsandboxed policy (D1, resolved)
- Off by default: no `sandbox_policies` row means the `unsandboxed` level is hidden from cards and refused by the API (`policy_denied`).
- A human enables it in Settings → Access → *Unsandboxed access*, for the tenant or one machine, optionally with an expiry. The screen shows each risk row in plain text (e.g. "commands can read and change any file your account can, including other projects and keys") and requires ticking each one; the ticked ids are stored in `acknowledged_risk_ids`.
- Enabling and disabling are audited (`policy.unsandboxed.*`, with who, scope, and risks acknowledged). While enabled, the header shows a persistent "Unsandboxed allowed" marker even with no live grants. Each unsandboxed grant still goes through the card.

## Selector shape (data, validated by `capabilities.selector_schema`)
```json
{"paths":["~/code/shop"],"argv_prefix":[["npm","test"],["git"]],
 "hosts":["registry.npmjs.org"],"env_allow":["CI"],"max_runtime_s":600}
```
Matching: the path is canonicalised on the bridge (realpath, case-fold rules per OS) **and** enforced by the sandbox. The argv prefix is compared element-wise, never as a string. GUI selectors are in [11](11-gui-control.md#grant-scopes).

## Rules
- The model can never create, widen, or extend a grant, change the default preset, or enable the unsandboxed policy; no MCP tool writes those. Fixes DC `set_config_value`.
- Requests never ask for secrets; a secret goes in through the Subscriptions screen only (MCP elicitation rule). Guides never ask the operator to paste a secret into chat.
- Risk text is data (`risks`), with presets written in plain English and editable per tenant.
- Repeated requests: after a denial, a new request for the same capability and selector within `deny_cooldown_minutes` (preset 10) is shown collapsed ("asked again") to avoid prompt fatigue.
