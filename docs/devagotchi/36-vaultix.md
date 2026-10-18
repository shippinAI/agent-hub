# 36. Vaultix: secrets (Infisical, rebranded) with Doppler-style additions

[Index](../devagotchi-spec.md) · Requirement (Jesse, 2026-10-18): Infisical, commercially licensed with rebranding
allowed, becomes **Vaultix**; fold in Doppler features worth having; UI-first with A2UI popups and modals rendered
by React Aria ([21](21-a2ui.md)). Replaces the open secret-store choice in D5 ([10](10-decisions.md)) for the hosted
edition. Read 2026-10-18.

## Licence note
Infisical's repo is MIT except `ee/` directories, which use `ee/LICENSE`
([LICENSE](https://github.com/Infisical/infisical/blob/main/LICENSE)); paid features such as point-in-time recovery
need an enterprise licence when self-hosted ([PIT](https://infisical.com/docs/documentation/platform/pit-recovery)).
Jesse's commercial licence covers the `ee/` parts and the rebrand. We keep upstream attribution where the MIT
part requires it and track upstream releases as a vendor branch.

## Feature comparison (what Vaultix gets from where)
| Feature | Doppler | Infisical (already) | Vaultix plan |
|---|---|---|---|
| Environments | Root configs dev/stg/prd per project ([branch configs](https://docs.doppler.com/docs/branch-configs)) | Project → environment → folder → secret ([projects](https://infisical.com/docs/documentation/platform/secrets-mgmt/project)) | Keep Infisical's tree |
| **Branch configs + promote** | Branch off a root, diverge, then promote a value to the root ([branch configs](https://docs.doppler.com/docs/branch-configs)) | Personal overrides per user only (same page) | **Add**: `branch` folders under an environment with diff and "promote to root" |
| Inheritance | Config inheritance across projects, ordered precedence, child wins ([inheritance](https://docs.doppler.com/docs/config-inheritance)) | Secret imports, ordered, bottom-most import wins; local definition overrides ([reference](https://infisical.com/docs/documentation/platform/secret-reference)) | Keep imports; **add** the Doppler-style view: struck-through overridden values and drag to reorder in one panel |
| References | `${KEY}`, `${config.KEY}`, `${project.config.KEY}`, resolved at read time ([secrets](https://docs.doppler.com/docs/secrets)) | `${KEY}`, `${env.KEY}`, folder paths (same reference page) | Keep; **add** a dangling-reference report (Doppler warns these can fill in later) |
| Syncs / integrations | Automated syncs to AWS, GitHub, Vercel, Kubernetes, … ([integrations](https://docs.doppler.com/docs/integrations)) | Secret Syncs via App Connections; Kubernetes operator ([syncs](https://infisical.com/docs/integrations/secret-syncs/overview)) | Keep Infisical's; **add** Shippin targets (hosted workspaces, hub `providers` API keys) |
| CLI run injection | `doppler run`; file mounts with max reads ([secrets](https://docs.doppler.com/docs/secrets), [accessing](https://docs.doppler.com/docs/accessing-secrets)) | `infisical run -- cmd` ([run](https://infisical.com/docs/cli/commands/run)) | Keep, rebranded `vaultix run`; **add** max-read file mounts |
| Change log / rollback | Activity log with rollback ([versioning](https://docs.doppler.com/docs/versioning)) | Secret versioning ([versioning](https://infisical.com/docs/documentation/platform/secret-versioning)); commits and PIT recovery (ee) | Keep; surface commits as a timeline with one-click revert |
| Change requests | Approval before merge ([change requests](https://docs.doppler.com/docs/change-requests)) | Approval workflows ([PR workflows](https://infisical.com/docs/documentation/platform/pr-workflows)) | Keep; route approvals to the hub Requests inbox ([04](04-access-flow.md)) |
| Rotation | Two-secret strategy, triggers webhooks and syncs ([rotation](https://docs.doppler.com/docs/secrets-rotation)) | Scheduled rotation with dual-phase overlap ([rotation](https://infisical.com/docs/documentation/platform/secret-rotation/overview)) | Keep Infisical's |
| Dynamic secrets | Leases with TTL, revoked at expiry ([dynamic](https://docs.doppler.com/docs/dynamic-secrets)) | Short-lived, auto-revoked credentials ([dynamic](https://infisical.com/docs/documentation/platform/dynamic-secrets/overview)) | Keep Infisical's |
| Access requests | Scoped service tokens with expiry ([tokens](https://docs.doppler.com/docs/service-tokens)) | Access requests with approvers, max duration, break-glass; temporary access ([requests](https://infisical.com/docs/documentation/platform/access-controls/access-requests), [temporary](https://infisical.com/docs/documentation/platform/access-controls/temporary-access)) | Keep Infisical's; same card style as hub grants |
| Read audit | Access logs per secret: who read what, when ([access logs](https://docs.doppler.com/docs/access-logs)) | Audit logs ([audit](https://infisical.com/docs/documentation/platform/audit-logs)) | **Add** a per-secret "who read this" tab built from audit events |
| Webhooks | Signed webhooks with delivery log ([webhooks](https://docs.doppler.com/docs/webhooks)) | Webhooks ([webhooks](https://infisical.com/docs/documentation/platform/webhooks)) | Keep; **add** signature + delivery log if missing upstream |
Net: Infisical already covers most of the list. The Doppler additions worth building are **branch configs with
promote**, the **inheritance conflict view**, **dangling-reference report**, **max-read mounts**, and the
**per-secret read view**.

## How Vaultix fits the hub
- **Hosted**: one Vaultix deployment per region; each Shippin tenant is a Vaultix organization; the console's
  tenant defaults ([35](35-tenant-defaults.md)) seed a project template (environments, folders, rotation defaults).
- **Local edition**: the hub keeps its built-in `secrets` table (D5: local key + pgcrypto) behind the same
  `SecretStore` interface; Vaultix is an optional backend the operator can point at.
- The hub stores only **references** (`vaultix://org/project/env/path#KEY`) and fingerprints, never values
  (invariant 4, [01](01-data-model.md)). API-key provider rows ([34](34-provider-schema.md)) use `secret_ref`.
- Machines and hosted workspaces get secrets at run time via `vaultix run` with a short-lived machine identity
  issued per workspace; agents never see values in transcripts (the bridge redacts known fingerprints).
- **Subscription sign-ins** ([37](37-subscription-sign-in.md)): OAuth tokens our own app obtains (Sign in with
  ChatGPT plan usage) are stored in Vaultix under the user's personal scope; official-client logins stay in the
  client's own home.

## UI-first (A2UI surfaces, React Aria rendering)
Every action has a UI; the CLI and API mirror it. Agents trigger surfaces; humans act in them.
| Surface (A2UI component → RAC) | When |
|---|---|
| **Add secret** popover (`Popover` + `TextField` masked, environment `CheckboxGroup`) | From any "missing secret" warning, e.g. a provider row with no key |
| **Reveal** (`Dialog`, re-auth, auto-hide timer, logs a read) | Viewing a value |
| **Promote branch value** (`Modal` with side-by-side diff, `Table`) | Branch differs from root |
| **Inheritance conflicts** (`Popover` listing sources in order, drag to reorder via `GridList`) | Hover on a struck-through key |
| **Access request** card (`Dialog`: scope, duration `Select`, reason; approver sees the same card) | Agent or user needs access |
| **Rotation now** (`AlertDialog` with affected syncs and consumers) | Manual rotate |
| **Sync status** `Tooltip` per target (last run, error) | Sync table |
Agent-initiated A2UI surfaces may **ask** for a value but never receive it: the value goes from the user's input
straight to Vaultix, and the agent gets back the reference (data binding writes to a server-side sink, [21](21-a2ui.md)).

## Work items (to the gap register)
Rebrand build of Infisical (logo, names, domains, emails); `SecretStore` interface with `builtin` and `vaultix`
backends; the five Doppler-style additions; A2UI surfaces above; D5 update for hosted.
