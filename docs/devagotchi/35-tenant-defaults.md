# 35. Tenant defaults: Shippin console, inheritance, overrides

[Index](../devagotchi-spec.md) · Requirement (Jesse, 2026-10-18): the system admin sets tenant defaults in the
Shippin console; tenants inherit them and override only where allowed; everything is data. Builds on tenants and
RLS ([05](05-subscriptions.md)), policies ([14](14-policy-and-review.md)), providers ([34](34-provider-schema.md)),
budgets ([33](33-cost-data-and-ui.md)), secrets ([36](36-vaultix.md)). Design choices are **inference** unless cited.

## Layers (top wins only where the layer above allows it)
| Layer | Who edits | Where |
|---|---|---|
| `system` | Shippin system admin | Shippin console |
| `plan` | system admin (per plan: free, team, …) | console |
| `tenant` | tenant admin | tenant settings in the hub UI |
| `workspace` / `user` | workspace owner / user, only for keys marked `user_overridable` | hub UI |
The personal edition is the same model with one tenant `local`; there the operator is also system admin.

## Merge rules (borrowed from proven hierarchies)
- Each setting has a **lock mode** set by the layer that defines it:
  `locked` (children cannot change it), `overridable` (child replaces it), `mergeable` (lists merge: child adds,
  cannot remove a parent `deny`), `bounded` (child may change within `min`/`max`/`allowed` set by the parent).
- Lists merge like Google Cloud org policy with `inheritFromParent: true`; **deny beats allow** during merge; a
  child can `reset` to the parent value ([hierarchy evaluation](https://docs.cloud.google.com/organization-policy/hierarchy-evaluation)).
- Ordered sources resolve like Doppler config inheritance: the child's own value wins, then parents in listed
  order ([inheritance](https://docs.doppler.com/docs/config-inheritance)).
- The **effective value** and the layer it came from are always shown (a `Tooltip` "from: plan team"), and an API
  returns it: `GET /api/v1/effective/{kind}/{key}` (same idea as `gcloud org-policies describe --effective`).
- Code clamps stay code (e.g. `grant_cache_seconds` ≤ 30, [01](01-data-model.md)); no layer can lift them.

## What the console configures (each a catalog kind, all rows editable)
| Catalog | Row holds | Typical lock |
|---|---|---|
| **Services catalog** | Service id, label, icon, image or package ref, ports, health check, docs URL, enabled-by-default | mergeable (tenant can add, not remove required ones) |
| **Workspace templates** | Base image, toolchains, services, default size, parameters (mutable or immutable after create), update policy | overridable; template versions are immutable once published (model: Coder templates, [templates](https://coder.com/docs/admin/templates/managing-templates), [parameters](https://coder.com/docs/admin/templates/extending-templates/parameters)) |
| **Workspace sizes** | CPU, memory, disk, GPU, idle shutdown, max lifetime | bounded by plan |
| **Toolchains** | Devcontainer Features refs with options and version pins (`ghcr.io/devcontainers/features/python:1` + `version`) ([spec](https://github.com/devcontainers/spec/blob/main/docs/specs/devcontainer-features.md), [catalog](https://containers.dev/features)) | mergeable |
| **Access endpoints** | Hostname patterns, HTTPS ingress, mesh (L1/L2, [26](26-connectivity.md)), SSH, IDE links, allowed egress | bounded (tenant narrows, cannot widen egress) |
| **Providers** | Rows of [34](34-provider-schema.md), `editions`, `runner`, allowed `auth_kinds`, router mode (default `direct`, LiteLLM optional) | mergeable (tenant can disable or add custom rows if allowed) |
| **Quotas** | Machines, workspaces, seats, concurrent sessions, storage, spend (`budgets` rows), per-user caps | bounded by plan |
| **Policies** | Policy-gate rules ([14](14-policy-and-review.md)), grant presets, sandbox policy (D1), unsandboxed allowed? | locked for safety rules; overridable for the rest |
| **Guides / bundles** | Guide channel, signing keys ([12](12-guides.md)) | locked keys; overridable channel |
| **Secrets defaults** | Vaultix project template, environments, rotation defaults ([36](36-vaultix.md)) | overridable |

## Data model
- `config_layers(id, kind system|plan|tenant|workspace|user, ref_id, parent_id, version)`.
- `config_items(id, layer_id, catalog text, key text, value jsonb, lock locked|overridable|mergeable|bounded,
  bounds jsonb null, user_overridable bool, note, version)`. Unique on `(layer_id, catalog, key)`.
- `effective_config` is computed in Go (one resolver for every catalog) and cached per `(tenant, workspace,
  user)`; any `config_items` write appends an `events` row and bumps a cache generation number.
- Catalog rows themselves (a template, a service) are versioned rows; `config_items` point to them by id, so a
  tenant pins `template: web-node@7` or follows `@latest` (policy decides).
- Schema per catalog is JSON Schema stored as data (`catalog_schemas`), so the console renders forms from it.

## Console UI (React Aria only; editors render from catalog schema)
- **Left**: catalogs (`ListBox`). **Center**: rows (`Table`) with "effective for tenant…" `Select`.
  **Right**: inspector with the value, lock mode (`RadioGroup`), bounds (`NumberField`), and inheritance chain.
- **Diff before save** in a `Modal`: which tenants change effective value, which overrides become invalid
  (a bounded value now out of range) and what happens to them (clamped, flagged, or kept with a warning).
- **Preview as tenant**: render the tenant's effective settings read-only.
- Bulk actions: apply a template version to a plan, roll back a layer to a previous version (from `events`).
- Every console write needs a human system-admin principal and lands in the hash-chained log; agents get
  read-only MCP `effective_config` only.

## Edge cases
- A parent tightens a bound below a tenant's override: the override is **clamped** and flagged (default; a
  per-catalog setting can choose "keep and warn").
- A parent removes a catalog row a tenant uses: row goes `deprecated`, keeps working until the tenant switches or
  a removal date passes; tenants get a board task.
- Plan downgrade: quotas clamp at the next period; nothing is deleted.
