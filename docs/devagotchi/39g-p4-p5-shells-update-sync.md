# 39g. P4: native shells and signed self-update · P5: devpulse sync

[Plan](39-chatmaton-gold-build-plan.md) · Research: [39a](39a-research-app-shells.md),
[39b](39b-research-offline-sync.md). P4 closes G-09 and G-10 ([27](27-gap-tracking.md)). P5 closes the resolver part
of G-25.

## P4. Native shells (Tauri 2) and signed self-update
**Done when:** Chatmaton installs on macOS, Windows, Linux, Android (APK) and iOS (TestFlight or ad-hoc, if decision
2 says yes). Approvals arrive as native pushes. A desktop shell updates itself from a minisign-signed manifest
served by the hub. `hub-server`, `bridge` and `devpulse` update themselves, with rollback.

**Design.**
- **One shell project.** It loads the same `web/` build, bundled, so it starts offline. The API base URL comes from
  `access_endpoints` (cached and probed), never from a compiled-in host. The app id, update URL and pubkey are
  build-time config files, not code constants.
- **Plugins** (pinned): `updater` (desktop), `notification`, `deep-link`, `biometric` (local unlock only), and a
  community push plugin behind our `notify` adapter (`apns`/`fcm` channel rows). iOS passkeys use the
  associated-domains entitlement plus an AASA file served by the hub.
