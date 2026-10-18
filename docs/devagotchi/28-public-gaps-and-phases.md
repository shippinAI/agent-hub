# 28. Public product: gaps vs the repo, phased plan, open questions

[Index](../devagotchi-spec.md) · State of `agent-hub` on 2026-10-18 (commit `45216bf`): Go hub (`server/`), Go
bridge `devagotchi` (`bridge/`, 28 MCP tools, stdio + HTTP, hub mode), Vite + React Aria UI (`web/`), release
script with SHA256SUMS and per-user installers, guides and presets in `server/internal/presets/`. The full gap
list is the register in [27](27-gap-tracking.md) (G-01…G-20).

## Gap summary by area
| Area | Exists | Missing (register id) |
|---|---|---|
| Distribution | `release.sh`, `install.sh`/`.ps1`, client config docs ([17](17-standalone-release.md)) | MCPB, plugin repo, Cursor/Gemini/Registry packaging (G-12); remote OAuth connector (G-13) |
| Updates | Checksums | Signatures (G-09), updater + channels (G-10), config `schema` (G-11) |
| Env setup | 10 seed guides incl. docker/podman/WSL, Guide runner design | Component/profile model and planner (G-14) |
| UI protocol | RAC wrappers in `web/src/ui/` | A2UI renderer and catalog (G-15) |
| Website | — | Site (G-19) |
| Chatmaton | Subscriptions, role bindings, router presets (`server/internal/subs`, `presets/devagotchi/*.json`) | Rooms, modes, roles, routines, model endpoints (G-16); OpenAI row update (G-17) |
| Network | L0 HTTPS/WSS | Mesh adapter + guide (G-18) |
| Quality | Linux parity suite | Windows/macOS runs (G-07), stdout/fail-closed tests (G-20), Windows sandbox (G-05) |

## Phased plan (each step opens with what it touches)

**P1. Honesty and test debt.** *Touches:* `bridge/` tests and code only, on the box. *Builds:* G-20 tests;
`schema = 1` in `policy.toml` with refuse-newer (G-11); tool-description lint. *Done when:* tests green; no
behaviour change for existing users.

**P2. Signed releases and updater.** *Touches:* `scripts/release.sh`, `bridge/` (new `update` command, files
≤150 lines), installers; a signing key generated **by Jesse** and kept off the box. *Builds:* signed index,
minisign + Sigstore bundle, channels, notify/auto/off, rollback (G-09, G-10). *Done when:* tampered archive, older
index, expired index and interrupted swap are each refused or recovered in tests (Windows rename path tested in a
Windows VM only after Jesse approves that host).

**P3. Packaging for clients.** *Touches:* a new public plugin repo (created only after Jesse approves its name),
`dist/`; no listings submitted. *Builds:* MCPB per OS/arch, Claude Code plugin + marketplace, Agent Plugins package
for Codex, Cursor `.mcp.json` + deeplink, Gemini extension archives, `server.json` (G-12). *Done when:* each installs
and runs `tools/list` on a clean VM per client. Submissions (cursor.directory, Registry, Claude plugin directory)
are separate, approved steps.

**P4. Setup components.** *Touches:* hub DB (new `setup_components`, `setup_profiles`), presets, `web/` planner;
the box only. *Builds:* G-14 per [20](20-env-setup.md). *Done when:* `linux-rootless-docker` and `podman` profiles
reach verify twice in a row on a throwaway VM, the second run applying nothing.

**P5. A2UI.** *Touches:* `web/src/a2ui/` (one file per component), hub SSE event type `a2ui`, catalog JSON.
*Builds:* spike on `@a2ui/web_core` vs own processor, then catalog + RAC mapping + bindings (G-15). *Done when:*
golden streams render, axe clean, keyboard tests for Tooltip/Popover/Modal pass.

**P6. Chatmaton local edition.** *Touches:* hub DB (tables in [24](24-chatmaton-spec.md)), presets, `web/`, bridge
exec grants for CLI workers on the box only. *Builds:* rooms, modes, roles, routines, approvals, ProviderChip tabs;
workers for Claude Code, Codex app-server, Grok Build, Antigravity via their documented headless/ACP modes (G-16).
*Done when:* a debate and a review room run end to end with two API-key endpoints; CLI workers only after Jesse
answers D6 per provider.

**P7. Mesh layer.** *Touches:* `bridge/` (optional mesh adapter build), a test network on the box, guide
`add-mesh`. *Builds:* G-18 with `netbird-embed` against a self-hosted test control plane on loopback. *Done when:*
probe over mesh works and revoke removes the peer. No WardenClyffe or Nomen host is touched without approval.

**P8. Website and remote connector.** *Touches:* new `site/` (Next.js 16), and **only after Jesse's host plan**:
devagotchi.com DNS, Shippin cloud hub with OAuth 2.1 (phase 7 in [09](09-phases.md)). *Builds:* G-19, G-13; then
directory submissions with Jesse's approval. *Done when:* Lighthouse SEO/a11y checks pass; Claude and OpenAI
connector checks pass with a reviewer test account.

## Recommendations (inference)
1. Ship the local binary through self-distributed channels now; enter Claude/ChatGPT directories only with the
   remote hub connector (local MCPB is no longer listable; OpenAI local plugins need allowlisting).
2. Notify-only updates by default, signed index with expiry and monotonic versions; never self-update host-managed
   installs.
3. Model environment setup as test/apply/verify components; keep elevated steps with the operator.
4. Build A2UI v0.9.1 with our own catalog (Tooltip/Popover are not in the basic catalog); plan for v1.0.
5. Next.js 16 for the SEO site; keep `web/` on Vite.
6. Chatmaton: API-key routes first; CLI workers via documented headless/ACP; apply to OpenAI's Sign in with ChatGPT
   for the hosted edition.
7. Own the mesh control plane but reuse the BSD-3 NetBird client; settle AGPL obligations before hosting it.

## Open questions for Jesse
1. ~~D6~~ decided 2026-10-18: local orchestration with the user's own login, no badge ([29](29-provider-matrix.md)).
2. Apply to OpenAI's Sign in with ChatGPT partner programme for hosted Chatmaton? Is the local edition open source
   (the open-source path is documented separately)?
3. WardenClyffe: is it derived from NetBird's AGPL server parts, and will Shippin publish changes or buy a licence?
   Does "Layer 2" mean OSI L2 (needs ZeroTier-style VL2 or VXLAN) or the second connectivity layer?
4. Website: Next.js 16 (recommended) or TanStack Start to share the Vite toolchain?
5. Rename the DC-compat tool `give_feedback_to_desktop_commander` for directory listings, keeping an alias?
6. Release signing key custody (hardware key? who signs?) and the public plugin repo name/org.
7. Approve importing the G-01…G-20 rows as hub board tasks?
