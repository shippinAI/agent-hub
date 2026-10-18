import type { Adapter, FieldError, Provider, ProviderRow } from '../../../lib/providerTypes';
import { CAPABILITIES } from '../../../lib/providerTypes';

// Form state: the row plus text buffers for the parts edited as JSON (adapter config, fallbacks, detect,
// state paths), so a half-typed value never throws away the rest of the form.
export type Draft = { p: Provider; config: string; fallback: string; detect: string; paths: string; version: number };
export type SetDraft = (fn: (d: Draft) => Draft) => void;

const pretty = (v: unknown) => JSON.stringify(v ?? {}, null, 2);

// A new row starts with the adapter defaults' shape; every value is the user's to change.
export function blankProvider(): Provider {
  return {
    schema_version: 1, id: '', vendor: '', product: '', label: '', icon_ref: 'icons/', docs_url: '',
    auth_kinds: ['client_login'], runner: ['user_device', 'hosted_workspace'], editions: ['local', 'hosted'],
    adapter: { kind: 'acp', config: { argv: [] } }, fallback_adapters: [], host_for: '',
    models: { source: 'adapter', ref: '', refresh_minutes: 360 },
    usage: { tokens: 'stream', quota: 'observed', cost_basis: 'client_estimate' },
    state: { read: 'live_only' }, detect: {},
    capabilities: Object.fromEntries(CAPABILITIES.map((c) => [c, false])) as Provider['capabilities'],
    terms_url: '', terms_note: '',
  };
}

export function toDraft(row?: ProviderRow): Draft {
  const p: Provider = row ? strip(row) : blankProvider();
  return { p, version: row?.version ?? 0, config: pretty(p.adapter?.config), fallback: JSON.stringify(p.fallback_adapters ?? [], null, 2),
    detect: pretty(p.detect), paths: pretty(p.state.paths) };
}

function strip(row: ProviderRow): Provider {
  const { version: _v, disabled: _d, disabled_at: _a, ...rest } = row as ProviderRow & { model_cache?: unknown };
  delete (rest as { model_cache?: unknown }).model_cache;
  return structuredClone(rest) as Provider;
}

function parse<T>(text: string, path: string, errs: FieldError[]): T | undefined {
  try { return JSON.parse(text || '{}') as T; } catch (e) { errs.push({ path, message: `is not valid JSON (${(e as Error).message})` }); return undefined; }
}

// Builds the row to send: JSON buffers parsed back in, empty optionals dropped.
export function fromDraft(d: Draft): { row: Provider; errors: FieldError[] } {
  const errors: FieldError[] = [];
  const row: Provider = structuredClone(d.p);
  if (row.adapter) row.adapter = { ...row.adapter, config: parse<Adapter['config']>(d.config, 'adapter.config', errors) ?? {} };
  row.fallback_adapters = parse<Adapter[]>(d.fallback || '[]', 'fallback_adapters', errors) ?? [];
  row.detect = parse<Provider['detect']>(d.detect, 'detect', errors) ?? {};
  const paths = parse<NonNullable<Provider['state']['paths']>>(d.paths, 'state.paths', errors) ?? {};
  row.state = { ...row.state, paths: Object.keys(paths).length ? paths : undefined };
  for (const k of ['docs_url', 'terms_url', 'terms_note', 'host_for'] as const) if (!row[k]) delete row[k];
  if (!row.models.ref) delete row.models.ref;
  if (!row.usage.quota_ref) delete row.usage.quota_ref;
  if (!row.state.ref) delete row.state.ref;
  if (!row.state.parser) delete row.state.parser;
  if (!row.fallback_adapters.length) delete row.fallback_adapters;
  return { row, errors };
}

// Server errors carry {path, message}; this returns the first message at exactly that path or below it.
export function msg(errs: FieldError[], path: string): string | undefined {
  return errs.find((e) => e.path === path || e.path.startsWith(`${path}.`) || e.path.startsWith(`${path}[`))?.message;
}

export const opts = (xs: readonly string[]) => xs.map((x) => ({ id: x, label: x.replace(/_/g, ' ') }));
