# 9. Phased plan
[Index](../devagotchi-spec.md) · Each phase opens with what it touches. Each lands with tests on the box (loopback ports, `agenthub_test` DB) and screenshots in `test-evidence/`, like v0.1 of the hub. Files ≤150 lines, no barrels, no `core` names, no secrets printed, React Aria only. Nothing touches a user machine before phase 4, and phase 4 needs Jesse's approval per host. **v0.1 of Devagotchi = phases 1–4**, so GUI control, guides, and Docker setup ship in v0.1.

## Phase 0. Spec (this)
**Touches:** `docs/devagotchi-spec.md` and `docs/devagotchi/*` in this repo only. No code, no hosts.
**Done when:** committed; open decisions in [10](10-decisions.md) listed with proposed defaults.

## Phase 1. Data, presets, and guide content
**Touches:** hub Postgres schema (migrations 003+), `server/internal/{store,httpapi,presets}`, `server/internal/presets/guides/*.json`, `web/` Catalog, Access settings, Guides, and Guide editor screens. No bridge, no machines.
**Builds:** tables in [01](01-data-model.md) including `grant_presets`, `sandbox_policies`, `resume_acks`, `gui_sessions`, `gui_artifacts`, `guides*`; REST CRUD; new events; JSON Schema validation for guide steps; signed bundle import and the Guide updates inbox; the seed guides of [12](12-guides.md#seed-guides-preset-bundle-shippin-authored) as data (enroll, enable GUI per OS, Docker/Podman per OS).
**Done when:** CRUD + version-conflict + hash-chain tests pass; a bad bundle signature is rejected; published versions are immutable; the editor saves, validates, previews, diffs, and publishes; agents get `policy_denied` on every guide/preset/policy write.

## Phase 2. Access flow and guide runner
**Touches:** hub-server MCP + REST, the reaper, `web/` (Requests, Access card, Narrow editor, Grants, header chip, banners, resume alert, Guide runner panel). Still no bridge.
**Builds:** `request_access` (with `preset`), `wait_for_access`, `my_grants`; approve/narrow/deny/revoke; configurable default preset; unsandboxed policy enable/disable with per-risk acknowledgement; resume alerts on every session resume for full-access/unsandboxed grants; `guides_search`, `guide_get`, `guide_run_start`, `guide_run_step`; guides as MCP resources and prompts.
**Done when:** tests for widening rejected, self-approval rejected, task-scoped grant dies with the lease, `unsandboxed` refused without a policy row, `experimental-full-access` cannot be default without the policy, the resume alert fires on login/reload/idle-return/agent reconnect and cannot be disabled, guide runs pin their version and dry-run end to end; desktop + phone screenshots.

## Phase 3. Linux bridge with GUI and Docker, on the box only
**Touches:** new `bridge/` Go module; hub-server `/bridge/v1`; **the box only** (bridge and GUI helper run on the box's own desktop against a loopback hub; Docker/Podman installs go into a throwaway VM or container on the box if the box itself must stay unchanged). No user machines.
**Builds:** enroll, WSS/long-poll, signed calls, bubblewrap exec, fs ops with hashes, proc leases, probe, honest presence; hub routing of `exec`/`fs_*`/`proc_*`/`gui_*`. GUI helper for **X11** (XTEST, XShm) and **Wayland** (ScreenCast + RemoteDesktop portals with libei, restore token kept 0600), AT-SPI2 tree, overlay + stop hotkey, per-action target checks, screenshot retention modes. `docker-setup` (rootless Engine, Engine, Podman) and `enroll-machine` guides run end to end by an agent.
**Done when:** escape tests (symlink, `..`, `sh -c`, case), revoke-kills-process and revoke-ends-GUI-session, hash-mismatch, orphan reaping, online-only-after-exec; GUI tests: input refused for non-allowlisted app, stop hotkey ends the session, typed text never in events, `none` retention stores no image, wlroots reports input `unsupported`; Docker guide reaches `verify` for rootless and Podman.

## Phase 4. macOS and Windows bridges, GUI helpers, Docker Desktop, WSL
**Touches:** build and signing pipeline on the box; **then user machines, only with Jesse's per-host approval**, one host at a time, each listed with what will be installed (bridge, GUI helper, runtime) before anything runs.
**Builds:** Seatbelt profile generator; Windows sandbox user + ACL + firewall + ConPTY; signed builds; installers (operator-run). GUI helpers: macOS signed `.app` (ScreenCaptureKit, CGEvent, AX; TCC states incl. `needs_relaunch`), Windows per-user helper (Windows.Graphics.Capture, SendInput, UI Automation; no `uiAccess`). WSL enrollment as its own machine. Docker Desktop guides for Windows (WSL 2 backend) and macOS, Podman machine on both.
**Done when:** same test suite on each OS; Windows console apps (ssh) work; TCC denial and re-ask paths handled by the `enable-gui-macos` guide; UIPI-blocked input detected and reported; WSL machine has separate grants; Docker Desktop and Podman reach `verify` on each OS.

## Phase 5. Subscriptions and router
**Touches:** hub DB + UI; an optional local LiteLLM on the box (loopback).
**Builds:** providers, secrets (per D5), subscriptions, role bindings, router endpoints, dispatch + fallback; Subscriptions / Role bindings screens; Connect "subscription worker" templates and the `connect-subscription-worker` guide.
**Done when:** a secret never appears in any response/log (grep tests); dispatch picks by binding + presence; terms badges render.

## Phase 6. Hardening
**Touches:** hub + bridge + GUI helper code; no new hosts.
**Builds:** rate limits, deny cooldown, redaction patterns, audit export anchor (D11), a11y pass, guide lint rules.
**Done when:** threat-model tests T1–T27 have a check or a documented residual.

## Phase 7. Shippin cloud edition
**Touches:** hub schema (adds `tenant_id` to existing tables), auth adapters; **Shippin infrastructure and DNS for devagotchi.com only after Jesse approves the host plan**; the Shippin guide bundle signing key and channel.
**Builds:** `tenant_id` + RLS, Zitadel proxy-header, MCP OAuth 2.1 / RFC 9728 / RFC 8707, per-tenant keys, plan limits, per-user hosted workspaces where users sign in to official clients themselves (D21, [34](34-provider-schema.md)), devagotchi.com deploy, the Shippin guide publishing channel.
**Done when:** cross-tenant tests pass (including guide overrides); OAuth works with a third-party MCP client.

Ordering rationale (**inference**): guides and grants before the bridge mean no exec or GUI path ever exists without the approval flow and the operator-facing explanation; Linux first because the box is Linux and both X11 and Wayland paths, bubblewrap, rootless Docker, and Podman are testable there without touching user machines.
