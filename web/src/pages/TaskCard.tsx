import { principalName, setting, useHub } from '../lib/hub';
import type { Task } from '../lib/types';
import { Avatar, Badge } from '../ui/Badge';
import { LeaseTimer } from './LeaseTimer';

export function Priority({ p }: { p: number }) {
  const { config } = useHub();
  const names = setting<string[]>(config, 'priority_labels', []);
  const tone = p <= 0 ? 'red' : p === 1 ? 'amber' : 'neutral';
  return <Badge tone={tone} title={names[p] ?? `priority ${p}`}>P{p}</Badge>;
}

// Card body used on the board (GridList item content).
export function TaskCard({ task }: { task: Task }) {
  const { config } = useHub();
  const holder = task.holder ? config.principals.find((p) => p.id === task.holder) : null;
  return (
    <div className="task-card">
      <div className="row between gap-2">
        <span className="task-id">#{task.id}</span>
        <div className="row gap-1"><Priority p={task.priority} />{task.expiry_count > 0 && <Badge tone="amber" title="lease expiries">↻{task.expiry_count}</Badge>}</div>
      </div>
      <p className="task-title">{task.title}</p>
      {task.labels.length > 0 && <div className="row wrap gap-1">{task.labels.map((l) => <Badge key={l} tone="outline">{l}</Badge>)}</div>}
      <div className="task-foot">
        {holder ? (
          <>
            <span className="row gap-1"><Avatar name={holder.display_name} human={holder.is_human} /><span className="holder">{holder.display_name}</span></span>
            <LeaseTimer task={task} />
          </>
        ) : task.blocked_by.length ? (
          <span className="hint">blocked by {task.blocked_by.map((d) => `#${d}`).join(', ')}</span>
        ) : task.assignee ? (
          <span className="hint">reserved for {principalName(config, task.assignee)}</span>
        ) : task.ready ? (
          <Badge tone="green">ready to claim</Badge>
        ) : <span className="hint">{task.last_worker ? `last: ${principalName(config, task.last_worker)}` : ''}</span>}
      </div>
    </div>
  );
}
