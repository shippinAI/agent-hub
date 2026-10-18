# 18. Public product research: distribution and updates

[Index](../devagotchi-spec.md) · Research for shipping Devagotchi publicly as a Desktop Commander (DC) competitor.
Read on 2026-10-18 unless marked otherwise. Every fact has a source next to it. **Inference** marks our own reading;
**unverified** marks a claim with no primary source. DC is a reference only; none of its code is used.
The spec built on this is in [19](19-public-product-spec.md).

## 1. Where an MCP server can be listed (per client)

| Client | Accepts | Directory / review | Auth needs | Sources |
|---|---|---|---|---|
| Claude (claude.ai, Desktop, mobile, Code, Cowork) | **Remote** HTTPS MCP connectors; **local** servers only inside a **plugin** | Developer portal `claude.ai/directory/manage`; any paid plan can submit; automatic scan, may get human review | OAuth 2.0 (DCR or CIMD) for user data, or none; `client_credentials` not supported; a real `401` + `WWW-Authenticate` starts sign-in; discovery/registration/token endpoints must answer in 10 s | [submission](https://claude.com/docs/connectors/building/submission), [auth](https://claude.com/docs/connectors/building/authentication), [publish](https://claude.com/docs/directory/publish) |
| Claude Desktop, `.mcpb` bundle | A zip with `manifest.json`, stdio server, `user_config` drives a settings UI | **Directory no longer accepts MCPB**; users install the file themselves | n/a (local) | [MCPB](https://claude.com/docs/connectors/building/mcpb), [schemas v0.1–v0.4](https://github.com/modelcontextprotocol/mcpb/tree/main/schemas) |
| Claude Code plugins | `.claude-plugin/plugin.json`, `.mcp.json` (stdio or http), skills, hooks | Own marketplace = a `marketplace.json` in a git repo, no form; or the Anthropic directory from a public GitHub repo | Never put secrets in `.mcp.json` (every installer receives it) | [publish a plugin](https://code.claude.com/docs/en/plugins/publish), [submit plugin](https://claude.com/docs/plugins/submit), [quickstart](https://claude.com/docs/plugins/quickstart), [MCP in Claude Code](https://code.claude.com/docs/en/mcp) |
| ChatGPT + Codex (one shared plugin directory) | Public listing needs a **public HTTPS Streamable HTTP** endpoint; local/tunnel endpoints are refused | Plugin submission portal; identity verification (individual or business); domain verification at `/.well-known/openai-apps-challenge`; tool scan; 5 positive + 3 negative test cases; CSP for every fetched domain | OAuth 2.1 when user auth is needed; API-key auth is not supported by the portal's connection form | [Apps SDK submission](https://developers.openai.com/apps-sdk/deploy/submission), [submit plugins](https://learn.chatgpt.com/docs/submit-plugins), [build MCP server](https://developers.openai.com/plugins/build/mcp-server), [package plugin](https://developers.openai.com/plugins/build/plugins) |
| ChatGPT desktop, **desktop-only plugin** | Local stdio MCP in the plugin zip | Restricted to approved, **allowlisted** partners | n/a | [desktop-only](https://developers.openai.com/plugins/deploy/desktop-only) |
| Codex CLI (self-distributed) | `~/.codex/config.toml` `[mcp_servers]`; marketplaces via `codex plugin marketplace add owner/repo` | None for own marketplace; plugins with lifecycle hooks are **not** eligible for the public directory | Per-server `auth` in `mcp.json` (`none`/`oauth`/`mixed`/`api_key`) | [package plugin](https://developers.openai.com/plugins/build/plugins) |
| xAI Grok API (server-side MCP) | **Remote only**: Streamable HTTP or SSE; `server_url`, `server_label`, optional `allowed_tools`, `authorization`, `headers` | No directory found | Bearer token in `authorization`; `require_approval` not supported | [remote MCP](https://docs.x.ai/developers/tools/remote-mcp) |
| Grok Build CLI | stdio or http via `grok mcp add`, `~/.grok/config.toml`; OAuth flow for remote servers | Has a plugin marketplace (seen in the launch post; listing rules not found, **unverified**) | `${VAR}` expansion keeps secrets in env | [MCP servers](https://docs.x.ai/build/features/mcp-servers), [launch](https://x.ai/news/grok-build-cli) |
| Cursor | `mcp.json` (project or `~/.cursor`); one-click `cursor://anysphere.cursor-deeplink/mcp/install?name=..&config=<base64>` | cursor.directory: submit a public repo with `.mcp.json` at its root; an agent security scan gives `safe`/`suspicious`/`malicious` | per server config | [install links](https://cursor.com/docs/mcp/install-links), [MCP](https://cursor.com/docs/mcp), [community-plugins README](https://github.com/cursor/community-plugins) |
| Gemini CLI | `gemini-extension.json` at repo or archive root, `mcpServers` | Gallery auto-indexes public GitHub repos with topic `gemini-cli-extension`, daily | n/a | [releasing](https://geminicli.com/docs/extensions/releasing/), [reference](https://geminicli.com/docs/extensions/reference/) |
| Any MCP client | `server.json` in the official MCP Registry | `mcp-publisher publish`; namespace proven by GitHub login (`io.github.user/*`) or DNS/HTTP (`com.example/*`) | n/a | [registry quickstart](https://modelcontextprotocol.io/registry/quickstart), [auth](https://github.com/modelcontextprotocol/registry/blob/main/docs/modelcontextprotocol-io/authentication.mdx) |

Notes that change the plan:
- Gemini CLI for unpaid and Google One users was **replaced by Antigravity CLI on 2026-06-18** (banner on
  [geminicli.com](https://geminicli.com/docs/extensions/releasing/)). Antigravity reads `.agents/` hooks and
  `~/.gemini/antigravity-cli/settings.json` ([headless](https://antigravity.google/docs/cli/headless/)).
- Gemini extensions released as GitHub Releases can ship per-platform archives named `{platform}.{arch}.{name}.tar.gz`,
  which fits a Go binary; the CLI detects updates from the "Latest" release
  ([releasing](https://geminicli.com/docs/extensions/releasing/)).
- Anthropic directory policy for MCP servers: graceful errors, token frugality, tool names ≤64 chars, `title` +
  `readOnlyHint`/`destructiveHint` on every tool, Streamable HTTP for remote servers
  ([directory policy](https://support.claude.com/en/articles/13145358-anthropic-software-directory-policy)).
- OpenAI requires `readOnlyHint`, `openWorldHint` and `destructiveHint` on every tool
  ([submit plugins](https://learn.chatgpt.com/docs/submit-plugins)).
- MCP Apps (`io.modelcontextprotocol/ui`, `ui://` resources rendered in a sandboxed iframe) is the cross-client UI
  extension; Claude supports it, ChatGPT since 2026-01-26 ([overview](https://modelcontextprotocol.io/extensions/apps/overview),
  [blog](https://blog.modelcontextprotocol.io/posts/2026-01-26-mcp-apps/)).

**Inference, the shape that follows:** the local `devagotchi mcp` binary reaches users through *self-distributed*
channels (install script, MCPB file, Claude/Codex plugin marketplaces in our repo, Cursor deeplink, Gemini
extension, MCP Registry). *Directory* listings in Claude and ChatGPT need the **remote** hub (cloud edition, phase 7)
with OAuth. A remote connector can then reach the user's machine through the bridge, which is the safe way to give
directory clients local reach.

## 2. Update and maintainability practice

| Product | What it does | Source |
|---|---|---|
| Tailscale | `tailscale set --auto-update`; tracks `stable`/`unstable` (odd minor = unstable); `tailscale update --track=`; Go package `clientupdate` | [CLI](https://tailscale.com/docs/reference/tailscale-cli), [tracks](https://tailscale.com/docs/reference/tailscale-client-versions), [clientupdate](https://pkg.go.dev/tailscale.com/clientupdate) |
| Tailscale (failure) | Auto-update interrupted dpkg on Ubuntu (beta issue) | [#10400](https://github.com/tailscale/tailscale/issues/10400) |
| Ollama | macOS/Windows: downloads in background, user clicks "Restart to update"; Linux: no auto-update, re-run install script | [FAQ](https://docs.ollama.com/faq), [Linux](https://docs.ollama.com/linux) |
| gh CLI | Notifier only: checks at most once per 24 h, prints to stderr, off in CI/non-TTY, `GH_NO_UPDATE_NOTIFIER` | [env help](https://cli.github.com/manual/gh_help_environment), [update.go](https://github.com/cli/cli/blob/trunk/internal/update/update.go) |
| gh CLI (failure) | Cancelled check skips saving its timestamp, so checks repeat | [#12599](https://github.com/cli/cli/issues/12599) |
| DC (failure) | A beta was auto-pushed to stable users through Claude Extensions, causing crashes | [DC #327](https://github.com/wonderwhy-er/DesktopCommanderMCP/issues/327) |
| TUF | Signed metadata roles (root, timestamp, snapshot, targets); versions must not go down (rollback) and expired metadata is refused (freeze) | [TUF spec](https://theupdateframework.github.io/specification/latest/), [go-tuf updater](https://github.com/theupdateframework/go-tuf/blob/master/metadata/updater/updater.go) |
| go-tuf (failure) | v0.2.0 had no rollback protection for non-root roles; fixed in v0.3.0 | [GHSA-66x3-6cw3-v5gj](https://github.com/theupdateframework/go-tuf/security/advisories/GHSA-66x3-6cw3-v5gj) |
| minisign | Ed25519 signatures with a signed trusted comment; Go verifier available | [minisign](https://github.com/jedisct1/minisign), [go-minisign](https://github.com/jedisct1/go-minisign) |
| Sigstore cosign | Keyless `sign-blob --bundle`; verify pins the workflow identity and OIDC issuer | [signing blobs](https://docs.sigstore.dev/cosign/signing/signing_with_blobs/), [verify-blob](https://github.com/sigstore/cosign/blob/main/doc/cosign_verify-blob.md) |
| Windows swap | A running `.exe` can be renamed but not overwritten: stage `.new`, rename current to `.old`, move `.new` in, roll back on failure | [minio/selfupdate apply.go](https://github.com/minio/selfupdate/blob/master/apply.go) |
| Plugins as processes | HashiCorp go-plugin: plugin is a subprocess; handshake line on stdout; gRPC with health service | [internals](https://github.com/hashicorp/go-plugin/blob/main/docs/internals.md) |
| Schema versions | Kubernetes: `apiVersion` per group; fields never removed from a version; versions round-trip; storage version moves only after both are served | [deprecation policy](https://kubernetes.io/docs/reference/deprecation-policy/), [CRD versions](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definition-versioning/) |

**Inference, lessons:** notify by default and apply only with consent (gh, Ollama); channels are explicit and a
user can move from beta back to stable (Tailscale); never let a host's auto-update push beta to stable (DC #327);
the updater itself must survive interruption (Tailscale #10400, gh #12599); signatures alone do not stop rollback or
freeze, so the update metadata needs versions and expiry (TUF).
