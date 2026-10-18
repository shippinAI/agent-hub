import { get, errText } from '../../lib/api';
import { useData } from '../../lib/useData';
import type { GuiSession } from '../../lib/dtypes';
import { Badge } from '../../ui/Badge';
import { Button } from '../../ui/Button';
import { Card, Empty, Notice } from '../../ui/Notice';
import { useNow, countdown, ago } from '../../lib/time';

// GUI sessions list (07): one live screen-share lease per machine. Screenshots are sent to the agent and
// not stored, so the list shows the machine, holder, state, and the lease countdown. Open one to watch.
export function GuiSessions() {
  const live = useData(() => get<GuiSession[]>('/gui-sessions?live=true'), []);
  const now = useNow();
  if (live.error) return <Notice tone="error">{errText(live.error)}</Notice>;
  const rows = live.data;
  return (
    <div className="page">
      <header className="page-head"><h1>GUI sessions</h1></header>
      {!rows ? <Card><p role="status">Loading…</p></Card>
        : rows.length === 0 ? <Empty title="No live GUI sessions">A session appears while an agent is sharing a machine's screen.</Empty>
        : (
        <Card className="table-card">
          <table className="table">
            <thead><tr><th>Machine</th><th>Holder</th><th>State</th><th>Display</th><th>Lease</th><th></th></tr></thead>
            <tbody>
              {rows.map((s) => (
                <tr key={s.id}>
                  <td><strong>{s.machine_id}</strong><div className="hint">since {ago(s.started_at, now)}</div></td>
                  <td>{s.holder}</td>
                  <td><Badge tone={s.state === 'active' ? 'green' : 'neutral'}>{s.state}</Badge></td>
                  <td>{s.display_server ?? '—'}</td>
                  <td>{countdown(s.lease_expires_at, now).text}</td>
                  <td><a href={`#/dev/gui-session/${s.id}`}><Button size="sm">Watch</Button></a></td>
                </tr>
              ))}
            </tbody>
          </table>
        </Card>
      )}
    </div>
  );
}
