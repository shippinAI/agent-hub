import { get } from '../lib/api';
import { principalName, stateKind, useHub, workflowFor } from '../lib/hub';
import { ago, clock, setServerNow, useNow } from '../lib/time';
import { useData } from '../lib/useData';
import type { HubEvent, Note, Task } from '../lib/types';
import { Badge, kindTone } from '../ui/Badge';
import { LinkButton } from '../ui/Button';
import { Card, Kv, Notice } from '../ui/Notice';
import { EventRow } from './EventRow';
import { LeaseTimer } from './LeaseTimer';
import { NoteThread } from './NoteThread';
import { TaskActions } from './TaskActions';
import { Priority } from './TaskCard';

type Detail = { task: Task; notes: Note[]; depends_on: number[]; events: HubEvent[]; server_now: string };

export function TaskDetail({ id }: { id: number }) {
  const { config } = useHub();
  const now = useNow(5000);
  const d = useData(async () => { const r = await get<Detail>(`/tasks/${id}`); setServerNow(r.server_now); return r; }, [id]);
  if (d.error) return <div className="page"><Notice tone="error">{d.error.message}</Notice></div>;
  if (!d.data) return <div className="page-loading" role="status">Loading task…</div>;
  const { task: t, notes, depends_on, events } = d.data;
  const wf = workflowFor(config, t.project);
  const st = wf?.states.find((s) => s.key === t.state);
  return (
    <div className="page">
      <LinkButton href={`#/board/${t.project}`} size="sm" className="back">← Board</LinkButton>
      <div className="page-head">
        <div className="stack-1">
          <div className="row wrap gap-2"><span className="task-id">#{t.id}</span>
            <Badge tone={kindTone[stateKind(wf, t.state)]}>{st?.label ?? t.state}</Badge><Priority p={t.priority} />
            {t.labels.map((l) => <Badge key={l} tone="outline">{l}</Badge>)}</div>
          <h1 className="page-title">{t.title}</h1>
          <p className="muted">created by {principalName(config, t.created_by)} {ago(t.created_at, now)} · v{t.version}</p>
        </div>
      </div>
      <div className="detail-grid">
        <div className="stack-4">
          <Card title="Description">
            {t.description ? <p className="prewrap">{t.description}</p> : <p className="muted">No description.</p>}
          </Card>
          <NoteThread task={t} notes={notes} onChanged={d.reload} />
        </div>
        <div className="stack-4 detail-side">
          <Card title="Lease" className={t.holder ? 'card-accent' : ''}>
            <dl className="kvs">
              <Kv k="Holder">{t.holder ? principalName(config, t.holder) : <span className="muted">unclaimed</span>}</Kv>
              {t.holder && <Kv k="Expires"><LeaseTimer task={t} /></Kv>}
              {t.heartbeat_at && <Kv k="Heartbeat">{ago(t.heartbeat_at, now)}</Kv>}
              <Kv k="Fence">{t.fence}</Kv>
              <Kv k="Expiries">{t.expiry_count}</Kv>
              {t.assignee && <Kv k="Reserved for">{principalName(config, t.assignee)}</Kv>}
              {t.last_worker && <Kv k="Last worker">{principalName(config, t.last_worker)}</Kv>}
              {depends_on.length > 0 && <Kv k="Depends on">{depends_on.map((x) => (
                <a key={x} href={`#/task/${x}`} className={`dep ${t.blocked_by.includes(x) ? 'dep-open' : ''}`}>#{x}</a>))}</Kv>}
            </dl>
            <TaskActions task={t} wf={wf} onChanged={d.reload} />
          </Card>
          <Card title="Activity">
            <ol className="timeline compact">{[...events].reverse().slice(0, 30).map((e) => <EventRow key={e.seq} e={e} compact />)}</ol>
            <p className="hint">Times: {clock(t.updated_at)} last update</p>
          </Card>
        </div>
      </div>
    </div>
  );
}
