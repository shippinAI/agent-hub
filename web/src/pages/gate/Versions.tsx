import { useState } from 'react';
import { errText, get, post } from '../../lib/api';
import type { DocRow } from '../../lib/gtypes';
import { ago } from '../../lib/time';
import { useData } from '../../lib/useData';
import { Badge } from '../../ui/Badge';
import { Button } from '../../ui/Button';
import { Card, Notice } from '../../ui/Notice';

// Append-only history. Rollback writes a new version that copies an old one.
export function Versions({ current, canEdit, onSaved }: { current: DocRow; canEdit: boolean; onSaved: () => void }) {
  const vs = useData(() => get<DocRow[]>('/policy/versions'), [current.version]);
  const [err, setErr] = useState('');
  const rollback = async (to: number) => {
    setErr('');
    try { await post('/policy/rollback', { tenant_id: current.tenant_id, to_version: to, base_version: current.version, reason: 'from the UI' }); onSaved(); }
    catch (e) { setErr(errText(e)); }
  };
  return (
    <Card>
      {err && <Notice tone="error">{err}</Notice>}
      <ul className="version-list" aria-label="Policy versions">
        {(vs.data ?? []).map((v) => (
          <li key={v.version} className="row between wrap gap-2 version-row">
            <div className="stack-0">
              <div className="row gap-2"><Badge tone={v.version === current.version ? 'blue' : 'neutral'}>v{v.version}</Badge><strong>{v.reason}</strong></div>
              <span className="hint">{v.author} · {ago(v.created_at)} · <span className="mono">{v.sha256.slice(0, 12)}</span></span>
            </div>
            {canEdit && v.version !== current.version && <Button size="sm" variant="outline" onPress={() => rollback(v.version)}>Roll back to v{v.version}</Button>}
          </li>
        ))}
      </ul>
    </Card>
  );
}
