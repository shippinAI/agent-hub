import { useState } from 'react';
import { get } from '../../lib/api';
import { useData } from '../../lib/useData';
import { ago } from '../../lib/time';
import type { Request } from '../../lib/dtypes';
import { Badge } from '../../ui/Badge';
import { Card, Empty, Notice } from '../../ui/Notice';
import { AccessCard } from './AccessCard';

// Requests inbox: split view (list left, card right on desktop; stacked on phone) (07).
export function Requests() {
  const reqs = useData(() => get<Request[]>('/access-requests?status=pending'), []);
  const [sel, setSel] = useState<string | null>(null);
  if (reqs.error) return <Notice tone="error">{String(reqs.error)}</Notice>;
  const rows = reqs.data;
  const current = rows?.find((r) => r.id === sel) ?? rows?.[0] ?? null;
  return (
    <div className="page">
      <header className="page-head"><h1>Access requests</h1>{rows && rows.length > 0 && <Badge tone="amber">{rows.length} pending</Badge>}</header>
      {!rows ? <Card><p role="status">Loading…</p></Card>
        : rows.length === 0 ? <Empty title="No pending requests">Agents will appear here when they ask for access.</Empty>
        : (
        <div className="split">
          <ul className="req-list" aria-label="Pending requests">
            {rows.map((r) => (
              <li key={r.id}>
                <button className={`req-row ${current?.id === r.id ? 'is-active' : ''}`} onClick={() => setSel(r.id)}>
                  <div className="row between"><strong>{r.requester_name}</strong><span className="hint">{ago(r.created_at)}</span></div>
                  <div className="muted">{r.capability} · {r.machine_label}</div>
                </button>
              </li>
            ))}
          </ul>
          <div className="req-detail">{current && <AccessCard key={current.id} req={current} onDone={reqs.reload} />}</div>
        </div>
      )}
    </div>
  );
}
