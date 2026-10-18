import { useState } from 'react';
import { errText, put } from '../../lib/api';
import type { DocRow, PolicyDoc } from '../../lib/gtypes';
import { Button } from '../../ui/Button';
import { TextField } from '../../ui/Field';
import { Notice } from '../../ui/Notice';

// Save a new policy version: needs a reason, sends base_version (a concurrent edit gets 409).
export function SaveBar({ row, doc, dirty, onSaved }: { row: DocRow; doc: PolicyDoc | null; dirty: boolean; onSaved: () => void }) {
  const [reason, setReason] = useState('');
  const [err, setErr] = useState('');
  const [ok, setOk] = useState('');
  const save = async () => {
    setErr(''); setOk('');
    try {
      const r = await put<{ version: number }>('/policy/doc', { tenant_id: row.tenant_id, base_version: row.version, doc, reason });
      setOk(`Saved as v${r.version}`); setReason(''); onSaved();
    } catch (e) { setErr(errText(e)); }
  };
  return (
    <div className="card save-bar">
      <TextField label="Reason for this change" value={reason} onChange={setReason} placeholder="e.g. allow reviewed package installs on lab boxes" />
      <div className="row gap-2">
        <Button variant="primary" isDisabled={!dirty || !doc || reason.trim().length < 3} onPress={save}>Save as v{row.version + 1}</Button>
        {!dirty && <span className="hint">No changes</span>}
      </div>
      {err && <Notice tone="error">{err}</Notice>}
      {ok && <Notice tone="success">{ok}</Notice>}
    </div>
  );
}
