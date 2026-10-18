import { get } from '../../lib/api';
import { useData } from '../../lib/useData';
import { ago } from '../../lib/time';
import type { Machine, Grant } from '../../lib/dtypes';
import { Badge } from '../../ui/Badge';
import { Card, Kv, Notice, Empty } from '../../ui/Notice';
import { SelectorText } from './risk';

// Machine detail: overview + GUI permission states + grants (07, trimmed to the shipped surface).
export function MachineDetail({ id }: { id: string }) {
  const m = useData(() => get<Machine>(`/machines/${id}`), [id]);
  const grants = useData(() => get<Grant[]>(`/grants?live=true&machine=${id}`), [id]);
  if (m.error) return <Notice tone="error">{String(m.error)}</Notice>;
  if (!m.data) return <Card><p role="status">Loading…</p></Card>;
  const d = m.data;
  return (
    <div className="page">
      <header className="page-head"><h1>{d.label}</h1>
        <span className={`dot ${d.connected ? 'dot-green' : 'dot-grey'}`} aria-hidden /> {d.connected ? 'connected' : 'offline'}</header>
      <Card title="Overview">
        <dl className="kv-grid">
          <Kv k="OS">{d.os} · {d.kind}</Kv>
          <Kv k="Architecture">{d.arch || '—'}</Kv>
          <Kv k="Sandbox"><Badge tone={d.sandbox_kind === 'none' ? 'red' : 'neutral'}>{d.sandbox_kind}</Badge></Kv>
          <Kv k="Bridge">{d.bridge_version || '—'}</Kv>
          <Kv k="Display">{d.display_server ?? '—'}</Kv>
          <Kv k="Last ok">{ago(d.last_exec_ok_at)}</Kv>
        </dl>
      </Card>
      <Card title="GUI permissions">
        {Object.keys(d.gui_ready ?? {}).length === 0 ? <p className="muted">No GUI helper reported.</p>
          : <ul className="gui-perms">{Object.entries(d.gui_ready).map(([k, v]) => (
            <li key={k} className="row gap-2"><Badge tone={v === 'ready' ? 'green' : 'amber'}>{String(v)}</Badge> {k}</li>))}</ul>}
      </Card>
      <Card title="Live grants">
        {!grants.data || grants.data.length === 0 ? <Empty title="No live grants" />
          : <ul className="stack-2">{grants.data.map((g) => (
            <li key={g.id} className="row gap-2"><Badge tone="blue">{g.capability}</Badge> <SelectorText selector={g.selector} /></li>))}</ul>}
      </Card>
    </div>
  );
}
