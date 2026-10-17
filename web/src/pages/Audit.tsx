import { useState } from 'react';
import { get } from '../lib/api';
import { useHub } from '../lib/hub';
import { useData } from '../lib/useData';
import type { HubEvent } from '../lib/types';
import { Badge } from '../ui/Badge';
import { Button } from '../ui/Button';
import { SearchField } from '../ui/Field';
import { Card, Empty, Notice } from '../ui/Notice';
import { Select } from '../ui/Select';
import { EventRow } from './EventRow';

type Chain = { ok: boolean; count: number; head_seq: number; head_hash: string; broken_seq?: number; reason?: string };

// Append-only, hash-chained event log: live via SSE, filterable, verifiable from the UI.
export function Audit() {
  const { config } = useHub();
  const [actor, setActor] = useState('');
  const [action, setAction] = useState('');
  const ev = useData(() => get<{ events: HubEvent[] }>(`/events?order=desc&limit=300&actor=${encodeURIComponent(actor)}&action=${encodeURIComponent(action)}`)
    .then((r) => r.events), [actor, action]);
  const chain = useData(() => get<Chain>('/events/verify'));
  const c = chain.data;
  return (
    <div className="page">
      <div className="page-head">
        <div><h1 className="page-title">Audit</h1><p className="muted">Every mutation, attributed to the token's principal, in one SHA-256 chain.</p></div>
        <div className="row gap-2">
          {c && <Badge tone={c.ok ? 'green' : 'red'} title={c.head_hash}>{c.ok ? `chain ok · ${c.count} events` : `broken at #${c.broken_seq}`}</Badge>}
          <Button size="sm" variant="outline" onPress={chain.reload}>Verify chain</Button>
        </div>
      </div>
      {c && !c.ok && <Notice tone="error">Hash chain broken at event #{c.broken_seq}: {c.reason}</Notice>}
      {c?.ok && <p className="hint mono">head #{c.head_seq} {c.head_hash.slice(0, 32)}…</p>}
      <div className="toolbar">
        <Select label="Actor" hideLabel value={actor || '*'} onChange={(v) => setActor(v === '*' ? '' : v)} className="w-auto"
          options={[{ id: '*', label: 'All actors' }, { id: 'system', label: 'system (reaper)' }, ...config.principals.map((p) => ({ id: p.id, label: p.display_name }))]} />
        <SearchField label="Action prefix" value={action} onChange={setAction} placeholder="task.claimed, lease., admin." />
      </div>
      {ev.error && <Notice tone="error">{ev.error.message}</Notice>}
      <Card>
        {ev.data?.length === 0 && <Empty title="No events match" />}
        <ol className="timeline">{(ev.data ?? []).map((e) => <EventRow key={e.seq} e={e} />)}</ol>
      </Card>
    </div>
  );
}
