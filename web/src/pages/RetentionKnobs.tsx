import { useState } from 'react';
import { errText, put } from '../lib/api';
import { useHub } from '../lib/hub';
import type { Setting } from '../lib/types';
import { Button } from '../ui/Button';
import { NumberField, TextField } from '../ui/Field';
import { Notice } from '../ui/Notice';
import { Select } from '../ui/Select';

const keys = ['audit_archive_dir', 'audit_total_max_bytes', 'audit_partition_ahead_days', 'audit_prune_interval_seconds', 'audit_on_archive_failure'];

// Global retention knobs: plain settings rows, saved with optimistic versions like every other setting.
export function RetentionKnobs() {
  const { config, reloadConfig } = useHub();
  const rows = keys.map((k) => config.settings.find((s) => s.key === k)).filter(Boolean) as Setting[];
  return <div className="knob-grid">{rows.map((s) => <Knob key={s.key} s={s} onSaved={reloadConfig} />)}</div>;
}

function Knob({ s, onSaved }: { s: Setting; onSaved: () => void }) {
  const [v, setV] = useState<unknown>(s.value);
  const [err, setErr] = useState('');
  const dirty = JSON.stringify(v) !== JSON.stringify(s.value);
  async function save() {
    try { await put(`/settings/${s.key}`, { value: v, version: s.version }); setErr(''); onSaved(); } catch (e) { setErr(errText(e)); }
  }
  const field = s.key === 'audit_on_archive_failure'
    ? <Select label={s.key} value={String(v)} onChange={setV} options={[{ id: 'drop', label: 'drop (storage never fills)' }, { id: 'keep', label: 'keep (stop pruning unarchived rows)' }]} />
    : typeof s.value === 'number'
      ? <NumberField label={s.key} description={s.description} minValue={0} value={Number(v)} onChange={setV} />
      : <TextField label={s.key} description={s.description} value={String(v ?? '')} onChange={setV} />;
  return (
    <div className="knob-cell">
      {field}
      {dirty && <Button size="sm" variant="primary" onPress={save}>Save</Button>}
      {err && <Notice tone="error">{err}</Notice>}
    </div>
  );
}
