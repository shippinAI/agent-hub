# 38. Action feed: recorded actions and learning by glimpse

[Index](../devagotchi-spec.md) · Status: spec, 2026-10-18. It extends the shipped action feed (commit 45e375e:
`server/internal/actionfeed`, `web/src/ui/actions`, Actions page). No action-feed doc existed before this one; the
code comments cite "39-action-feed.md" and "40", which were never written. This file is that spec (rename those
references in a follow-up). Threat model: [08](08-threat-model.md). A2UI carriage: [21](21-a2ui.md).

## Why
Every agent action is already recorded for governance. The same record can teach. A user who glances at the feed
sees, again and again, a plain-English line, the real command, and the two or three terms it uses. Over many
glimpses the commands, scripts and words become familiar. There are no lessons and no quizzes. The feed stays an
audit view first.

## Event schema (additions to `actionfeed.Event`)
Existing fields stay as they are. New optional fields are stored inside `data` (jsonb), so no partition change is
needed. Migration 009 only adds an index on `data->>'on_behalf_of'`.

| Field | Type | Meaning |
|---|---|---|
| `on_behalf_of` | string | Principal the actor works for (`jesse`). `actor` stays the agent (`Clyffy-Cisco-M5`) and is never replaced by the principal |
| `step` | string | Stable step id inside a run, e.g. `commands:06`, `verify:08` |
| `purpose_plain` | string | One plain-English sentence: what the command does and why |
| `purpose_source` | enum | `ticket` (comes from a reviewed steps file bound to a ticket sha256), `glossary` (hub catalog), `agent` (written by the acting agent: untrusted, labelled) |
| `terms` | `[{term, gloss}]` | Glossary entries for this command, at most 5 |
| `stdout_ref`, `stderr_ref` | string | `<path>#sha256=<hex>` of the full output; the inline `stdout`/`stderr` stay capped by `max_output_bytes` |
| `run` | object | `{ticket_id, ticket_sha256, seq, prev}` for ticketed runs; `prev` is the hash-chain link |

### Ingest of gold-rec runs (GC-20261018-00 rev 4 and later)
`gold-rec` writes `events.jsonl` with one event per executed command (fields `ts, step, actor, on_behalf_of, argv,
purpose_plain, terms, rc, duration, stdout_ref, stderr_ref` plus `seq, phase, ts_end, ticket_sha256, prev`). It is
ingested by `POST /actions/ingest` (admin only), using the rec tarball copied to the box:

| gold-rec | Event |
|---|---|
| `ts` / `ts_end` | `at` = `ts_end`; `ts` is kept in `data.started_at` |
| `rc`, `duration` (ms) | `exit_code`, `duration_ms`; `status` = `ok` if rc = 0, else `error` |
| `argv` | `argv` (scrubbed by `Scrubber.Argv`) |
| `phase` | `op` = `ticket.<phase>`, `category` = `change` (new audit category, default retention 365 d) |
| `ticket_sha256:step` | `call_id`, so a re-ingest is idempotent |
| out files | `stdout`/`stderr` = first `max_output_bytes`, scrubbed; refs kept |

Ingest rejects the batch if the `prev` chain breaks, if `seq` has gaps, if a ref's sha256 doesn't match its file, or
if `ticket_sha256` is not a ticket with a recorded approval. Rejections are themselves audited.

### Where `purpose_plain` and `terms` come from
1. **Ticketed runs:** the steps file (`<ticket>.steps.json`) lives in the bundle under SHA256SUMS. gold-rec refuses to
   run if a step's `cmd` differs from the ticket line. The explanation is therefore reviewed together with the command.
2. **Live tool calls:** the hub glossary catalog (`glossary` table, versioned) matches `argv[0]`, the subcommand and
   known flags, e.g. `install -d`, `chattr +a`, `incus exec`. Unmatched commands get no purpose line, never a guess.
3. **Agent text** (the `reason` on a call) is shown only as "Agent says: …", in plain text and with an
   `agent` badge (agents are untrusted, [08](08-threat-model.md)).

## UI
**React Aria Components only** (`react-aria-components`, already a dependency) for every interactive part. Plain
semantic HTML (`ol`, `pre`, `code`, `time`) is used for static structure. No other component or tooltip library.

