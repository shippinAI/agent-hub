import { useState } from 'react';
import { errText, put } from '../lib/api';
import { useHub } from '../lib/hub';
import type { Setting } from '../lib/types';
import { Button } from '../ui/Button';
import { TextField } from '../ui/Field';
import { Card, Notice } from '../ui/Notice';

// Lease TTLs, heartbeat hint, WIP limit, expiry escalation, note/principal kinds... all rows of the settings table.
export function Knobs() {
  const { config, me, reloadConfig } = useHub();
  const edit = me.can['admin.policies.put'];
  return (
    <Card title="Leases & limits">
      <ul className="knobs">{config.settings.map((s) => <Knob key={s.key} s={s} edit={edit} onSaved={reloadConfig} />)}</ul>
    </Card>
  );
}

function Knob({ s, edit, onSaved }: { s: Setting; edit: boolean; onSaved: () => void }) {
  const isList = Array.isArray(s.value);
  const show = isList ? (s.value as unknown[]).join(', ') : JSON.stringify(s.value);
  const [v, setV] = useState(show);
  const [err, setErr] = useState('');
  const parse = () => (isList ? v.split(',').map((x) => x.trim()).filter(Boolean) : JSON.parse(v));
  return (
    <li className="knob">
      <div className="stack-0 grow"><code>{s.key}</code><span className="hint">{s.description}</span></div>
      <TextField label={s.key} value={v} onChange={setV} isDisabled={!edit} className="knob-input" />
      {edit && <Button size="sm" isDisabled={v === show} onPress={async () => {
        setErr('');
        try { await put(`/settings/${s.key}`, { value: parse(), version: s.version }); onSaved(); } catch (e) { setErr(errText(e)); }
      }}>Save</Button>}
      {err && <Notice tone="error">{err}</Notice>}
    </li>
  );
}
