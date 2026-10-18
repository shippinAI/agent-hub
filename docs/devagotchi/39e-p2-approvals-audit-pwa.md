# 39e. P2: approvals and audit, mobile-friendly PWA first

[Plan](39-chatmaton-gold-build-plan.md) · Research: [39a](39a-research-app-shells.md). Builds on the shipped
`changepolicy` (tickets, reviews, decisions), `access` (requests, grants), `actionfeed`, and doc
[38](38-action-feed-learning.md). **Done when:** on Jesse's phone the hub is installed as a home-screen app. A new
ticket that needs approval sends a push. Tapping it opens the ticket. A passkey approval (Face ID) moves the ticket
to approved. The action feed shows the run with plain-English lines and term tooltips.

## Requirements (scope)
- **In:** unified approvals inbox (change tickets awaiting human review, access requests, room gates later);
  passkey-signed decisions; Web Push; PWA manifest and service worker; the doc 38 feed additions (glossary, terms,
  output refs, gold-rec ingest); `access_endpoints` rows; read-only Gold service tiles.
- **Out:** APNs/FCM native push (P4), approvals by email reply, and offline approvals. An approval always needs a
  live server round trip.

## Design
- **Inbox = a view, not a new store.** `approvals.List` unions `change_tickets` (status `submitted`, needs a human
  review on the current `sha256`), `access_requests` (`pending`) and later `room_gates`. Each item has
  `{kind, id, title, subject_sha256, risk, requested_by, on_behalf_of, age}`.
- **Signed decision flow.** (1) `POST …/challenge` returns WebAuthn request options. The challenge is the hash in
  [39a](39a-research-app-shells.md) and is stored with a 120 s TTL. (2) The browser calls
  `navigator.credentials.get` with `userVerification: "required"`. (3) `POST …/decide {verdict, note, assertion}`.
  The server verifies with go-webauthn, checks the UV flag, the sign counter and that the subject sha is
  unchanged, then calls the existing `changepolicy` review (PASS/FAIL) or `access.Approve`/`Deny` inside the same
  `store.Write`. It also stores an `approval_signatures` row. Policy docs can require `signed: true` per class
  (`policyrules` gains one check kind).
- **Push.** `notify` fans out on hub events (`ticket.submitted`, `access.requested`, budget alerts later). Channel
  adapters are data rows (`webpush` now). Payload: kind, id, sha prefix, `navigate` URL. Re-notify for unacked
  items follows the setting `notify.renotify_minutes`.
- **PWA.** `manifest.webmanifest` (`display: standalone`, icons) and `sw.js`: precache the built app shell; never
  cache `/api/*`; handle `push` and `notificationclick` (focus or open the deep link). The install and notification
  prompt is a guide step with a user gesture, as iOS requires.
- **Mobile layout.** The existing `Shell.tsx` already has a tab bar (`tab-item`). Approvals and Actions become
  first-class tabs on narrow screens. Cards follow doc 38's glimpse-first layout.

## Construction: files
**Migrations.** `009_action_feed_terms.sql` (it may land after 010: `db/pool.go` applies every unrecorded file in sorted order, so 009 must not depend on 010; doc 38: index on `data->>'on_behalf_of'`, `glossary`, `term_seen`, the
`change` audit category). `011_approvals_push.sql`:
- `passkeys(id uuid, principal_id fk, credential_id bytea unique, public_key bytea, sign_count bigint, aaguid uuid,
  transports text[], label, created_at, last_used_at, revoked_at)`
- `webauthn_challenges(id uuid, principal_id, purpose register|approve, subject jsonb, challenge bytea,
  expires_at)`
- `approval_signatures(id bigserial, subject_kind ticket|access_request|room_gate, subject_id, subject_sha256,
  principal_id, credential_id, verdict, authenticator_data bytea, client_data_json bytea, signature bytea,
  verified_at)`. Append-only (trigger refuses UPDATE and DELETE, like `events`).
- `push_subscriptions(id uuid, principal_id, channel, endpoint text, keys_ref text (secretbox), device_label,
  created_at, last_ok_at, failures int, revoked_at)` · `notify_channels(id, kind webpush|apns|fcm|ntfy, config
  jsonb, enabled, version)`
- `access_endpoints(...)` and `gold_services(...)` as in [39c](39c-architecture-on-gold.md). Seeds come from
  presets.

