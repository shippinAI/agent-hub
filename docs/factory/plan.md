# agent-hub as a software factory: rewrite plan

Started 2026-11-26. Baseline: the Nov 24 review (board and audit trail strong; no agent runtime, no
verification gate, no git provenance, no cost ledger, CI never ran on master). This file is the plan of
record; each increment lands as its own reviewable change and ticks its line here.

## Target architecture

Keep what is already right and build the missing factory pillars around it.

| Layer | Status | What it is |
|---|---|---|
| Board | keep | Leases, fencing, SKIP LOCKED claim-next, WIP, deps, idempotency (`server/internal/store`). |
| Audit | keep | Append-only hash-chained `events`, verify endpoint, action feed. |
| Sandbox + bridge | keep, prove in CI | bubblewrap / Seatbelt confinement, egress proxy, grants (`bridge/`). |
| Evidence gate | new | Tasks carry acceptance criteria; a lane can require evidence kinds (CI check, test run, eval, review) before `complete` moves the task. Requirements are workflow data; the refusal is a code rule. |
| Git provenance | new | Task to branch, commits, PR and check runs. Check-run results become evidence automatically. |
| Usage ledger | new | `usage_events` (tokens by kind, `billing_kind`, `cost_basis`), price catalog, `subscriptions.spend` written, budgets trip routing (docs/devagotchi/30, 33). |
| Seat runtime | new | A dispatcher claims ready tasks for configured seats and runs a headless agent (Claude Code `-p --output-format stream-json`, Codex exec) inside the bridge sandbox on a git worktree; streams the transcript, records usage, attaches evidence, opens the PR, heartbeats the lease. |
| Run records | new | Per-claim transcript and run metadata (`runs`), OpenTelemetry spans. |
| Outcome loop | new | Score each run from review verdict and CI result; feed scores into seat routing. |
| Conformance suite | new | The hubtest protocol tests driven over HTTP/MCP only, so a second implementation (Rust) is measured against the same suite. |

## Increments, in order

1. **CI that gates.** `ci` and `codeql` trigger on `master`; the runner allows unprivileged user
   namespaces so bwrap works; sandbox tests skip honestly on machines without a sandbox and fail in CI
   (`DEVAGOTCHI_REQUIRE_SANDBOX=1`). Fixes the gofmt drift and two order-dependent tests.
2. **Acceptance criteria and evidence.** `tasks.acceptance` field, `task_evidence` table (append-only,
   hash in the event chain), lane `requires_evidence` in workflow data, `complete` refuses with
   `evidence_missing`. REST, MCP and CLI.
3. **Git provenance.** `task_links` (branch, commit, PR, check run); GitHub ingest (webhook with a
   shared secret, or poll); a green required check run attaches `ci_check` evidence.
4. **Usage ledger.** `usage_events`, `prices`, spend roll-up into `subscriptions.spend`, budget trip,
   per-task and per-agent cost views.
5. **Seat runtime v1.** One adapter end to end: headless Claude Code in the sandbox on a worktree,
   lease heartbeats, transcript capture, usage events from the stream, PR opened, evidence attached.
6. **Second adapter.** Codex exec, same contract.
7. **Run records and traces.** `runs` table, transcript viewer in the UI, OTel export.
8. **Outcome loop.** Run scores, routing weights from outcomes.
9. **Conformance suite.** Extract hubtest into a black-box suite a Rust build can run.

## Open decisions (parked, the owner decides)

- **Rust and Go.** The code is Go today; this pass stays in Go and keeps every contract (REST, MCP, DB
  schema) language-neutral so a Rust twin can follow. Which one is primary is open.
- **Own org.** Moving agent-hub to its own GitHub org needs the owner's yes; nothing here assumes it.
- **One board or two.** Devagotchi's hub also has a board. Whether agent-hub stays a separate module or
  becomes the board Devagotchi embeds is open.
- **Freeze banner.** README still says the repo is frozen and superseded by Devagotchi. It stays until
  the owner says to lift it.
- **Release and images.** `release.yml` and `images.yml` still trigger on `main`, so merging to
  `master` does not start release PRs or GHCR pushes. Flip them when a real release is wanted.
