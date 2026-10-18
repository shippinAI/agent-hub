import { get } from '../../lib/api';
import type { Ticket } from '../../lib/gtypes';
import { statusTone } from '../../lib/gtypes';
import { go } from '../../lib/route';
import { ago } from '../../lib/time';
import { useData } from '../../lib/useData';
import { Badge } from '../../ui/Badge';
import { Card, Empty, Notice } from '../../ui/Notice';
import { ChangeDetail } from './ChangeDetail';

// Change tickets: list left, ticket with evidence, reviews and the live policy preview right (stacked on phones).
export function Changes({ id }: { id?: string }) {
  const ts = useData(() => get<Ticket[]>('/change-tickets'), []);
  if (ts.error) return <Notice tone="error">{ts.error.message}</Notice>;
  const rows = ts.data;
  const sel = id ?? rows?.[0]?.id;
  const pending = rows?.filter((t) => t.status === 'pending_review').length ?? 0;
  return (
    <div className="page">
      <header className="page-head">
        <div><h1 className="page-title">Change tickets</h1>
          <p className="muted">Gated changes wait here for a reviewer who is not the author. A PASS is bound to the ticket's exact content.</p></div>
        {pending > 0 && <Badge tone="violet">{pending} awaiting review</Badge>}
      </header>
      {!rows ? <Card><p role="status">Loading…</p></Card>
        : rows.length === 0 ? <Empty title="No change tickets">Agents file them with ticket_create when the policy asks for review.</Empty>
        : (
          <div className="split">
            <ul className="req-list" aria-label="Change tickets">
              {rows.map((t) => (
                <li key={t.id}>
                  <button className={`req-row ${sel === t.id ? 'is-active' : ''}`} onClick={() => go(`dev/changes/${t.id}`)}>
                    <div className="row between"><strong className="mono">{t.id}</strong><Badge tone={statusTone[t.status] ?? 'neutral'}>{t.status.replace('_', ' ')}</Badge></div>
                    <div className="muted">{t.title}</div>
                    <div className="hint">{t.author}{t.machine ? ` · ${t.machine}` : ''} · {ago(t.updated_at)}</div>
                  </button>
                </li>
              ))}
            </ul>
            <div className="req-detail">{sel && <ChangeDetail key={sel} id={sel} onChange={ts.reload} />}</div>
          </div>
        )}
    </div>
  );
}
