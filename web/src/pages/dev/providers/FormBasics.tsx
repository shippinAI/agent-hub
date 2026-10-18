import type { FieldError, Provider } from '../../../lib/providerTypes';
import { AUTH_KINDS, EDITIONS, RUNNERS } from '../../../lib/providerTypes';
import { CheckboxGroup, Switch, TextField } from '../../../ui/Field';
import { msg, opts, type Draft, type SetDraft } from './draft';

type Props = { d: Draft; set: SetDraft; errs: FieldError[]; isNew: boolean };

// Identity, sign-in and where the client runs (doc 34 rules 1 and 4).
export function FormBasics({ d, set, errs, isNew }: Props) {
  const p = d.p;
  const put = <K extends keyof Provider>(k: K, v: Provider[K]) => set((x) => ({ ...x, p: { ...x.p, [k]: v } }));
  return (
    <fieldset className="stack-2 form-section">
      <legend className="card-title">Identity</legend>
      <div className="grid-2">
        <TextField label="ID" value={p.id} isDisabled={!isNew} onChange={(v) => put('id', v)} isRequired
          description="vendor-product, lower case" errorMessage={msg(errs, 'id')} />
        <TextField label="Label" value={p.label} onChange={(v) => put('label', v)} isRequired errorMessage={msg(errs, 'label')} />
        <TextField label="Vendor" value={p.vendor} onChange={(v) => put('vendor', v)} isRequired errorMessage={msg(errs, 'vendor')} />
        <TextField label="Product" value={p.product} onChange={(v) => put('product', v)} isRequired errorMessage={msg(errs, 'product')} />
        <TextField label="Icon" value={p.icon_ref} onChange={(v) => put('icon_ref', v)} description="e.g. icons/xai" errorMessage={msg(errs, 'icon_ref')} />
        <TextField label="Docs URL" value={p.docs_url ?? ''} onChange={(v) => put('docs_url', v)} type="url" errorMessage={msg(errs, 'docs_url')} />
      </div>
      <CheckboxGroup label="Sign-in kinds" options={opts(AUTH_KINDS)} value={p.auth_kinds}
        onChange={(v) => put('auth_kinds', v as Provider['auth_kinds'])} errorMessage={msg(errs, 'auth_kinds')} />
      <CheckboxGroup label="Runs on" options={opts(RUNNERS)} value={p.runner} description="hub is for API adapters only"
        onChange={(v) => put('runner', v as Provider['runner'])} errorMessage={msg(errs, 'runner')} />
      <CheckboxGroup label="Editions" options={opts(EDITIONS)} value={p.editions}
        onChange={(v) => put('editions', v as Provider['editions'])} errorMessage={msg(errs, 'editions')} />
      <Switch isSelected={p.host_for === 'harness'} onChange={(v) => put('host_for', v ? 'harness' : '')}>
        Runs inside another row's client (plan key in a harness)
      </Switch>
    </fieldset>
  );
}

// Plain-info terms (no badge, D6).
export function FormTerms({ d, set, errs }: Omit<Props, 'isNew'>) {
  const put = (k: 'terms_url' | 'terms_note', v: string) => set((x) => ({ ...x, p: { ...x.p, [k]: v } }));
  return (
    <fieldset className="stack-2 form-section">
      <legend className="card-title">Terms (plain info)</legend>
      <TextField label="Terms URL" value={d.p.terms_url ?? ''} onChange={(v) => put('terms_url', v)} type="url" errorMessage={msg(errs, 'terms_url')} />
      <TextField label="Terms note" value={d.p.terms_note ?? ''} onChange={(v) => put('terms_note', v)} multiline rows={2} errorMessage={msg(errs, 'terms_note')} />
    </fieldset>
  );
}