**Server.** `internal/approvals/`: `list.go`, `challenge.go`, `decide.go`, `types.go`. `internal/passkeys/`:
`register.go`, `assert.go`, `store.go`, `config.go` (RP ID, origins from `access_endpoints` plus setting
`passkeys.rp_id`). `internal/notify/`: `dispatch.go` (subscribes to `stream.Broker`), `webpush.go` (VAPID; private
key in `secretbox`, only the public key is ever served), `channels.go`, `payload.go`. `internal/endpoints/`:
`service.go`, `probe.go`. `internal/goldsvc/`: `service.go`, `health.go`. `internal/actionfeed/`: `ingest.go`,
`glossary.go`, `terms.go` (doc 38). `internal/policyrules/`: the `signed` check (one file). Routes:
`devapi/approval_routes.go`, `devapi/push_routes.go`, `devapi/endpoint_routes.go`.
New deps: `github.com/go-webauthn/webauthn`, `github.com/SherClockHolmes/webpush-go` (pinned; Dependabot).

**Web.** `web/public/manifest.webmanifest`, `web/public/sw.js`, `web/public/icons/*`; `web/src/lib/pwa.ts`
(register the SW, subscribe to push); `web/src/lib/webauthn.ts` (base64url helpers, get/create);
`web/src/pages/Approvals.tsx`; `pages/approvals/ApprovalCard.tsx`; `pages/approvals/DecideDialog.tsx` (`Modal`,
`RadioGroup` verdict, `TextArea` note, a "Sign with passkey" `Button`); `pages/approvals/SubjectDiff.tsx`;
`pages/settings/Passkeys.tsx`; `pages/settings/Devices.tsx` (push subscriptions, revoke);
`pages/settings/Endpoints.tsx` (`Table`, edit `Modal`); `pages/GoldServices.tsx`; doc 38 files under
`web/src/ui/actions/` (`TermChip.tsx`, `CodeBlock.tsx`, `LiveRegion.tsx`, `NewItemsButton.tsx`).

## Endpoints
| Method and path | Purpose |
|---|---|
| `GET /api/v1/approvals` | Unified inbox (filter by kind and risk) |
| `POST /api/v1/approvals/{kind}/{id}/challenge` · `POST …/decide` | Passkey-signed approve or deny |
| `GET /api/v1/approvals/{kind}/{id}/signatures` | Stored assertions for audit re-verification |
| `POST /api/v1/passkeys/register/begin` · `…/finish` · `GET/DELETE /api/v1/passkeys[/{id}]` | Manage passkeys |
| `GET /api/v1/push/vapid-public-key` · `POST/DELETE /api/v1/push/subscriptions[/{id}]` | Web Push |
| `GET/PUT /api/v1/endpoints` · `GET /api/v1/gold-services` | Endpoint rows; service tiles with health |
| `GET /api/v1/actions/{id}/output` · `POST /api/v1/actions/ingest` · `GET /api/v1/glossary` · `GET/POST /api/v1/glossary/seen` | Doc 38 |

## Testing
- **Go:** go-webauthn verified against a software authenticator (virtual key in tests); a stale sha rejects;
  a reused challenge rejects; UV missing rejects when `signed: true`; signatures are append-only; a webpush payload
  holds no command text; the doc 38 ingest tests (chain break, seq gap, ref hash).
- **hubtest:** `approvals_test.go`: submit ticket → push recorded (fake channel) → signed decide → `ticket.reviewed`
  event, and `/events/verify` ok.
- **UI:** `tests/ui/approvals-shots.mjs` (mobile viewport, axe, keyboard; Playwright virtual authenticator via CDP
  `WebAuthn.addVirtualAuthenticator`); Lighthouse PWA installability check.
- **Manual on devices:** iPhone (iOS 26) home-screen install, push, Face ID approve; Android Chrome; desktop
  Safari and Firefox.

## Gold tickets (drafts)
| Draft id | Change | Verify |
|---|---|---|
| GC-CM-P2-01 | Canonical hub domain and TLS (decision 1): split-DNS record on Gold DNS, cert via Caddy; optional Cloudflare route off | `https://<domain>/healthz` from LAN, tailnet, phone |
| GC-CM-P2-02 | VAPID key pair generated on the hub and stored in `secretbox`; only the public key is exported | `GET /push/vapid-public-key` answers; key never in logs |
| GC-CM-P2-03 | Deploy migrations 009 and 011, new hub build; set `signed: true` for the `change` class in the policy doc (a versioned edit, rollback ready) | Signed approval of a dummy ticket end to end; `events/verify` ok |
