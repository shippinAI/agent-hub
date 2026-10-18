# 39b. Research: offline sync for devpulse

[Plan](39-chatmaton-gold-build-plan.md) · Read 2026-10-18. Devpulse is the local-first client: it keeps a working
copy of the Gold devspace on the user's machine and syncs it back. What it syncs is scoped by tenant info, through
the layers system > plan > tenant > workspace of [35](35-tenant-defaults.md).

## What devpulse needs (requirements, inference from 35 and the goal)
1. **Offline reads and writes** for a laptop on a plane: effective config, workspace catalog rows, tasks and notes,
   room transcripts (read only), and drafts.
2. **Server authority** for anything governed: locked config items, grants, policy and approvals. These are never
   decided on the device.
3. **Tenant-scoped partial replication**: the device gets only the rows the effective config and grants allow.
4. **Durable evidence**: every accepted or rejected device write is recorded, and the hash-chained `events` log
   stays the record.
5. **Go server, compiled client**: no Node service on Gold beyond what exists, and a CGO-free client binary.

## Engines compared
| Engine | Replicated unit | Offline writes | Server side | Licence | Fit |
|---|---|---|---|---|---|
| **ElectricSQL** | Shapes: one table plus `WHERE`, an HTTP change log ([repo](https://github.com/electric-sql/electric)) | **Read path only.** Writes go through your API; TanStack DB adds an outbox ([syncular survey](https://syncular.dev/blog/offline-first-writes/)) | Elixir service next to Postgres | Apache-2.0 | Good for read replicas. Writes would still be our own |
| **PowerSync** | Bucketed row operations at checkpoints; Postgres logical replication; client SQLite with a durable `ps_crud` upload queue ([survey](https://syncular.dev/blog/offline-first-writes/)) | Yes. The app's `uploadData()` calls our API, which owns validation and conflicts ([comparison](https://queryplane.com/blog/electricsql-vs-powersync-vs-replicache/)) | PowerSync Service (Node) plus bucket storage | Service under FSL-1.1-ALv2 (no competing paid service; becomes Apache after 2 years). SDKs Apache/MIT ([open source](https://powersync.com/open-source), [FSL](https://powersync.com/legal/fsl)) | Strongest off-the-shelf choice, at the cost of one more service on Gold and no Go SDK |
| **Zero** (Replicache successor) | Live query results with incremental view maintenance | **Rejected while disconnected** ([self-host](https://zero.rocicorp.dev/docs/self-host), survey) | `zero-cache` (Node) | Open source | Not for long offline periods |
| **Replicache** | Mutation log `{clientID, mutationID, name, args}`, pull patch plus cookie, rebase ([survey](https://syncular.dev/blog/offline-first-writes/)) | Yes, server-authoritative | **Any backend** that implements push and pull | Open source, maintenance mode | The protocol is the useful part, and it is simple to implement in Go |
| **CRDT** (Automerge, Yjs) | Mergeable documents ([Automerge](https://automerge.org/), [Yjs](https://docs.yjs.dev/)) | Yes, merge without a server | Optional relay | MIT | Only for collaborative text. Governance needs server order (survey: Figma is server-authoritative) |

Patterns the survey finds in every engine, and which we adopt: visible state = confirmed base + ordered pending
intent; at-least-once delivery + durable operation identity = exactly-once application; a content-free "poke" plus
HTTP pull; snapshot plus log with a retention horizon; and routing (which rows belong on this device) treated as an
authorization question.

## Recommendation
**An own Go push/pull protocol in the Replicache style, over SQLite on the client.** PowerSync is the fallback if
devpulse grows into full multi-table relational sync on several platforms.
- **Why:** the hub already has the pieces. These are the ordered `events.seq` (a natural pull cursor), Postgres
  NOTIFY with `stream.Broker` (the poke), `store.Write` with idempotency keys (`idempotency_keys` table, migration
  001), and grants and policy for routing. One Go service and one Go client. No Node service on Gold.
- **Client store:** `modernc.org/sqlite` (pure Go, CGO off, matching the static build style of
  `hub-server.Dockerfile`).
- **Conflict rules come from doc 35 lock modes**, not from code per table:
  `locked` = server wins and the device write is rejected with evidence; `overridable` = last writer wins per key,
  using `version` (the existing optimistic-concurrency column pattern); `mergeable` = set union; `bounded` = clamp
  and flag, the same clamp as the console resolver.
- **Rejections are durable:** `sync_mutations` stores the outcome. The device shows "rejected because …" from
  the stored row, never by guessing.
- **Retention:** pull answers `cursor_expired` when a device falls behind the retained horizon. The device then
  re-bootstraps from a snapshot pinned to a seq.
- **CRDTs only where merge is the operation:** shared notes or room drafts can use Yjs or Automerge documents later,
  stored as opaque blobs inside the same protocol.

## Tradeoffs we accept
| Choice | Gain | Cost |
|---|---|---|
| Own protocol | One language, no extra service, policy-aware routing | We write and test rebase, retention and bootstrap (about 8 Go files, P5) |
| PowerSync instead | Mature checkpoints, client SDKs (JS, Kotlin, Swift, Flutter) | One more service on Gold, FSL licence, routing through stream parameters, writes still through our API |
| Electric for reads only | Cacheable HTTP shapes | Two systems (shapes plus our own write outbox) |
| No CRDT for governed rows | Approvals and grants stay server-decided | Concurrent edits of an `overridable` key: the loser is kept as evidence and shown |
