import { principalName, useHub } from '../lib/hub';
import { clock } from '../lib/time';
import type { HubEvent } from '../lib/types';
import { Badge } from '../ui/Badge';

const tone = (a: string) =>
  a.includes('expired') || a.includes('force') ? 'red' : a.startsWith('admin') || a.startsWith('token') ? 'violet'
    : a.includes('claimed') ? 'blue' : a.includes('completed') ? 'green' : a.includes('handoff') ? 'amber' : 'neutral';

function summary(e: HubEvent): string {
  const d = e.data as Record<string, unknown>;
  if (d.from && d.to) return `${d.from} → ${d.to}`;
  if (d.title) return String(d.title);
  if (d.kind) return `${d.kind} note`;
  if (d.reason) return String(d.reason);
  if (d.key) return `${d.key} = ${JSON.stringify(d.value)}`;
  if (d.id) return String(d.id);
  return '';
}

export function EventRow({ e, compact }: { e: HubEvent; compact?: boolean }) {
  const { config } = useHub();
  return (
    <li className="event">
      <span className={`dot dot-${tone(e.action)}`} aria-hidden />
      <div className="event-body">
        <div className="row wrap gap-2">
          <Badge tone={tone(e.action)}>{e.action}</Badge>
          <strong>{principalName(config, e.actor) || e.actor}</strong>
          {!compact && e.task_id && <a href={`#/task/${e.task_id}`}>#{e.task_id}</a>}
          <span className="hint">{summary(e)}</span>
        </div>
        <div className="hint mono">#{e.seq} · {clock(e.at)}{e.fence ? ` · fence ${e.fence}` : ''}{!compact ? ` · ${e.hash.slice(0, 12)}` : ''}</div>
      </div>
    </li>
  );
}
