# ADR 0001: Track gaps as register rows, hub board tasks and ADRs

- **Status:** Proposed (2026-10-18), awaiting Jesse.
- **Context:** Gaps were listed only in prose ([16](../16-dc-parity.md) "Gaps left"). Nothing tied a gap to work,
  review, or a decision, so a gap could be forgotten or closed without a record.
- **Decision:** Each gap gets a stable `G-NN` row in [27](../27-gap-tracking.md), one hub board task (label `gap`)
  for the work, and an ADR in this folder when it is closed by a decision rather than by code. The parity matrix
  links `G-NN` ids. Releases list open `sev:1` gaps.
- **Consequences:** One place to read (27), one place to work (the board, with review lanes and the hash-chained
  log), one place for reasons (ADRs). Cost: keeping the register and the tasks in step by hand until an import
  script exists.
