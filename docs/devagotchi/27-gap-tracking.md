# 27. How gaps and decisions are tracked (and the current register)

[Index](../devagotchi-spec.md) · Proposal for one system of record for gaps, plus the gaps known on 2026-10-18.

## The system (inference)
1. **Gap register** (this file, table below): one row per gap, ID `G-NN`, never reused. It is the human-readable
   index; status changes are made here and in the task.
2. **Hub board task per gap**: project `devagotchi`, label `gap`, plus `area:<x>` and `sev:<1-3>`. The task carries
   the work (claims, leases, notes, review lane with `distinct_from_previous`, [spec.md](../spec.md)); the hash-chained
   events log gives the history. Import fields per row: `title`, `labels`, `body` (= the row's "done when").
3. **ADR per decision** that closes or accepts a gap: `docs/devagotchi/adr/NNNN-<slug>.md` in the Nygard format
   (Title, Status, Context, Decision, Consequences) ([Nygard](https://cognitect.com/blog/2011/11/15/documenting-architecture-decisions),
   [adr.github.io](https://adr.github.io/)). Open product questions stay in [10](10-decisions.md) until Jesse answers;
   the answer becomes an ADR.
4. **Parity matrix status**: [16](16-dc-parity.md) keeps a Gaps section; each gap there links its `G-NN`.
5. **Release gate**: a release lists every open `sev:1` gap in its notes; none may be silently dropped.
ADR [0001](adr/0001-track-gaps-as-tasks-and-adrs.md) records this process.

**Not done yet:** the board tasks below are *drafted* here but not created in the hub. Creating them is a write
Jesse should approve (one import of the rows below).

## Register
| ID | Gap | Area | Sev | Source | Done when |
|---|---|---|---|---|---|
| G-01 | MCP-UI / MCP Apps widgets (file preview, config editor) not ported | bridge UI | 3 | [16](16-dc-parity.md) gap 1 | A2UI-in-MCP-Apps renderer ships ([21](21-a2ui.md)) with CSP, or ADR accepts text-only |
| G-02 | `.xls` (BIFF) read refused | formats | 3 | 16 gap 2 | ADR: keep refusing, or a pure-Go BIFF reader with fuzz tests |
| G-03 | PDF styling: no HTML/CSS/SVG; non-1252 chars print `?` | formats | 2 | 16 gap 3 | Embedded Unicode font (licence checked) and a test with CJK/emoji text |
| G-04 | DOCX create has no embedded images | formats | 3 | 16 gap 4 | Images from Markdown `![]()` embedded; round-trip test |
| G-05 | Windows OS sandbox (restricted token / AppContainer) not built; processes need `unsandboxed` | security | 1 | 16 gap 5 | Sandbox on Windows passes the X03 confinement tests |
| G-06 | Windows `list_processes` CPU/RSS are 0 | procs | 3 | 16 gap 6 | Real values from Windows APIs, test |
| G-07 | macOS/Windows built but parity suite not run there | QA | 1 | 16 gap 7, [25](25-dc-lessons.md) §5 | Parity + PTY + handle-leak tests green on both, results in `test-evidence/` |
| G-08 | `track_ui_event` dropped (telemetry) | parity | — | 16 gap 8 | Closed by ADR (deliberate; no telemetry) |
| G-09 | No release signatures (checksums only) | release | 1 | [19](19-public-product-spec.md) | Signed index + minisign + Sigstore bundle verified by installers |
| G-10 | No self-update / channels | release | 2 | 19 | `devagotchi update` with notify default, rollback, channel switch tests |
| G-11 | Config files have no `schema` version | config | 2 | 19 | `schema = 1` everywhere, refuse-newer test, converter scaffold |
| G-12 | No MCPB / plugin / extension packages | distribution | 2 | [18](18-public-product-research.md) | C3–C8 built and installed on a clean machine per client |
| G-13 | Remote connector (OAuth 2.1) for directories | distribution | 2 | 18, phase 7 | Passes Claude and OpenAI connector checks with a test account |
| G-14 | Setup components (test/apply/verify) not modelled; guides only | setup | 2 | [20](20-env-setup.md) | Component + profile schema, planner UI, idempotency test |
| G-15 | No A2UI renderer | UI | 2 | 21 | Golden-stream tests + axe pass |
| G-16 | Chatmaton tables, rooms, modes, roles, routines absent | chat | 2 | [24](24-chatmaton-spec.md) | Migrations + editors + a debate room end to end |
| G-17 | Provider rows still say OpenAI reuse "unverified" | data | 2 | [23](23-chatmaton-research.md) row 5 | `providers.json` + 05 updated after Jesse's D6 answer |
| G-18 | No mesh adapter | network | 3 | [26](26-connectivity.md) | `add-mesh` guide reaches verify on the box with a loopback test net |
| G-19 | No public website | web | 2 | [22](22-website.md) | Site live after Jesse approves the host plan |
| G-20 | Test that stdout carries only JSON-RPC; policy-parse fail-closed test | QA | 2 | 25 §1, §3 | Both tests in `bridge/parity` |