- **Desktop sidecar:** `devpulse` (Go) ships as `externalBin` (desktop only, [sidecar](https://v2.tauri.app/develop/sidecar/)).
- **Releases.** CI (release-please and goreleaser configs; uncommitted in the working tree on 2026-10-18) builds artifacts, then
  `minisign -S` runs on a **signing host that agents cannot reach**. The hub stores release rows. Clients verify
  before they install. Channels: `stable`, `beta` (data). Rollback keeps the previous binary.
- **Self-patch loop:** a room proposes a patch → PR → CI → a gold-policy ticket carries the diff, build hashes and
  signature → Jesse signs the approval with a passkey (P2) → the release is published to a channel → updaters
  pick it up. Agents can never publish to a channel on their own (a code invariant, like the `human: true` rule).

**Construction.** New dir `apps/chatmaton/`: `src-tauri/Cargo.toml`, `src-tauri/tauri.conf.json`,
`src-tauri/capabilities/default.json`, `src-tauri/src/lib.rs` (plugin wiring only, ≤150 lines),
`src-tauri/src/push.rs` (token → `POST /push/subscriptions` with channel `apns`/`fcm`), `apps/chatmaton/package.json`
(uses `../../web` as `frontendDist`). Web: `web/src/lib/platform.ts` (detects Tauri vs browser; no SDK imports in
pages), `web/src/lib/nativePush.ts`. Server: `internal/releases/`: `manifest.go` (Tauri updater JSON),
`verify.go` (go-minisign), `channels.go`, `store.go`; `internal/notify/apns.go`, `fcm.go`, `ntfy.go`;
`devapi/release_routes.go`. Go self-update: `bridge/selfupdate/` (shared by bridge and devpulse): `check.go`,
`verify.go`, `swap.go` (atomic rename, keeps `.prev`), `rollback.go`; `server/cmd/hub-server/update.go` (admin
subcommand `update`). Migration `013_releases.sql`: `releases(id, product, channel, version, target, arch, url,
sha256, minisign_sig, notes, published_by, published_at, yanked_at)`, `update_channels`, `device_installs(device_id,
principal_id, product, version, channel, last_check_at)`.

**Endpoints.** `GET /api/v1/updates/{product}/{target}/{arch}/{current}` (Tauri format; 204 when current),
`GET /api/v1/releases`, `POST /api/v1/releases` (admin, needs a signed ticket id), `POST /api/v1/releases/{id}/yank`,
`GET /.well-known/apple-app-site-association` and `/.well-known/assetlinks.json` (generated from data rows).

**Tests.** Signature tamper → refuse; wrong channel → no offer; swap interrupted → `.prev` restored; yanked →
downgrade offer only if `allow_downgrade` (data); Tauri updater against a local manifest in CI; manual device
matrix (5 targets).

**Gold tickets (drafts).** GC-CM-P4-01: minisign key ceremony on an offline or signing host; public key to config;
the secret key never on Gold agents or the box; passphrase held by Jesse. GC-CM-P4-02: APNs key and FCM service
account stored in Vaultix (needs decision 2), refs only in `notify_channels`. GC-CM-P4-03: serve AASA and
assetlinks on the canonical domain; deploy migration 013. GC-CM-P4-04: enable `updates.auto=notify` for bridge on
`gold-seats` first (canary), then the hub.

## P5. Devpulse: local-first client synced with the Gold devspace
**Done when:** `devpulse` on a laptop shows the workspace's tasks, effective config and room transcripts offline.
Edits made offline sync when back online. A `locked` item edited offline is rejected with a visible reason. A
tenant-scoped device never receives rows outside its workspace.

**Design.** Own Replicache-style protocol (recommendation in [39b](39b-research-offline-sync.md)).
- **Pull:** `{client_id, cookie(seq), scopes}` → `{cookie, patch[], last_mutation_id}`. The patch is computed from
  `events.seq` after the cookie, filtered by the device's effective scope (tenant > workspace from the resolver of
  [35](35-tenant-defaults.md), plus grants). A NOTIFY poke over SSE tells the client to pull.
- **Push:** `{client_id, mutations:[{id(seq), name, args}]}`. They are applied in order inside `store.Write`. Each
  result (ok/rejected + reason + server row) is stored in `sync_mutations`. A retry is answered from the stored
  result.
- **Conflicts by lock mode:** `locked` reject; `overridable` LWW on `version`, the loser kept as evidence;
  `mergeable` union; `bounded` clamp and flag.
- **Retention:** `sync.retain_days` (data). Behind the horizon → `cursor_expired` → snapshot at a pinned seq.
- **Client:** Go binary; `modernc.org/sqlite`; local HTTP UI on loopback for the Tauri desktop shell (sidecar).
  The same React Aria pages read from the local API when offline, chosen by `web/src/lib/platform.ts`.

**Construction.** Server `internal/syncapi/`: `pull.go`, `push.go`, `scope.go` (resolver + grants → row filter),
`mutators.go` (registry of named mutations; each is ≤150 lines in its own file under `syncapi/mut/`), `snapshot.go`,
`poke.go`. Also `internal/configlayers/` if G-25 has not landed: `resolve.go`, `clamp.go`, `store.go`. New Go module
`devpulse/` (go.mod, `replace devagotchi/bridge => ../bridge` like `server/go.mod`): `cmd/devpulse/main.go`,
`internal/localdb/` (`open.go`, `schema.go`, `migrate.go`), `internal/syncer/` (`pull.go`, `push.go`, `outbox.go`,
`rebase.go`, `poke.go`), `internal/localapi/` (`server.go`, `routes.go`), `internal/mut/` (client mutators,
mirroring the server names). Migration `014_sync.sql`: `sync_clients(id, principal_id, device_label, tenant_id,
workspace_id, cookie_seq, last_mutation_id, created_at, revoked_at)`, `sync_mutations(client_id, mutation_id,
name, args jsonb, status, reason, server_row jsonb, applied_at, PK(client_id, mutation_id))`; `015_config_layers.sql`
(doc 35 tables) if not yet present.

**Endpoints.** `POST /api/v1/sync/pull`, `POST /api/v1/sync/push`, `GET /api/v1/sync/snapshot?scope=`,
`POST /api/v1/sync/clients` (register a device; token shown once), `DELETE /api/v1/sync/clients/{id}` (revoke and
purge-on-next-pull).

**Tests.** A table-driven conflict matrix per lock mode; a lost push ack → retry → exactly once; mutation N rejected
→ N+1 rebased; cursor expired → snapshot; scope change (workspace removed) → local purge; a property test with
two clients randomly offline converges to the server state. hubtest `sync_test.go`; `devpulse` unit tests;
`tests/devpulse-offline.sh` (network cut via a test proxy).

**Gold tickets (drafts).** GC-CM-P5-01: deploy migrations 014 and 015 and the new build; register Jesse's laptop as
the first sync client. GC-CM-P5-02: set `sync.retain_days` and the workspace scopes in the console (versioned
config, rollback ready).
