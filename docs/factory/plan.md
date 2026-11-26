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

1. **CI that gates.** Done. `ci` and `codeql` trigger on `master`; the runner allows unprivileged user
   namespaces so bwrap works; sandbox tests skip honestly on machines without a sandbox and fail in CI
   (`DEVAGOTCHI_REQUIRE_SANDBOX=1`). Fixes the gofmt drift and two order-dependent tests.
2. **Acceptance criteria and evidence.** Done. `tasks.acceptance` field, `task_evidence` table
   (append-only, each row logged as `evidence.added` in the hash chain), lane `requires_evidence` in
   workflow data, `complete` refuses with `evidence_missing`. Evidence counts only under the current
   fence, and the newest row per kind decides. REST (`POST /tasks/{id}/evidence`), MCP (`add_evidence`)
   and CLI (`hub evidence`, `hub new --accept`). The default preset requires nothing yet; turning on
   `requires_evidence: ["ci_check"]` for the build lane waits for increment 3, when CI attaches it.
3. **Git provenance.** Done. `task_links` (branch, commit, PR, check run; `link_git` over REST, MCP and
   `hub link`); signed GitHub webhook ingest at `/api/v1/integrations/github` (push, pull_request,
   check_run; deduplicated by delivery id). Each finished check attaches evidence `ci.<check name>` for
   the branch's head commit, so a lane can require e.g. `["ci.go", "ci.web"]`, and a push makes older
   results stale. Evidence only flows through links a hub principal made; a branch that merely names a
   task (`hub-12`) is linked for provenance but cannot satisfy its gate. Lane requirements stay
   per-project data, since check names differ per repo; the default preset still requires none.
4. **Usage ledger and budgets.** Done. `usage_events` (append-only, one row per model call or turn,
   with input, cache read, cache write, output and reasoning tokens, provider, model, billing kind,
   task and lease). Cost is micro-USD: as the provider charged it, computed from `price_catalog`, or
   the client's estimate, and the basis is stored. Retries are deduplicated by `external_ref` or
   `dedupe_key`. `budgets` (scope all, principal, project or task; cost or tokens; per day, week, month
   or total; humans only) stop new claims with `budget_exceeded` and log `budget.exceeded` once per
   period. REST (`POST/GET /usage`, `/budgets`, `/prices`), MCP (`record_usage`, `usage_summary`), CLI
   (`hub usage`, `hub budgets`), and task detail shows its spend. Not yet: the price catalog ships
   empty (prices are dated data with a source, not code), no monthly partitions or rollups, and
   `subscriptions.spend` is not written.
5. **Seat runtime v1.** Done. `hub-seat` (`server/cmd/hub-seat`, `internal/seat`): claim_next, a git worktree
   per task on branch `hub/<id>-<slug>`, headless Claude Code (`-p --output-format stream-json`) confined by
   bubblewrap to the worktree with egress only to allowed hosts, heartbeats with progress, transcript to a
   local JSONL file, usage per model from the stream, commit by the seat, branch and commit links, local
   checks as evidence for the head commit, push and draft PR, then complete (waiting for CI evidence up to
   `--ci-wait`) or hand off to a human. Tested end to end with a scripted agent, both unsandboxed and inside
   bwrap. Found and fixed on the way: the sandbox's egress forwarder died when `sandbox-init` exec'd the
   command, so allowlisted network never worked inside bwrap (bridge and seat now share `sandbox.Init`,
   with a regression test). Not yet: no real model run in CI (no API key there), budgets are checked at claim time, not mid-run.
6. **Second adapter.** Codex exec, same contract.
7. **Run records and traces.** Done (transcript storage). Each seat run is a `runs` row opened under the
   lease; the seat streams the agent's JSONL to the hub in ordered, retry-safe chunks while it works, and
   finishing hashes the transcript into the event chain (`run.finished`). Finished runs and chunks are
   frozen in the database. A new run on the task marks a still-open older one lost. Task detail and
   `get_task` list runs; `GET /runs/{id}` pages the transcript. Not yet: a transcript viewer in the UI,
   OTel export.
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
- **Gate on by default?** The default build lane requires no evidence. Turning it on for agent-hub's own
  project (`requires_evidence: ["ci.go", "ci.web", "ci.lint"]`) is a one-line workflow edit once the
  webhook is installed on the repo, which needs the owner's yes (it is a repo setting).
- **Release and images.** `release.yml` and `images.yml` still trigger on `main`, so merging to
  `master` does not start release PRs or GHCR pushes. Flip them when a real release is wanted.
