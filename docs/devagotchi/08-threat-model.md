# 8. Threat model
[Index](../devagotchi-spec.md) · Assets: user files and processes on enrolled machines, provider secrets, subscription logins, the integrity of the task board and audit log, and tenant isolation (cloud). Trust: humans with UI auth > hub > bridge > agents (agents are **untrusted**, including the text they put in `reason`).

| # | Threat | Mitigation | Residual |
|---|---|---|---|
| T1 | Prompt-injected agent tries to escape scope (paths, `sh -c`, `<()`, symlinks, case folding) | The OS sandbox enforces the selector. Argv only, no parsing. realpath on the bridge. Grants built from the hub row, not from call args ([#791](https://github.com/wonderwhy-er/DesktopCommanderMCP/issues/791), [#497](https://github.com/wonderwhy-er/DesktopCommanderMCP/issues/497), [#730](https://github.com/wonderwhy-er/DesktopCommanderMCP/issues/730)) | Sandbox bugs; `unsandboxed` level if enabled (D1) |
| T2 | Agent grants itself more access | No MCP tool writes grants. Human-only approval in code. No self-approval. No bridge config tool | A compromised human UI session |
| T3 | Social engineering via a persuasive `reason` | Reason labelled "the agent says". Risks come from the catalog, not the agent. Narrowing by default suggested. Deny cooldown collapses repeats | User fatigue |
| T4 | Data exfiltration over allowed hosts | `net.egress` default none. Broad-domain risk rows. Proxy limit (no TLS inspection) stated ([Claude sandboxing](https://code.claude.com/docs/en/sandboxing)) | Domain fronting through an allowed host |
| T5 | Stolen machine token | Token 0600/ACL. Per-machine HMAC on calls. Revoke from the UI. Token only lets the bridge *receive* calls, not issue them | A local attacker with user rights can run as the bridge |
| T6 | Compromised hub sends malicious calls | The bridge executes only under a grant it re-fetches. Calls are signed. The sandbox still applies. **Inference:** a hub compromise equals the union of live grants, so short durations and `task` grants limit the blast radius | Full compromise within live grants |
| T7 | Revocation lag | Revoke push + `grant_cache_seconds` ≤30 (code clamp). Running calls under the grant are killed | ≤30 s window if push fails |
| T8 | Stale/orphan process keeps running | proc leases with fence + TTL. The reaper kills the tree (job object / process group / cgroup) | Daemonised children that double-fork out of the group on Linux without cgroups |
| T9 | Edit clobbers concurrent change / partial write | `expect_sha256`, temp+fsync+rename, exact match ([#433](https://github.com/wonderwhy-er/DesktopCommanderMCP/issues/433), [#97](https://github.com/wonderwhy-er/DesktopCommanderMCP/issues/97)) | Non-atomic network filesystems |
| T10 | Secret leakage (logs, events, MCP, UI, screenshots) | Write-only secrets, `fp` only, redaction patterns in bridge logs, output bodies never in events, no "show" toggle | Secrets printed by the user's own commands into exec output (shown only to the caller) |
| T11 | Subscription credential misuse / ToS breach | No proxying of subscription tokens. `cli_login` only on the named device via the official CLI ([Anthropic terms](https://code.claude.com/docs/en/legal-and-compliance)) | Vendor ToS changes (unverified rows flagged) |
| T12 | Telemetry / supply chain | No telemetry. Signed pinned binaries. No auto-update without consent. Reproducible builds (goal) ([#797](https://github.com/wonderwhy-er/DesktopCommanderMCP/issues/797), [#327](https://github.com/wonderwhy-er/DesktopCommanderMCP/issues/327)) | Compromised signing key |
| T13 | Cross-tenant access (cloud) | `tenant_id` on every row + Postgres RLS. Tenant derived from the credential. Per-tenant data keys | RLS policy bugs (tests required) |
| T14 | Audit tampering | Existing append-only triggers + hash chain + verify endpoint. New events use the same writer | A DB superuser can rewrite and re-hash; export anchors are decision D11 |
| T15 | Presence spoofing / false "online" | Online only after an executed call result | None material |
| T16 | Enrollment code interception | 10 min, single use, hashed, 5 tries, entered at a prompt not argv | Shoulder-surfing |
| T17 | Phishing via MCP elicitation | Elicitation never asks for secrets. The hub card is canonical (MCP elicitation rule) | Client UI that hides the requesting server |

## Out of scope (v0.1)
Physical access to an unlocked machine; a malicious user attacking their own machine; kernel sandbox zero-days; GUI automation (D7).