### Card layout (glimpse first)
1. One line: status badge · **purpose_plain** · actor "for" on_behalf_of · time (PDT label from the user's zone).
2. A one-line code preview of the command (`<code>`, truncated with an ellipsis, monospace). This is the glimpse.
3. Term chips: the `terms`, shown in the order the command uses them.
4. Disclosures, closed by default except Output on error: **Command** (full argv, wrapped, copy button),
   **Output** (stdout/stderr, "Load full output" fetches the ref), **Policy** (decision and approver), **Run**
   (ticket, step, chain link).

### Expandable code
- `Disclosure` / `DisclosurePanel` with a `Button slot="trigger"`, as in `ActionCard` today. Every trigger has a
  heading level and a text label.
- The code block is a `<pre><code>`, horizontally scrollable, with `tabIndex=0` only when it overflows, and an
  `aria-label` like "Command for step commands:06".
- Copy uses a React Aria `Button` and confirms in the live region ("Copied").
- No syntax-highlighting library in v1. Shell tokens are bolded by a small local tokenizer: command name, flags,
  paths.

### Term tooltips
- Each chip is a React Aria `Button` inside a `TooltipTrigger` (delay 500 ms, close delay 150 ms) with a `Tooltip`
  that shows the gloss. Tooltips open on hover and keyboard focus.
- React Aria tooltips do not open on touch, so pressing the chip opens a `DialogTrigger` + `Popover` with the gloss,
  "seen N times", and a link to the Glossary page. Touch and screen-reader users get the gloss this way.
- The chip's accessible name is "Term: chattr +a"; its `aria-describedby` points at the tooltip.
- First sightings get a small "new" marker (text plus shape, not colour alone). After 5 sightings the term counts as
  familiar and the marker goes away. Counts are stored per principal on the server (`term_seen`), not in
  localStorage, so they follow the user. **Settings → Learning** can turn chips off or reset the counts.

### Accessible live feed
- The feed list is `<ol aria-label="Agent actions">`. New cards are inserted at the top **without moving focus**.
  If the user is scrolled or focused inside the list, new items are held behind a "N new actions" React Aria `Button`
  instead of shifting content.
- Announcements go through one visually hidden `<div role="status" aria-live="polite" aria-atomic="true">` owned
  by the feed (react-aria 3.53 keeps its announcer under `private/`, so it is not used). Announcements are summaries
  only ("Clyffy-Cisco-M5 for jesse: install gate binary, ok"), batched to at most one every 5 s ("3 more actions").
  Errors and denials use the same polite region with "failed" or "denied" first; `assertive` is never used.
- A React Aria `ToggleButton` "Pause live updates" stops both insertion and announcements. It is also announced.
- `prefers-reduced-motion`: no slide-in, only an instant insert. Status uses icon + text + colour. Contrast is at
  least 4.5:1, and the focus ring comes from React Aria `data-focus-visible`.
- Keyboard: Tab moves between cards' interactive elements; disclosures use Enter/Space; Escape closes popovers and
  returns focus to the chip.

### Feed filters
The existing Category/Actor `Select`s, plus **On behalf of** and **Run** (ticket). A run view shows the steps in
order with the decision-log entry under each step. The before/after snapshots appear as a diff disclosure.

## API
- `GET /actions` (unchanged long-poll), now returning the new fields.
- `GET /actions/{id}/output?stream=stdout|stderr` streams the full output from the archive, with the same
  authorization as the feed. The output is scrubbed again on read.
- `POST /actions/ingest` (admin) takes a gold-rec tarball and returns accepted/rejected with reasons.
- `GET /glossary`, `GET/POST /glossary/seen` (per principal).

## Security and privacy
- Scrubbing happens before storage and again on read. The UI never renders output as HTML or markdown.
- `on_behalf_of` is display and filter data. It never grants anything: authorization stays on the acting agent's
  grant.
- Recording is the governance record. Learning features are read-only views of it and cannot hide or reorder
  events.
- Retention for `change` defaults to 365 days with archive on (auditkeep). The learning counters hold term ids only.

## Tests
- Go: the ingest mapping, chain-break / seq-gap / ref-hash rejections, idempotent re-ingest, scrubbing of
  argv/outputs.
- Web: a vitest unit test for card folding with the new fields; Playwright + axe on the Actions page (no
  violations). Keyboard path: chip → tooltip → popover → Escape. A live-region test asserts batching to ≤1
  announcement per 5 s and no focus change on insert.
- Manual: NVDA + Firefox and VoiceOver + Safari read one card, open a term, and pause the feed.

## Open
- Glossary authoring: who reviews catalog entries. Proposal: catalog changes go through the policy review path
  ([14](14-policy-and-review.md)).
- Whether `agent`-sourced purpose lines should be shown at all by default (proposal: off; shown only in the
  Command disclosure).
