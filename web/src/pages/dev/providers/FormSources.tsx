import type { Capability, FieldError, Provider } from '../../../lib/providerTypes';
import { CAPABILITIES, COST_BASES, MODEL_SOURCES, QUOTA_SOURCES, STATE_READS, TOKEN_SOURCES } from '../../../lib/providerTypes';
import { CheckboxGroup, NumberField, TextField } from '../../../ui/Field';
import { Select } from '../../../ui/Select';
import { msg, opts, type Draft, type SetDraft } from './draft';

type Props = { d: Draft; set: SetDraft; errs: FieldError[] };

// Where models, usage/quota and past sessions come from, how the bridge detects the client, and the nine
// capability flags. Paths and detect rules are JSON keyed by OS (linux, macos, windows).
export function FormSources({ d, set, errs }: Props) {
  const p = d.p;
  const sub = <K extends 'models' | 'usage' | 'state'>(k: K, v: Partial<Provider[K]>) =>
    set((x) => ({ ...x, p: { ...x.p, [k]: { ...x.p[k], ...v } } }));
  const caps = CAPABILITIES.filter((c) => p.capabilities[c]);
  const setCaps = (on: string[]) => set((x) => ({ ...x, p: { ...x.p,
    capabilities: Object.fromEntries(CAPABILITIES.map((c) => [c, on.includes(c)])) as Record<Capability, boolean> } }));
  return (
    <>
      <fieldset className="stack-2 form-section">
        <legend className="card-title">Models</legend>
        <div className="grid-3">
          <Select label="Model list source" value={p.models.source} options={opts(MODEL_SOURCES)}
            onChange={(v) => sub('models', { source: v as Provider['models']['source'] })} errorMessage={msg(errs, 'models.source')} />
          <TextField label="Model list ref" value={p.models.ref ?? ''} onChange={(v) => sub('models', { ref: v })}
            description="command, URL or method" errorMessage={msg(errs, 'models.ref')} />
          <NumberField label="Refresh (minutes)" value={p.models.refresh_minutes} minValue={1} maxValue={10080}
            onChange={(v) => sub('models', { refresh_minutes: v })} errorMessage={msg(errs, 'models.refresh_minutes')} />
        </div>
      </fieldset>
      <fieldset className="stack-2 form-section">
        <legend className="card-title">Usage and quota</legend>
        <div className="grid-2">
          <Select label="Tokens from" value={p.usage.tokens} options={opts(TOKEN_SOURCES)}
            onChange={(v) => sub('usage', { tokens: v as Provider['usage']['tokens'] })} errorMessage={msg(errs, 'usage.tokens')} />
          <Select label="Cost basis" value={p.usage.cost_basis} options={opts(COST_BASES)}
            onChange={(v) => sub('usage', { cost_basis: v as Provider['usage']['cost_basis'] })} errorMessage={msg(errs, 'usage.cost_basis')} />
          <Select label="Quota from" value={p.usage.quota} options={opts(QUOTA_SOURCES)}
            onChange={(v) => sub('usage', { quota: v as Provider['usage']['quota'] })} errorMessage={msg(errs, 'usage.quota')} />
          <TextField label="Quota ref" value={p.usage.quota_ref ?? ''} onChange={(v) => sub('usage', { quota_ref: v })} errorMessage={msg(errs, 'usage.quota_ref')} />
        </div>
      </fieldset>
      <fieldset className="stack-2 form-section">
        <legend className="card-title">Session history</legend>
        <div className="grid-3">
          <Select label="Read sessions via" value={p.state.read} options={opts(STATE_READS)}
            onChange={(v) => sub('state', { read: v as Provider['state']['read'] })} errorMessage={msg(errs, 'state.read') ?? msg(errs, 'state')} />
          <TextField label="State ref" value={p.state.ref ?? ''} onChange={(v) => sub('state', { ref: v })} errorMessage={msg(errs, 'state.ref')} />
          <TextField label="Parser" value={p.state.parser ?? ''} onChange={(v) => sub('state', { parser: v })}
            description="id@version, for file reads" errorMessage={msg(errs, 'state.parser')} />
        </div>
        <TextField label="Transcript paths per OS (JSON)" value={d.paths} multiline rows={3} className="mono"
          onChange={(v) => set((x) => ({ ...x, paths: v }))} description="Never a credential file; the hub refuses those."
          errorMessage={msg(errs, 'state.paths')} />
      </fieldset>
      <fieldset className="stack-2 form-section">
        <legend className="card-title">Detection</legend>
        <TextField label="Detection rules per OS (JSON)" value={d.detect} multiline rows={5} className="mono"
          onChange={(v) => set((x) => ({ ...x, detect: v }))}
          description="bins, dirs, app_ids, version_cmd, version_regex, signed_in_cmd" errorMessage={msg(errs, 'detect')} />
      </fieldset>
      <fieldset className="stack-2 form-section">
        <legend className="card-title">Capabilities</legend>
        <CheckboxGroup label="Supported" options={opts(CAPABILITIES)} value={caps} onChange={setCaps} errorMessage={msg(errs, 'capabilities')} />
      </fieldset>
    </>
  );
}
