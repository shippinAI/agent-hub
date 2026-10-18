# 12. Guides: Shippin-authored, versioned, editable content
[Index](../devagotchi-spec.md) · Step-by-step guides that the AI follows as the subject-matter expert to walk the operator through granting access, enrolling machines, enabling GUI control, installing Docker, and similar tasks. Guides are **data** (rows plus a JSON step document), authored and maintained by Shippin, shipped as a signed preset bundle, editable per tenant in an admin editor, and updated through a reviewable flow. No guide text lives in code. Researched 2026-10-17; sources inline; **inference** marks our choices.

## What this touches
Hub DB (`guides`, `guide_versions`, `guide_runs`, `guide_updates`), `server/internal/presets/guides/*.json` (the seed bundle), the UI (runner, editor, updates), and MCP tools/resources/prompts ([06](06-mcp-tools.md)). A guide run touches a machine only through bridge calls under grants the operator approved.

## Writing rules (from documentation practice)
- Guides are **how-to guides**, not tutorials: goal-titled, for an operator with a real task, allowed to branch for real-world variations, and free of digressions ([Diátaxis how-to](https://diataxis.fr/how-to-guides/), [tutorial vs how-to](https://diataxis.fr/tutorials-how-to/)).
- Numbered steps, **one action per step**, imperative, with the context or condition before the action, and alternatives split into separate branches rather than mixed into a step ([Google procedures](https://developers.google.com/style/procedures)).
- Every step says **who acts** (operator or agent), **what changes**, and **how we know it worked** (a check). **Inference**: the explicit actor and check fields let the AI explain and verify instead of guessing.
- Plain language first, exact commands and settings paths second. No secrets in guide text, ever; a step that needs one links to the Subscriptions screen.

## Step kinds
| Kind | Who | Does | Example |
|---|---|---|---|
| `explain` | AI | Says why the next steps exist and the risks, from text + linked `risks` rows | "Docker needs a small Linux VM on macOS. Here is what it can access." |
| `check` | agent | Runs a probe under an existing grant, or asks the operator; branches on the result | `docker version` exit code; `gui_ready.accessibility` |
| `request_access` | agent | Calls `request_access` with the step's selector or `preset`; waits | `docker-setup` preset on this machine |
| `agent_action` | agent | One `exec` / `fs_*` / `gui_*` call, template filled from run variables | `exec ["podman","machine","init"]` |
| `operator_action` | operator | Shows exact instructions (per OS), optional copy button for a command, then **Done** / **Problem** | "Open PowerShell as administrator and run `wsl --install`, then restart." |
| `confirm` | operator | A yes/no the agent must not decide (licences, terms, purchases) | "Does your organisation qualify for free Docker Desktop use?" |
| `branch` | – | Chooses the next step by condition (`os`, `display_server`, `arch`, earlier check results, operator choice) | Desktop vs Engine vs Podman |
| `verify` | agent | Final check that the goal is met; writes the run outcome | `docker run --rm hello-world` |
| `link_guide` | – | Runs another guide as a sub-run, then returns | `enroll-machine` → `enable-gui-macos` |
Rules (01 invariant 9): steps never create grants; `agent_action` argv must fit the live grant or it fails with `grant_required` and the runner shows the `request_access` step; `operator_action` is used for anything elevated (sudo, UAC, admin PowerShell), any OS permission dialog, and any licence or terms acceptance (**inference**, following Anthropic's "ask a human to confirm" precaution, [computer use](https://platform.claude.com/docs/en/agents-and-tools/tool-use/computer-use-tool)).

## Data
- `guides(id text pk, tenant_id null (null = Shippin global), slug, title, goal, tags text[], applies_to jsonb ({os, kind, display_server, arch}), source shippin|tenant, overrides_guide_id null, current_version_id, channel stable|beta, enabled, version)`.
- `guide_versions(id uuid, guide_id, semver text, schema_version int, steps jsonb, change_notes text, risk_ids text[], authored_by, signed_by null, signature null, content_sha256, status draft|published|retired, created_at, published_at)`. Published versions are **immutable**; an edit makes a new draft.
- `guide_runs(id uuid, guide_version_id, machine_id, task_id null, started_by, agent_principal, vars jsonb (no secrets; schema forbids `secret`-typed vars), step_log jsonb ([{step_id, outcome, at, call_ids, request_ids}]), status running|paused|done|failed|abandoned, started_at, ended_at)`. A run pins its version, so an update never changes a running guide.
- `guide_updates(id uuid, guide_id, from_semver, to_semver, bundle_sha256, change_notes, status pending|accepted|skipped|merged, decided_by null)`.
- Steps JSON is validated against a JSON Schema ([2020-12](https://json-schema.org/draft/2020-12)) stored as a preset (`guide_step_schema`); `schema_version` lets old versions keep validating.

## Versioning and the update flow
1. Shippin authors guides in its own repo and publishes **signed bundles** (guides + versions + `content_sha256`). Versions use [SemVer](https://semver.org/): major = steps reordered/removed or actor changed; minor = new branch or step; patch = wording. **Inference**: the mapping is ours.
2. The hub fetches the bundle from the configured `guide_channel` URL (data) or the operator imports a file. It verifies the signature against the Shippin public key in settings; a bad signature is rejected and logged (`guide.import_rejected`).
3. New versions land in **Guide updates** as `pending`. `guide_auto_update`: `notify` (default, human accepts), `auto_minor` (patch/minor accepted automatically, majors wait), or `manual`.
4. Accepting publishes the version for the tenant. If the tenant has an **override** (a forked copy), the inbox shows a three-way diff (old Shippin, new Shippin, override) and offers **Merge into my override** or **Keep mine**.
5. Retiring a version stops new runs; running ones finish on their pinned version. Every step appends `guide.*` events.

## Admin editor
- Who: humans with `guide.edit` (draft) and `guide.publish` (publish/retire); agents never (06).
- Editing a Shippin guide creates a tenant override; the original stays untouched and keeps receiving updates.
- The editor ([07](07-ui.md)) offers form editing per step kind, a JSON tab, schema validation on every save, a **Preview** against a sample machine profile (OS, display server), a dry-run that resolves branches without calling any machine, and a version diff.
- Lint on save (**inference**): each step has one action; `operator_action` has per-OS text for every OS in `applies_to`; every `agent_action` has a `check` or `verify` after it; no literal that matches `secret_patterns`.

## How the AI uses a guide
1. The operator asks ("set up Docker on my Mac") or picks the MCP prompt; the AI calls `guides_search`, then `guide_run_start{guide, machine}`.
2. For each step the AI reads `explain` text and risks, says them in its own words, and either acts (agent steps) or hands over (operator steps), then reports the `check`. It records the outcome with `guide_run_step`.
3. If a check fails, the step's `on_fail` branch runs (e.g. "Screen Recording not granted → show where in System Settings → relaunch → re-check"). After `max_retries` the run pauses and the AI offers **Report a problem**, which attaches the step log (no secrets, no screenshots unless retention keeps them) for Shippin.
4. The runner panel always shows the same step the AI is on, so the operator can follow along or do it themselves.

## Seed guides (preset bundle, Shippin-authored)
`enroll-machine` (per OS, offers WSL on Windows) · `enroll-wsl` · `enable-gui-macos` (Accessibility, Screen Recording, relaunch, test) · `enable-gui-windows` (helper logon task, capture border explained, test) · `enable-gui-linux` (Wayland portals or X11 warning) · `approve-access` (how cards, narrowing, and revoke work) · `enable-unsandboxed` (the risk walk-through for D1) · `docker-setup` with per-OS branches ([13](13-docker-setup.md)) · `podman-setup` · `connect-subscription-worker`.
