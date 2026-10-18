import { get, post, errText } from '../../lib/api';
import { useData } from '../../lib/useData';
import { ago } from '../../lib/time';
import type { Machine } from '../../lib/dtypes';
import { Badge } from '../../ui/Badge';
import { Button } from '../../ui/Button';
import { Card, Empty, Notice } from '../../ui/Notice';
import { AddMachine } from './AddMachine';
import { useState } from 'react';

// Machines list (07): label, OS, connected dot, verified-online time, sandbox kind, live grants, last error.
export function Machines() {
  const machines = useData(() => get<Machine[]>('/machines'), []);
  const [msg, setMsg] = useState<string | null>(null);
  if (machines.error) return <Notice tone="error">{errText(machines.error)}</Notice>;
  const rows = machines.data;
  const test = async (id: string) => {
    try { await post(`/machines/${id}/test`); setMsg(`Tested ${id}.`); } catch (e) { setMsg(errText(e)); }
  };
  const pause = async (m: Machine) => {
    try { await post(`/machines/${m.id}/${m.disabled ? 'resume' : 'pause'}`); } catch (e) { setMsg(errText(e)); }
  };
  return (
    <div className="page">
      <header className="page-head"><h1>Machines</h1><AddMachine onDone={machines.reload} /></header>
      {msg && <Notice tone="info">{msg}</Notice>}
      {!rows ? <Card><p role="status">Loading…</p></Card>
        : rows.length === 0 ? <Empty title="No machines yet">Add a machine to enrol its bridge.</Empty>
        : (
        <Card className="table-card">
          <table className="table">
            <thead><tr><th>Machine</th><th>OS</th><th>Status</th><th>Sandbox</th><th>Grants</th><th>Last error</th><th></th></tr></thead>
            <tbody>
              {rows.map((m) => (
                <tr key={m.id}>
                  <td><a href={`#/dev/machine/${m.id}`}><strong>{m.label}</strong></a></td>
                  <td>{m.os} · {m.kind}</td>
                  <td>
                    <span className={`dot ${m.connected ? 'dot-green' : 'dot-grey'}`} aria-hidden /> {m.online ? 'Online' : m.connected ? 'connected' : 'offline'}
                    {m.online && <span className="hint"> · ok {ago(m.last_exec_ok_at)}</span>}
                  </td>
                  <td><Badge tone={m.sandbox_kind === 'none' ? 'red' : 'neutral'}>{m.sandbox_kind}</Badge></td>
                  <td>{m.live_grants}{m.high_risk_grants > 0 && <Badge tone="red" title="high risk"> {m.high_risk_grants}!</Badge>}</td>
                  <td className="muted">{m.last_error ?? '—'}</td>
                  <td className="row gap-1">
                    <Button size="sm" onPress={() => test(m.id)}>Test</Button>
                    <Button size="sm" variant="ghost" onPress={() => pause(m)}>{m.disabled ? 'Resume' : 'Pause'}</Button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </Card>
      )}
    </div>
  );
}
