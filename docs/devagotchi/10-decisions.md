# 10. Decisions
[Index](../devagotchi-spec.md) · Resolved decisions record Jesse's answer (2026-10-17). Open ones carry a proposed default that the spec uses until answered.

## Resolved
| # | Decision | Jesse's answer | Where |
|---|---|---|---|
| D1 | Allow an `unsandboxed` grant level? | **Off by default; a configurable policy the operator can permit** (tenant or machine). Risks shown and ticked, enabling audited, persistent reminder while active | [04](04-access-flow.md#unsandboxed-policy-d1-resolved), `sandbox_policies` in [01](01-data-model.md) |
| D2 | Windows: native only, or WSL as a separate machine? | **Native first; WSL enrolled as its own Linux machine** | [02](02-bridge.md#windows-native-first-wsl-as-its-own-machine-d2-resolved) |
| D4 | Default grant duration | **`task` by default, and the default is configurable.** An `experimental-full-access` preset may be the default; it shows an alert and reminder **every** time a session resumes | [04](04-access-flow.md#grant-length-and-presets-d4-resolved), 01 invariant 7 |
| D7 | Desktop GUI automation in scope? | **Yes, from v0.1**: capture, mouse/keyboard, window/app control on Windows, macOS, Linux X11/Wayland, with its own grant scopes, risks, and audit | [11](11-gui-control.md), phases 3–4 |
| D10 | What "SME" means | **The AI acts as the subject-matter expert who guides the operator.** It does not mean the operator is non-technical | [04](04-access-flow.md), index "Who is who" |
| D16 | Guided setup content | **Shippin-authored, versioned, editable content data** with an admin editor and an update flow | [12](12-guides.md) |
| D17 | Docker setup | **Guided per OS** (Docker Desktop vs Engine, WSL 2 backend on Windows, Podman as an alternative preset), via guides and the bridge under grants | [13](13-docker-setup.md) |

## Open
| # | Decision | Options | Proposed default |
|---|---|---|---|
| D3 | Approval channel | hub card only / MCP elicitation only / both | Both, hub card canonical |
| D5 | Secret store | Postgres + pgcrypto with a local key file / OS keychain on the bridge / external vault (cloud) | Personal: local key file 0600 + pgcrypto; cloud: KMS-wrapped per-tenant keys; **2026-10-18: hosted uses Vaultix (rebranded Infisical, [36](36-vaultix.md)); local keeps the built-in store, Vaultix optional** |
| D6 | Accept that Claude/ChatGPT subscriptions run only as on-device official CLIs (hub workers), never proxied? Non-interactive worker use is unverified as "ordinary use" | accept / API keys only | **Decided 2026-10-18: accept** for the free, personal local edition; users sign in themselves; no badge ([29](29-provider-matrix.md)) |
| D21 | Subscriptions in the hosted edition | API keys only / user's own sign-in | **Decided 2026-10-18: user's own sign-in in both editions** (hosted workspace or own machine); [ADR 0002](adr/0002-provider-schema-and-adapters.md) |
| D8 | Tenancy in the personal edition | `tenant_id` from day one / add in phase 7 | New tables get it in phase 1; existing tables in phase 7 |
| D9 | macOS sandbox fallback if `sandbox-exec` goes away | Endpoint Security / VM / refuse sandboxed grants | Refuse (read-only only) until decided |
| D11 | Audit anchoring | none / periodic hash export to a file or git / external timestamping | Daily export of the head hash to a file the operator keeps |
| D12 | Bridge distribution | GitHub releases / devagotchi.com downloads / package managers | Signed releases on devagotchi.com + checksums |
| D13 | Router | LiteLLM (self-host) / OpenRouter BYOK / both as data | **Decided 2026-10-18:** direct keys by default; LiteLLM optional and configurable; OpenRouter as a row ([34](34-provider-schema.md)) |
| D14 | Cloud pricing/plan limits for Shippin | – | Out of scope here; plan limits stay data |
| D15 | Keep GUI screenshots? | `none` / redacted thumbnails / full encrypted, with retention days | `none` (hash + size only); operator can opt in per tenant |
| D18 | What counts as a "session resume" for the full-access alert | UI only / UI + agent reconnects / also every task claim | UI sign-in, reload, return after `resume_alert_idle_minutes` (15), and agent reconnect or task claim under the grant |
| D19 | `gui.input` with `apps: any` | never / only with the unsandboxed policy / allowed | Only with the unsandboxed policy, plus its risk card |
| D20 | Windows `uiAccess` (drive elevated windows) | never / opt-in signed build | Never in v0.1 (elevated steps stay with the operator) |
| D21 | Guide signing and channel | Shippin key in settings / Sigstore-style keyless / unsigned for personal | One Shippin signing key (public key in settings), `stable` and `beta` channels |
| D22 | Guide auto-update default | notify / auto minor+patch / manual | `notify` |
| D23 | Default container runtime per OS | Docker Desktop / Engine (rootless) / Podman | Windows + macOS: Docker Desktop (Podman offered when licence needs a paid plan); Linux: rootless Engine |
| D24 | Wayland compositors without RemoteDesktop (wlroots) | capture only / fall back to X11 via Xwayland / refuse GUI | Capture only, input reported `unsupported` |

## Unverified items carried in this spec
- None for subscriptions: D6 and D21 close them for both editions. Sign in with ChatGPT is an extra auth kind where offered ([23](23-chatmaton-research.md)).
- Long-term availability of macOS `sandbox-exec`.
- macOS Screen Recording needing a relaunch after first grant (community source only); Accessibility PPPC deprecation in macOS 26.2 (third-party wiki); launchd-started helpers failing TCC responsible-process checks on macOS 26 (forum report). See [11](11-gui-control.md).
- Window move/resize of other apps on Wayland (compositor-specific; no portal found).
- Goose / Open Interpreter / LibreChat comparisons (pages did not render for the research); Reddit sentiment not covered (blocked).
