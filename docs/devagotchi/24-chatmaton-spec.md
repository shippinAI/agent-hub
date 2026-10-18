# 24. Chatmaton spec: editions, tabs, rooms, roles, routines, approvals

[Index](../devagotchi-spec.md) · Uses the provider facts in [23](23-chatmaton-research.md) and the hub model in
[spec.md](../spec.md) (principals, roles, workflows, lanes, notes, events). Design choices are **inference** unless
cited. Nothing is hardcoded: modes, roles, providers, icons, prompts, limits and routines are data rows with presets.

## Two editions, one codebase

| | Local (personal hub) | Hosted (shippin.cloud) |
|---|---|---|
| Where models run | Official CLIs on the user's machines as **workers** (bridge exec under a grant), plus API keys via router | API keys via per-tenant router; OpenAI plan only through Sign in with ChatGPT once we are an approved partner ([help](https://help.openai.com/en/articles/20001542-using-your-chatgpt-plan-in-other-apps-and-sites)); the user's own bridges for CLI workers |
| Consumer subscriptions | Yes, per provider rule in 23 (unmodified CLI, user's own login) | Never executed on Shippin servers |
| Tenancy | `tenancy_mode=single` | `tenant_id` + RLS ([05](05-subscriptions.md)) |
| Difference in code | Auth adapter and a provider-availability flag per edition | same |
`providers.edition_availability` (data) decides which rows each edition shows; the UI explains why a row is hidden.

## Data model additions (new tables, `tenant_id` from day one per D8)
- `model_endpoints(id, provider_id, kind: cli_worker|api_router|signin_plan, model_slug, display_label, icon_ref,
  capabilities jsonb, terms_status, version)` — `icon_ref` points into an editable icon set (SVG asset rows), so the
  provider/model chip is data. Vendor logos only where the vendor's brand rules allow; otherwise a neutral glyph +
  label (**inference**; Anthropic forbids implying endorsement, [legal](https://code.claude.com/docs/en/legal-and-compliance)).
- `rooms(id, name, mode_id, participants jsonb, settings jsonb, visibility, version)`;
  `room_participants(room_id, seat, endpoint_id|principal_id, role_id, order)`.
- `room_modes(id, label, turn_policy jsonb, termination jsonb, prompts_ref, version)` — presets below.
- `room_roles(id, label, description, system_prompt_ref, permissions jsonb, may_block bool, version)`.
- `messages(id, room_id, seat, endpoint_id, role_id, content, tool_calls jsonb, usage jsonb, parent_id, created_at)`;
  every message is also an `events` row, so the hash chain covers chat.
- `routines(id, name, trigger jsonb, steps jsonb, required_roles jsonb, approvals jsonb, version)`.

## Tabs with shared state
- A **tab** is a view of one room (or one thread in it). All tabs read the same room state from the hub; switching
  tabs never forks state. Open tabs sync over the SSE stream (`room.*` events).
- Each tab header shows `ProviderChip`s (icon + provider + model label, from `model_endpoints`) for its seats, with a
  `Tooltip` for model details and terms badge (verified/unverified). A tab whose model is offline shows the honest
  presence state ([15](15-live-ops.md)).
- Phone: tabs become a `Select` of rooms; seats as a horizontal chip row.

## Room modes (presets, editable)

| Mode | Turn policy | Ends when | Source of the idea |
|---|---|---|---|
| Round robin | Seats speak in fixed order | `max_rounds` or a moderator "done" | AutoGen `RoundRobinGroupChat` ([teams](https://microsoft.github.io/autogen/stable/reference/python/autogen_agentchat.teams.html)) |
| Selector | Moderator seat picks the next speaker | termination rules | AutoGen `SelectorGroupChat` (same) |
| Debate | Each seat answers alone, then reads others and revises, for N rounds; judge seat summarises | rounds or consensus check | Multiagent debate ([Du et al., ICML 2024](https://arxiv.org/abs/2305.14325)) |
| Brainstorm | Divergent round (no critique allowed), then clustering by the scribe, then a critique round | idea budget or time | **inference** |
| Red team | Builder proposes; red team attacks; builder responds; warden rules | warden verdict | NIST red team definition ([CSRC](https://csrc.nist.gov/glossary/term/red_team)) |
| Review | Author seat, reviewer seats (≠ author), approver | required approvals reached | GitHub required reviews ([branch protection](https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/managing-protected-branches/managing-a-branch-protection-rule)) |
Termination is data like AutoGen's composable conditions (max messages, text mention, token budget, timeout,
external stop) ([termination](https://microsoft.github.io/autogen/stable/user-guide/agentchat-user-guide/tutorial/termination.html)).

## Roles (presets, editable)
Industry terms first, our own last: **moderator** (runs turns), **facilitator** (keeps brainstorm divergent),
**scribe** (writes the summary and decisions as notes), **reviewer**, **approver**, **red team**, **devil's advocate**,
**judge**, **researcher**, **builder**, and **skeptic** (asks for evidence, flags unsupported claims) and **warden**
(our term: enforces room rules and policy, may stop a turn, cannot write content). `may_block` marks roles whose "no"
holds a routine step. A role can be bound to a human or a model seat; the same role rules apply to both.

## Routines and board tasks
- A **routine** is a saved recipe: trigger (manual, schedule, event such as `task.state=review`), steps (open room
  with mode X, seat roles, prompts, produce artifact), and gates.
- A routine step may create a **board task** (existing hub task) with labels `role:<id>`; claiming follows lanes and
  `distinct_from_previous` (reviewer ≠ author) from the hub workflow (spec.md).
- **Approvals** are data: `{gate: "publish", require: [{role: "approver", count: 1, human: true},
  {role: "warden", count: 1}], distinct_from: ["author"]}`. A model may hold the reviewer role but an approval with
  `human: true` only counts from a human principal (code invariant, like policy gate approvals in [14](14-policy-and-review.md)).
- Every gate decision is an event with who, role, verdict and reason.

## UI (React Aria Components)
Rooms list (`GridList`), Room view (message list with `ProviderChip` per message, composer `TextArea`, seat panel),
Room settings (`Tabs`: Seats, Mode, Roles, Limits), Modes/Roles/Routines editors in Catalog (07), Approvals inbox
reusing the Requests inbox pattern. Agent-generated widgets in a room render as A2UI surfaces ([21](21-a2ui.md)).

## Limits and costs (data)
Per room and per seat: token budget, max turns, max parallel seats, timeout; router spend caps per virtual key
([05](05-subscriptions.md)); OpenAI per-app weekly limits are set by the user in ChatGPT, and we show them as
read-only hints when the API exposes them (**unverified** whether it does).

## Decision (Jesse, 2026-10-18)
Local edition is free, non-profit, personal use in the user's own devspace. The user signs in to each official client (Claude Code, Codex, Grok, Antigravity, Cursor) themselves; Chatmaton drives those clients and pulls/combines their state. No "unverified" badge; not a blocker.
