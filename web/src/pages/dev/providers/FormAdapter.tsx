import type { Adapter, AdapterKind, FieldError, Router } from '../../../lib/providerTypes';
import { ADAPTER_KINDS, ROUTER_MODES, isApiAdapter } from '../../../lib/providerTypes';
import { TextField } from '../../../ui/Field';
import { Select } from '../../../ui/Select';
import { msg, opts, type Draft, type SetDraft } from './draft';

type Endpoint = { id: string; label: string; kind: string };
type Props = { d: Draft; set: SetDraft; errs: FieldError[]; endpoints: Endpoint[] };

const NONE = 'none';
const HINT: Record<string, string> = {
  acp: 'argv, env, session_load', codex_app_server: 'argv, env, rate_limits',
  headless_json: 'argv, prompt_arg, resume_arg, output, framing, session_id_path, text_path, usage_path, event_map',
  openai_compatible: 'base_url, api, secret_ref, router', anthropic_compatible: 'base_url, secret_ref, router',
};

function readConfig(text: string): Record<string, unknown> | null {
  try { return JSON.parse(text || '{}'); } catch { return null; }
}

// Preferred adapter, its config (JSON, validated per kind by the hub), the router option on API adapters,
// and the ordered fallbacks.
export function FormAdapter({ d, set, errs, endpoints }: Props) {
  const kind = d.p.adapter?.kind ?? NONE;
  const cfg = readConfig(d.config);
  const router: Router = (cfg?.router as Router | undefined) ?? { mode: 'direct' };
  const setKind = (k: string) => set((x) => ({ ...x, p: { ...x.p, adapter: k === NONE ? null : { kind: k as AdapterKind, config: {} } as Adapter },
    config: k === NONE ? '{}' : x.config }));
  const setRouter = (r: Router) => {
    if (!cfg) return;
    const next = { ...cfg, router: r.mode === 'direct' ? { mode: 'direct' } : r };
    set((x) => ({ ...x, config: JSON.stringify(next, null, 2) }));
  };
  const eps = endpoints.filter((e) => e.kind === router.mode);
  return (
    <fieldset className="stack-2 form-section">
      <legend className="card-title">Adapter</legend>
      <Select label="Preferred adapter" value={kind} onChange={setKind} errorMessage={msg(errs, 'adapter.kind') ?? (kind === NONE ? msg(errs, 'adapter') : undefined)}
        options={[...opts(ADAPTER_KINDS), { id: NONE, label: 'none (MCP host only)' }]} />
      {kind !== NONE && (
        <TextField label="Adapter config (JSON)" value={d.config} multiline rows={6} className="mono"
          onChange={(v) => set((x) => ({ ...x, config: v }))} description={`Keys: ${HINT[kind] ?? ''}; plus timeout_ms, max_retries, backoff_ms`}
          errorMessage={msg(errs, 'adapter.config')} />
      )}
      {isApiAdapter(kind) && (
        <div className="grid-2">
          <Select label="Router" value={router.mode} options={opts(ROUTER_MODES)} isDisabled={!cfg}
            onChange={(m) => setRouter({ mode: m as Router['mode'], endpoint_id: m === router.mode ? router.endpoint_id : undefined })}
            errorMessage={msg(errs, 'adapter.config.router.mode')} />
          {router.mode !== 'direct' && (
            <Select label="Router endpoint" value={router.endpoint_id ?? null} isDisabled={!cfg}
              options={eps.map((e) => ({ id: e.id, label: e.label, hint: e.id }))}
              onChange={(id) => setRouter({ ...router, endpoint_id: id })} errorMessage={msg(errs, 'adapter.config.router.endpoint_id')} />
          )}
        </div>
      )}
      <TextField label="Fallback adapters (JSON list, tried in order)" value={d.fallback} multiline rows={4} className="mono"
        onChange={(v) => set((x) => ({ ...x, fallback: v }))} errorMessage={msg(errs, 'fallback_adapters')} />
    </fieldset>
  );
}
