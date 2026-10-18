import { useRef, useState } from 'react';
import { Form } from 'react-aria-components';
import { get, post, put, HttpError, errText } from '../../../lib/api';
import { useData } from '../../../lib/useData';
import { checkProvider } from '../../../lib/providerCheck';
import type { FieldError, ProviderRow } from '../../../lib/providerTypes';
import { Button } from '../../../ui/Button';
import { Notice } from '../../../ui/Notice';
import { fromDraft, toDraft, type Draft } from './draft';
import { FormBasics, FormTerms } from './FormBasics';
import { FormAdapter } from './FormAdapter';
import { FormSources } from './FormSources';

type Endpoint = { id: string; label: string; kind: string };

// Add or edit one provider row. Client checks run first; the hub's validator has the final word and its
// per-field errors land on the same inputs plus a summary list (announced to screen readers).
export function ProviderForm({ row, onSaved, onCancel }: { row?: ProviderRow; onSaved: (r: ProviderRow) => void; onCancel: () => void }) {
  const [d, setD] = useState<Draft>(() => toDraft(row));
  const [errs, setErrs] = useState<FieldError[]>([]);
  const [fail, setFail] = useState('');
  const [busy, setBusy] = useState(false);
  const summary = useRef<HTMLDivElement>(null);
  const eps = useData(() => get<Endpoint[]>('/router-endpoints'), [], false);
  const isNew = !row;

  const show = (list: FieldError[], text = '') => {
    setErrs(list); setFail(text);
    requestAnimationFrame(() => summary.current?.focus());
  };
  const save = async () => {
    const { row: out, errors } = fromDraft(d);
    const all = [...errors, ...checkProvider(out)];
    if (all.length) { show(all); return; }
    setBusy(true);
    try {
      const saved = isNew
        ? await post<ProviderRow>('/providers', out)
        : await put<ProviderRow>(`/providers/${encodeURIComponent(out.id)}`, { ...out, version: d.version });
      setErrs([]); setFail(''); onSaved(saved);
    } catch (e) {
      const list = e instanceof HttpError ? ((e.current as { errors?: FieldError[] } | undefined)?.errors ?? []) : [];
      show(list, errText(e));
    } finally { setBusy(false); }
  };

  return (
    <Form className="stack-3 provider-form" aria-label={isNew ? 'Add provider' : `Edit ${row?.label}`}
      onSubmit={(e) => { e.preventDefault(); void save(); }}>
      {(errs.length > 0 || fail) && (
        <div ref={summary} tabIndex={-1} className="error-summary" role="alert" aria-labelledby="provider-errors-title">
          <p id="provider-errors-title"><strong>{errs.length ? `${errs.length} problem${errs.length > 1 ? 's' : ''} to fix` : 'Not saved'}</strong></p>
          {fail && !errs.length && <Notice tone="error">{fail}</Notice>}
          <ul>{errs.map((x) => <li key={x.path + x.message}><code>{x.path}</code> {x.message}</li>)}</ul>
        </div>
      )}
      <FormBasics d={d} set={setD} errs={errs} isNew={isNew} />
      <FormAdapter d={d} set={setD} errs={errs} endpoints={eps.data ?? []} />
      <FormSources d={d} set={setD} errs={errs} />
      <FormTerms d={d} set={setD} errs={errs} />
      <div className="row end gap-2 form-actions">
        <Button variant="ghost" onPress={onCancel}>Cancel</Button>
        <Button variant="primary" type="submit" isDisabled={busy}>{busy ? 'Saving…' : isNew ? 'Add provider' : 'Save changes'}</Button>
      </div>
    </Form>
  );
}
