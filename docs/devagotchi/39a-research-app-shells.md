# 39a. Research: app shells, push for approvals, passkey signing, self-update

[Plan](39-chatmaton-gold-build-plan.md) · Read 2026-10-18. The constraints: one React Aria web UI, a Go backend on
Gold, compiled binaries, files ≤150 lines, and nothing hardcoded.

## Options at a glance
| | Tauri 2 | Wails v3 | PWA | React Native |
|---|---|---|---|---|
| Targets | macOS, Windows, Linux, iOS, Android from one project ([Tauri](https://v2.tauri.app/)) | Desktop beta; iOS and Android **experimental**, added 2026-06-13 ([commit](https://github.com/wailsapp/wails/commit/07bc6f1f503ce9d92ab277882185f111b92fbc6a), [beta post](https://v3.wails.io/blog/wails-v3-beta/)) | Every modern browser; iOS/iPadOS 26 treat any home-screen site as a web app ([OpenPWA](https://openpwa.net/reference/platforms/ios-by-version/)) | iOS, Android (desktop via forks) |
| Status 2026-10 | Stable 2.x | `v3.0.0-beta.32`, pre-release ([releases](https://github.com/wailsapp/wails/releases)) | Stable web standards | Stable |
| Our React Aria UI | Runs unchanged in the system WebView | Runs unchanged in the system WebView | Runs unchanged | **No.** React Aria is web-only; only React Stately is shared ([issue 696](https://github.com/adobe/react-spectrum/issues/696)) |
| Go backend | On Gold (HTTP). On desktop, a Go **sidecar** binary ([sidecar](https://v2.tauri.app/develop/sidecar/)). Sidecars are desktop-only ([#9774](https://github.com/tauri-apps/tauri/issues/9774)) | **In process**: Go compiled for `GOOS=ios/android` ([mobile](https://v3.wails.io/guides/mobile/)) | On Gold (HTTP) | On Gold (HTTP) |
| Shell language | Rust (thin; plugins do the work) | Go | none | JS/TS + native modules |
| Remote push | Official plugin is **local** notifications only ([notification](https://v2.tauri.app/plugin/notification/)). Push is community: `tauri-plugin-push-notifications` (APNs/FCM, 0.1.0, 2026-07) ([docs.rs](https://docs.rs/tauri-plugin-push-notifications/latest/tauri_plugin_push_notifications/)), `tauri-plugin-fcm` 0.2.0 ([crate](https://crates.io/crates/tauri-plugin-fcm)). Upstream request still open ([#11651](https://github.com/tauri-apps/tauri/issues/11651)) | Not in the mobile feature table ([mobile](https://v3.wails.io/guides/mobile/)) | Web Push. On iOS only for installed home-screen apps, after a user gesture ([Apple](https://developer.apple.com/documentation/usernotifications/sending-web-push-notifications-in-web-apps-and-browsers)). Declarative Web Push from iOS 18.4 ([WebKit](https://webkit.org/blog/16535/meet-declarative-web-push/)) | Mature (APNs/FCM libraries) |
| Biometric | Official `biometric` plugin (mobile) ([plugin](https://v2.tauri.app/plugin/biometric/)) | Not listed | Through passkey user verification (Face ID, Touch ID, Android biometrics) | Native modules |
| Passkeys | In WKWebView only if the RP ID is an **associated domain** (`webcredentials:` entitlement plus AASA file) ([passkeys.dev](https://passkeys.dev/docs/reference/ios/), [Corbado](https://www.corbado.com/blog/native-app-passkeys)). `tauri-plugin-webauthn` has no iOS ([crate](https://crates.io/crates/tauri-plugin-webauthn)) | Same WebView rule | Native browser WebAuthn, best support | Native modules |
| Self-update | Desktop updater with **minisign** signatures, keys from `tauri signer generate` ([updater](https://v2.tauri.app/plugin/updater/)) | Has an updater package (desktop) | The service worker updates the web app itself | OTA services (JS only) |
| Fit to rules | Good: one shell for 5 targets, our UI unchanged | Good for desktop; mobile not ready | Best for P2 speed | Breaks the React-Aria-only rule |

## Recommendation (**inference** from the table)
1. **P2: PWA.** It has the lowest cost and gives approvals on phone, tablet and desktop now. Web Push and passkeys are
   first-class. The service worker keeps the app shell offline. API data is never cached.
2. **P4: Tauri 2 shells** around the same `web/` build, for five targets. Native push through a community plugin
   (pinned, wrapped behind our `notify` channel adapter so it can be swapped), the biometric plugin, deep links,
   and the minisign updater on desktop. The desktop shell runs the Go `devpulse` binary as a sidecar.
3. **Wails v3: watch.** If v3.0 ships with stable mobile and push, a Wails desktop build is a cheap swap, since the UI
   is identical. Go in process would then replace the sidecar.
4. **React Native: no.** It would need a second accessible component set (`react-native-aria`, archived and moved
   to GlueStack, [repo](https://github.com/gluestack/react-native-aria)).
5. **Electron:** not asked for. It ships Chromium per app and its updater is not minisign-based. Not considered.

## Push notifications for approvals
- **Server (Go):** a `notify` package with channel adapters as data rows: `webpush` (VAPID through
  `github.com/SherClockHolmes/webpush-go`, [repo](https://github.com/SherClockHolmes/webpush-go/)), then `apns`,
  `fcm` and `ntfy` in P4. UnifiedPush/ntfy gives Android push with no Google dependency
  ([discussion in #11651](https://github.com/tauri-apps/tauri/issues/11651)).
- **Payload:** never the secret or the full command. It carries "Ticket GC-… needs your approval", the ticket sha256
  prefix and a deep link (`navigate` in Declarative Web Push). The app fetches the details after sign-in.
- **Reliability:** push is a hint, not the record. The approvals inbox is the source of truth. Unacked
  `sev:1` approvals re-notify on a schedule (data), and the feed shows "notified at / opened at".
- **iOS rules:** the permission prompt needs a user gesture inside the installed app. Explain this in the P2
  onboarding guide (an A2UI step).

## Passkey and biometric approval signing
- **Server:** `github.com/go-webauthn/webauthn`, FIDO2-conformant ([repo](https://github.com/go-webauthn/webauthn)).
- **What gets signed (inference):** the WebAuthn challenge is
  `sha256("chatmaton-approve-v1" ‖ subject_kind ‖ subject_id ‖ subject_sha256 ‖ verdict ‖ server_nonce)`. A ticket
  edit changes `sha256` (`changepolicy/tickets.go` already recomputes it on update), so an old assertion cannot
  approve new content. The server stores the full assertion (authenticatorData, clientDataJSON, signature) in
  `approval_signatures`, so an auditor can verify it again later.
- **User verification:** the policy check `approval.signed: true` requires the UV flag. This is the biometric or
  device-PIN step. A human principal is still required, as with the `human: true` invariant in
  [14](14-policy-and-review.md) and [24](24-chatmaton-spec.md).
- **RP ID:** a domain, never an IP, and the same across endpoints. This is open decision 1 in
  [39](39-chatmaton-gold-build-plan.md). In Tauri iOS it also needs the associated-domain entitlement plus an AASA
  file served by the hub.
- **Tauri biometric plugin:** a convenience unlock for the local app only. It is **not** an approval signature,
  because it proves nothing to the server.

## Self-update and self-patch
- **Desktop shells:** Tauri updater. Minisign public key in the app config. The manifest is served by the hub
  (`/api/v1/updates/{target}/{arch}/{current_version}`) ([updater](https://v2.tauri.app/plugin/updater/)).
- **Go binaries** (`hub-server`, `bridge`, `devpulse`): verify with `github.com/jedisct1/go-minisign`
  ([pkg](https://pkg.go.dev/github.com/jedisct1/go-minisign)), then swap atomically with rollback, the same pattern
  as `minio/selfupdate` ([apply.go](https://github.com/minio/selfupdate/blob/master/apply.go)). This closes
  G-09/G-10 of [27](27-gap-tracking.md).
- **Mobile:** iOS forbids downloading code that changes app features. Native updates go through the App Store,
  TestFlight or ad-hoc builds ([guidelines 2.5.2](https://developer.apple.com/app-store/review/guidelines/)).
  Play-store apps may not self-update APKs with `REQUEST_INSTALL_PACKAGES`
  ([Play policy](https://support.google.com/googleplay/android-developer/answer/12085295)). A sideloaded private APK can offer a
  user-started install. Web UI content served by the hub updates freely, like a browser.
- **Self-patch loop (inference):** a Chatmaton room drafts a patch, CI builds it, a gold-policy ticket carries the
  diff and build hashes, Jesse signs the approval with a passkey, the release is minisign-signed, and the updaters
  pull it. Signing keys never live on an agent machine (P4 ticket).
