# 26. Connectivity layers: HTTPS first, then an owned mesh

[Index](../devagotchi-spec.md) · Research 2026-10-18. Shippin facts come from Jesse's repo
`shippin-cloud/docs` (cited by path); public facts from vendor docs. **Inference** marks our design.

## Layers
| Layer | What | When | Status |
|---|---|---|---|
| L0 | **HTTPS/WSS, outbound only** from bridge to hub; no inbound ports ([02](02-bridge.md), [03](03-bridge-protocol.md)) | Always; onboarding never needs more | Built |
| L1 | **Owned mesh** (WireGuard overlay) between the user's machines, hub and optional cloud workspaces | Offered by a guide *after* onboarding succeeds on L0 | Design |
| L2 | **Cloud extension**: per-workspace mesh peers on shippin.cloud | Hosted edition only, opt-in | Design |
Shippin already decided "HTTPS default, then WardenClyffe Layer 2" (`shippin-cloud/docs/03-decisions.md` #4) and
lists WardenClyffe as "Layer 2 mesh, after HTTPS: per-workspace peers, ACLs generated from authz", status "Exists
(rebuild)" (`shippin-cloud/docs/02-architecture.md`). Nomen is the planned rebranded, optimised Zitadel IdP over
OIDC with token exchange + `act` claims for agents (same file; `03-decisions.md` #6, #15).

**Rule (inference):** a mesh is a *transport*, never an authorization. Every call still needs a hub grant and a
signed call ([03](03-bridge-protocol.md)); being on the mesh grants nothing.

## Options compared
| Option | Shape | Identity | Licence / ownership | Sources |
|---|---|---|---|---|
| WardenClyffe (NetBird-derived, Jesse's) | Management + signal + relay + WireGuard peers | Nomen/Zitadel OIDC; setup keys scoped to tenant/workspace (`shippin-cloud/docs/07b-compute-network.md`) | Owned; but NetBird `management/`, `signal/`, `relay/` are **AGPL-3.0 since v0.53.0** (2025-08-06); clients stay BSD-3; a modified server offered as a public service must publish its changes, or buy NetBird's commercial licence | [LICENSE](https://github.com/netbirdio/netbird/blob/main/LICENSE), [announcement](https://netbird.io/knowledge-hub/netbird-agpl-announcement), [v0.53.0](https://github.com/netbirdio/netbird/releases/tag/v0.53.0) |
| NetBird (stock, self-hosted) | Same parts; ICE via pion, STUN, relay over QUIC/WebSocket; 100.64.0.0/10 addresses | Any OIDC IdP or setup keys | AGPL server, BSD client | [how it works](https://docs.netbird.io/about-netbird/how-netbird-works), [NAT](https://docs.netbird.io/about-netbird/understanding-nat-and-connectivity) |
| Tailscale (SaaS control) | DERP for discovery + fallback, peer relays, direct UDP; >90% direct in their metrics | Tailscale SSO providers | Vendor-run control plane | [connection types](https://tailscale.com/docs/reference/connection-types), [DERP](https://tailscale.com/docs/reference/derp-servers), [NAT traversal pt 1](https://tailscale.com/blog/nat-traversal-improvements-pt-1) |
| Headscale | Self-hosted Tailscale control server, **single tailnet** by design | One OIDC provider; OIDC groups **not** usable in policy | Open source, separate instance per tenant | [README](https://github.com/juanfont/headscale/), [OIDC](https://github.com/juanfont/headscale/blob/main/docs/ref/oidc.md), [policy](https://github.com/juanfont/headscale/blob/main/docs/ref/policy.md) |
| ZeroTier | True **OSI layer 2**: VL1 encrypted P2P, VL2 virtual Ethernet with VLANs/multicast | Controller-issued credentials | Vendor + self-host controller | [protocol](https://docs.zerotier.com/protocol/) |

Note: WireGuard meshes (NetBird, Tailscale, WardenClyffe) are IP (layer 3) overlays. "Layer 2" in Shippin docs
reads as "the second connectivity layer"; if OSI L2 (broadcast, non-IP) is wanted, that is ZeroTier-style VL2 or
VXLAN over WireGuard (open question, [28](28-public-gaps-and-phases.md)).

## Embedding in the `devagotchi` binary (no second daemon)
- NetBird's `client/embed` package runs a peer inside a Go process with a userspace netstack by default (no TUN,
  no admin); `Dial`/`ListenTCP` need userspace mode ([embed.go](https://github.com/netbirdio/netbird/blob/1e7b16db/client/embed/embed.go), [PR #3239](https://github.com/netbirdio/netbird/pull/3239)). It lives under the BSD-3 part of the repo.
- Tailscale's `tsnet` does the same for Tailscale/Headscale ([tsnet](https://pkg.go.dev/tailscale.com@v1.102.0/tsnet)).
- **Inference:** a `mesh` adapter interface in the bridge with `netbird-embed` (WardenClyffe and NetBird) and
  `tsnet` (Tailscale/Headscale) implementations; the user picks one in a guide; the binary grows by the client's
  size, so ship it as an optional build or extension ([19](19-public-product-spec.md)) and measure first.

## WireGuard practice we follow
- Private keys are generated on the device and never leave it; relays forward ciphertext only
  ([DERP](https://tailscale.com/docs/reference/derp-servers), [NetBird relays](https://docs.netbird.io/help/troubleshooting-relayed-connections)).
- Cryptokey routing: each peer's public key is bound to its AllowedIPs; never overlap AllowedIPs between peers
  ([whitepaper](https://www.wireguard.com/papers/wireguard.pdf)).
- `PersistentKeepalive = 25` only for peers behind NAT that must receive; off otherwise ([quickstart](https://www.wireguard.com/quickstart/)).
- Optional preshared key adds a symmetric layer for post-quantum resistance ([wg(8)](https://man.archlinux.org/man/wg.8.en)).
- Handshake is Noise_IK with forward secrecy ([protocol](https://www.wireguard.com/protocol/)); rotate device keys on
  revoke and on a schedule (data), add-new-then-remove-old.
- UDP-blocked networks exist (a campus Wi-Fi that drops all UDP but DNS), so a TCP/HTTPS relay path is required
  ([how NAT traversal works](https://tailscale.com/blog/how-nat-traversal-works)); L0 already covers this.

## Build vs own (inference)
- **Own the control plane** (WardenClyffe): identity, ACLs from authz (OpenFGA per Shippin docs), per-tenant
  keys; we choose AGPL compliance (publish server changes) or a NetBird commercial licence; decide before hosting.
- **Do not build** NAT traversal or a WireGuard stack: reuse the BSD-3 NetBird client (pion ICE) or tsnet.
- **Offer Tailscale/Headscale** as presets for users who already run them; identical grant model on top.

## Guide `add-mesh` (data, after onboarding)
1. `explain`: what a mesh adds (direct paths, lower latency, LAN-style addresses) and what it does not (access).
2. `branch`: WardenClyffe (default for Shippin users) · NetBird · Tailscale · Headscale · skip.
3. `request_access`: `net.mesh` capability (new) with the chosen provider's hosts; duration `task`.
4. `operator_action`: sign in through the IdP (Nomen/Zitadel OIDC device flow) or paste a setup key (masked).
5. `agent_action`: bridge starts the embedded peer; `check`: hub sees the machine's mesh address and a ping RTT.
6. `verify`: a signed `probe` call succeeds over the mesh; the hub then *prefers* mesh, falls back to L0.
7. `explain`: how to remove it; revoking the machine also removes its mesh peer and rotates keys.
