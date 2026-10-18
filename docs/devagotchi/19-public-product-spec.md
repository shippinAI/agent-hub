# 19. Public product spec: packaging, updates, extensions, config versions

[Index](../devagotchi-spec.md) · Built on the research in [18](18-public-product-research.md). No code yet.
Everything here is a design choice (**inference**) unless a source is cited. Lists, URLs, channels, keys and
schedules are data, not code.

## Channels we ship to (one binary, many wrappers)

| # | Artifact | Touches | Built from | Notes |
|---|---|---|---|---|
| C1 | Signed release archives + `SHA256SUMS` + signatures | `dist/release/`, devagotchi.com downloads (D12) | `scripts/release.sh` (exists) | Base for every other row |
| C2 | `install.sh` / `install.ps1` | user's `~/.local/bin` or `%LOCALAPPDATA%` (exists, [17](17-standalone-release.md)) | C1 | Add signature check next to the checksum |
| C3 | `.mcpb` bundle per OS/arch | Claude Desktop only, user-installed | C1 + `manifest.json` (MCPB schema latest) | `user_config` → roots list; self-hosted file, the directory no longer takes MCPB ([18](18-public-product-research.md)) |
| C4 | Claude Code / Cowork plugin (`.claude-plugin/plugin.json`, `.mcp.json` stdio, skills) | a public plugin repo we own | C2 for the binary; plugin points at the installed path | Own marketplace first; directory submission later |
| C5 | Agent Plugins package (`plugin.json`, `mcp.json`) for Codex | same plugin repo | C2 | Local marketplace only; the public ChatGPT/Codex directory needs remote HTTPS ([18](18-public-product-research.md)) |
| C6 | Cursor `.mcp.json` + install deeplink | plugin repo + website button | C2 | Submit to cursor.directory |
| C7 | Gemini/Antigravity extension (`gemini-extension.json`, per-platform archives) | plugin repo + GitHub topic | C1 | Check Antigravity's extension format before release (**unverified** whether it reads `gemini-extension.json`) |
| C8 | MCP Registry `server.json` | registry entry under a domain namespace (`com.devagotchi/*`) | C1 | DNS proof on devagotchi.com |
| C9 | **Remote connector** (cloud hub `/mcp`, OAuth 2.1, RFC 9728 metadata) | Shippin cloud (phase 7) | hub | The only route into the Claude and ChatGPT directories; also usable from the xAI API `mcp` tool |
| C10 | **Hosted workspace runner** (per-user isolated workspace on shippin.cloud with the bridge) | Shippin cloud; one workspace and one encrypted home volume per user | C1 + a base image | Official clients are installed by the user's guide step from the vendor's own installer, not bundled; the user runs each client's login inside the workspace; the hub never reads those homes ([34](34-provider-schema.md), [ADR 0002](adr/0002-provider-schema-and-adapters.md)) |

Listing checklist (data, shown as a release gate in the UI): every tool has `title`, `readOnlyHint`,
`destructiveHint`, `openWorldHint` (bridge tools already declare titles and hints, [16](16-dc-parity.md) P1); tool
names ≤64 chars; privacy policy URL, docs URL, support contact, icon 512×512 PNG; reviewer test account for C9;
5 positive + 3 negative test prompts for OpenAI; CSP list for any MCP App UI. Sources are in [18](18-public-product-research.md).

## Update architecture

**Parts.**
1. **Release index** (`releases/<channel>/index.json`), signed: `{schema: 1, channel, version, published_at,
   expires_at, min_supported, artifacts:[{os, arch, url, sha256, size}], notes_url}`. `expires_at` stops freeze
   attacks; the client stores the highest `version` it has accepted and refuses a lower one (rollback), as TUF
   requires ([TUF](https://theupdateframework.github.io/specification/latest/)). **Inference**: one key-pair and
   index is enough for v1; full TUF roles (go-tuf) are the upgrade path if we add mirrors or delegated publishers.
2. **Signatures.** Ed25519 (minisign format) with the public key compiled in and also listed in settings for
   rotation; Sigstore bundle as a second, keyless proof tied to the release workflow
   ([minisign](https://github.com/jedisct1/minisign), [cosign](https://docs.sigstore.dev/cosign/signing/signing_with_blobs/)).
   Key rotation: the new key is announced in an index signed by the old key.
3. **Channels.** `stable`, `beta`, `nightly` as data rows. Switching down (beta → stable) is allowed and installs the
   newest stable even if it is lower, but only on an explicit human command (Tailscale allows the same switch,
   [PR #12477](https://github.com/tailscale/tailscale/pull/12477)).
4. **Policy.** `update.mode = notify | auto | off` (default `notify`, like gh and D22). `auto` applies only between
   tool calls when no proc lease is live. The check runs at most once per `update.check_interval` (24 h), is skipped in
   CI and when `DEVAGOTCHI_NO_UPDATE_CHECK` is set, and writes its timestamp *before* the network call so a crash
   does not cause repeat checks (gh #12599).
5. **Apply.** Download to the config dir, verify size + sha256 + signature, run `<new> version` and `<new> doctor
   --quick` from the staged path, then swap: rename-to-`.old` on Windows, atomic rename on Unix
   ([minio/selfupdate](https://github.com/minio/selfupdate/blob/master/apply.go) shows the Windows pattern; we write our
   own). Keep one `.old` for `devagotchi update --rollback`. Services installed by `devagotchi install` restart
   through their unit/LaunchAgent/task.
6. **Where updates are refused.** Package-manager installs (brew, winget, apt) and MCPB installs report "managed by
   <x>" and never self-replace; the host owns them. The hub can push "update available" to bridges but never forces
   one; a bridge below `min_supported` is shown as *needs update* and keeps read-only tools only (**inference**).
7. **Hub-pushed updates in hub mode.** The hub forwards the signed index; the bridge verifies it with its own
   compiled-in key, so a compromised hub cannot ship a binary.

## Extension model

- **First choice: MCP servers as extensions.** Devagotchi already speaks MCP; a third-party tool pack is an MCP server
  the bridge launches under the same sandbox and grants, with its tools namespaced `ext.<name>.*`.
- **Out-of-process only**, like go-plugin's subprocess model ([internals](https://github.com/hashicorp/go-plugin/blob/main/docs/internals.md)):
  a crashing extension cannot take the bridge down, and the sandbox applies to it.
- **Manifest** (`extension.json`, versioned): name, version, publisher, `requires_caps`, `requires_hosts`, binary per
  os/arch with sha256, signature. Install shows an access card; nothing runs before a grant.
- **Guide packs** stay as signed guide bundles ([12](12-guides.md)); they are content, not code.

## Config schema versioning

- Every on-disk file and every preset bundle carries `schema = N` (TOML) / `"schema": N` (JSON). Today
  `policy.toml` has no version key (gap, [28](28-public-gaps-and-phases.md)).
- A version never drops a field. Removal means `N+1` and a converter `N → N+1`, applied on load, written back only
  after a `.bak` copy, and logged; old binaries refuse a newer `schema` with a clear "upgrade devagotchi" error
  (Kubernetes rules, [deprecation policy](https://kubernetes.io/docs/reference/deprecation-policy/)).
- JSON Schema files for each version ship in the binary and in the docs, so the UI editors and `config validate` use
  the same rules.
