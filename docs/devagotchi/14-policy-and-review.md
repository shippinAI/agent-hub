# 14. Policy gate and reviewer agents in the hub

The gold-policy engine (rules as JSON data, strictest wins) moves into the hub as a first-class module. Policies become
versioned, tenant-scoped rows edited in the UI. Every decision lands in the hub's hash-chained `events` log, and the bridge
authorize path asks the engine before any grant is checked, failing closed. A **reviewer** role lets agents such as
Clyffy-Infra-Auditor review change tickets through MCP. A change that needs review cannot run without a PASS, and nobody
reviews their own ticket. Researched 2026-10-18. Sources are cited inline. **Inference** marks our own design choices.

## What comparable products do

### OPA: bundles and decision logs
- Policy and data ship as **bundles** that OPA loads "on the fly without requiring a restart". Each bundle carries a
  `.manifest` with a `revision` and `roots` that scope it to part of the data tree. Bundles can be signed, and OPA keeps the
  last good bundle when it can't verify or download a new one.
  [OPA bundles](https://www.openpolicyagent.org/docs/management-bundles)
- **Decision logs**: each event records the policy path, the `input`, the `result`, the bundle `revision`, a
  `decision_id` (also returned to the caller) and a timestamp. Sensitive input is erased or masked before upload with the
  `system.log.mask` rule, and `drop` rules filter out noise.
  [OPA decision logs](https://www.openpolicyagent.org/docs/management-decision-logs)
- Takeaway: decisions are reproducible from (revision, input). Logs are separate from enforcement and redacted at the source.

### Cedar and Amazon Verified Permissions
- A request is principal, action, resource and context. The algorithm is: "If any `forbid` policy evaluates to `true`...
  `Deny`", else any satisfied `permit` allows, else **default deny**. Forbid policies act as "guardrails that permit
  policies cannot cross". Cedar *skips* a policy that errors rather than denying everything.
  [Cedar authorization](https://docs.cedarpolicy.com/auth/authorization.html)
- Verified Permissions recommends "one policy store per tenant for multi-tenant applications". Schemas validate policies
  and reject invalid ones on write.
  [AVP policy stores](https://docs.aws.amazon.com/verifiedpermissions/latest/userguide/policy-stores.html)
- Takeaway: tenant = policy store, validate on write, and keep deny-overrides semantics simple.

### Teleport access requests and reviewers
- Who may *request* and who may *review* are both role data (`allow.request.roles`, `allow.review_requests.roles`, with
  `where` filters). Approval needs **thresholds** (`approve: N`, `deny: M`) counted over reviewers that match a filter on
  `reviewer.roles`. `suggested_reviewers` routes the request. Reasons can be required.
  [Teleport access request configuration](https://goteleport.com/docs/admin-guides/access-controls/access-requests/access-request-configuration/)
- Takeaway: reviewer eligibility and quorum belong in data, and the request carries its own reason and evidence.

### GitHub required reviews
- Protected branches can require N approving reviews. A blocking "request changes" holds the merge until the same reviewer
  approves. **Stale approvals are dismissed** when the diff changes, and you can "require approval of the most recent
  reviewable push" by someone other than the pusher.
  [About protected branches](https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/managing-protected-branches/about-protected-branches)
- "Pull request authors cannot approve their own pull requests." Copilot approvals do not count toward requirements.
  [Approving with required reviews](https://docs.github.com/en/pull-requests/collaborating-with-pull-requests/reviewing-changes-in-pull-requests/approving-a-pull-request-with-required-reviews)
- Takeaway: bind each approval to a content hash, never count self-approval, and let a block hold until it is re-reviewed.

### Fail-closed admission (already used)
- Kubernetes admission webhooks with `failurePolicy: Fail` reject the request when the webhook errors or times out
  ([Kubernetes dynamic admission control](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/#failure-policy)).
  The hub keeps this rule for its own engine: no current policy version, a DB error or an unknown check all deny.

## What we build (and what each part touches)
| Part | Path | Touches |
|---|---|---|
| Engine | `server/internal/policyrules` | Pure Go: classes, regions, rules, checks. Ported from gold-policy `rules/`, with file and ticket I/O replaced by a `Facts` interface |
| Store + gate | `server/internal/changepolicy` | Tables `policy_docs`, `change_tickets`, `ticket_evidence`, `ticket_reviews` (migration 006); events `policy.*`, `ticket.*` |
| REST | `/api/v1/policy/*`, `/api/v1/change-tickets/*` | hub-server only |
| MCP | `policy_check`, `ticket_create`, `ticket_evidence_add`, `ticket_submit`, `review_queue`, `review_ticket`, `review_submit` | hub `/mcp` |
| UI | `web/src/pages/gate/*` | Policy editor, decisions, change tickets and reviews (React Aria) |
| gold-policy | `guardrails/policy/hubpdp` + `GOLD_POLICY_HUB_URL` | The hook and `check` POST to the hub. Hub down = exit 2 (deny) |

### Data model (inference, after OPA + AVP)
- `policy_docs(tenant_id, version, doc jsonb, sha256, author, reason, created_at)`, one row per version. Current = max
  version per tenant (an append-only history, like bundle revisions). An edit sends `base_version`, and a mismatch returns
  409 (optimistic concurrency). Rollback writes a new version that copies an old one. Validation runs on write (as in AVP).
- Decisions append `policy.decision` to the hash chain with `decision_id`, tenant, version, doc sha, tool, an argv
  **fingerprint** (never raw argv or output), matched rules, failures and ticket id. That gives OPA's
  (revision, input, result) shape without leaking secrets (OPA's masking idea, applied before write).
- `change_tickets(id, tenant_id, author, title, machine_id, fields, lists, sha256, status)` with statuses
  `draft -> pending_review -> passed | blocked | findings`. `sha256` covers title, machine, fields and lists. Any edit
  recomputes it and sends the ticket back to `pending_review`, so older reviews go stale (GitHub dismiss-stale).
- `ticket_evidence(ticket_id, kind, path, sha256, excerpt, captured_at, added_by)`: snapshots, topology, research.
  Engine `file_exists` checks resolve against evidence rows, because the hub can't see files on the target machine.
- `ticket_reviews(ticket_id, reviewer, verdict PASS|BLOCK|FINDINGS, findings jsonb, ticket_sha256, created_at)`, append-only.

### Review rules (code invariants vs data)
- **Code (safety invariants)**: a reviewer can't be the ticket author, and the store rejects it before any data rule runs.
  A review counts only if its `ticket_sha256` equals the current hash. Any BLOCK on the current hash denies (forbid
  overrides permit, as in Cedar). Reviews are append-only.
- **Data**: which rules require review (check kind `review` with `min` PASS count and `role`, like Teleport thresholds),
  who may review (hub data policy `role:reviewer -> change.review`), the reviewer principals, and the queue routing.
  The preset adds principal `clyffy-infra-auditor` (role `reviewer`) and a rule `infra-change-review` that needs 1 PASS
  for network/service-class commands. The UI can change any of these.
- `approval` checks in the hub mean a PASS from a **human** reviewer on the current hash (the same contract as gold's
  approver signature on the ticket sha256).

### Enforcement path
`machineops.Call -> access.Authorize -> policyCheck -> changepolicy.Gate.Decide` (tenant of the machine, the current doc,
facts from the ticket named by `ticket` on the call or `GOLD_TICKET=` in argv) `-> grant check`. If `HUB_POLICY_URL` is
also set, the external PDP must allow too (both gates, strictest wins). Errors return 503 `policy_unavailable`, a deny
returns 403 `policy_denied`, and neither consumes a grant use.

### Scaling (inference)
- Tenants scale like AVP stores: one doc chain per tenant, with an in-process engine and a per-(tenant, version) cache.
  No network hop on the authorize path.
- Reviewers scale like Teleport: add principals with the `reviewer` role, raise `min`, or add role-filtered rules
  (`role: "infra-reviewer"`). `review_queue` filters to tickets this reviewer is eligible for.
- Export: `audit anchor` (D11) already pins the chain head daily. Shipping `policy.decision` rows to an external sink is
  the OPA decision-log service pattern.

## Not done here / open
- Bundle signing (OPA `.signatures.json`) for docs pushed between hubs: not needed while there is one hub per tenant.
- Cedar-style typed schemas for rule documents: today validation is structural (`Validate()`), not schema-typed.
