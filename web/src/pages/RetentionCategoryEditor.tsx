import { useState } from 'react';
import { errText, put } from '../lib/api';
import type { AuditCategory } from '../lib/actionTypes';
import { Button } from '../ui/Button';
import { NumberField, Switch, TextField } from '../ui/Field';
import { Modal } from '../ui/Modal';
import { Notice } from '../ui/Notice';

// Edit one audit category: on/off, what to capture, and the time / row / size limits (0 = no limit).
export function RetentionCategoryEditor({ c, onSaved }: { c: AuditCategory; onSaved: () => void }) {
  return (
    <Modal title={`Audit category: ${c.category}`} trigger={<Button size="sm" variant="outline" aria-label={`Edit ${c.category}`}>Edit</Button>}>
      {(close) => <Form c={c} onDone={() => { onSaved(); close(); }} onCancel={close} />}
    </Modal>
  );
}

function Form({ c, onDone, onCancel }: { c: AuditCategory; onDone: () => void; onCancel: () => void }) {
  const [v, setV] = useState(c);
  const [err, setErr] = useState('');
  const set = <K extends keyof AuditCategory>(k: K) => (x: AuditCategory[K]) => setV((s) => ({ ...s, [k]: x }));
  async function save() {
    try { await put(`/audit/categories/${c.category}`, v); onDone(); } catch (e) { setErr(errText(e)); }
  }
  return (
    <form className="stack gap-3" onSubmit={(e) => { e.preventDefault(); save(); }}>
      <Switch isSelected={v.enabled} onChange={set('enabled')}>Record this category</Switch>
      <Switch isSelected={v.capture_output} onChange={set('capture_output')}>Capture output and diffs</Switch>
      <Switch isSelected={v.archive} onChange={set('archive')}>Archive before pruning</Switch>
      <NumberField label="Keep for (days)" description="0 = no time limit" minValue={0} value={v.retain_days} onChange={set('retain_days')} />
      <NumberField label="Max rows" description="0 = no row limit" minValue={0} value={v.max_rows} onChange={set('max_rows')} />
      <NumberField label="Max bytes" description="0 = no size limit; the global cap still applies" minValue={0} value={v.max_bytes} onChange={set('max_bytes')} />
      <NumberField label="Max output bytes per stream" minValue={0} maxValue={16777216} value={v.max_output_bytes} onChange={set('max_output_bytes')} />
      <TextField label="Description" value={v.description} onChange={set('description')} />
      {err && <Notice tone="error">{err}</Notice>}
      <div className="row gap-2 justify-end">
        <Button variant="ghost" onPress={onCancel}>Cancel</Button>
        <Button variant="primary" type="submit">Save</Button>
      </div>
    </form>
  );
}
