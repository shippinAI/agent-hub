import { get } from '../../lib/api';
import type { DecisionEvent } from '../../lib/gtypes';
import { ago } from '../../lib/time';
import { useData } from '../../lib/useData';
import { Badge } from '../../ui/Badge';
import { Card, Empty, Notice } from '../../ui/Notice';

// The decision log: policy.decision events in the hash chain (argv as a fingerprint, never raw).
export function Decisions() {
  const ev = useData(() => get<DecisionEvent[]>('/policy/decisions?limit=100'), []);
  if (ev.error) return <Notice tone="error">{ev.error.message}</Notice>;
  const rows = ev.data ?? [];
  return (
    <Card>
      {rows.length === 0 ? <Empty title="No decisions yet">Bridge calls and hook checks appear here.</Empty> : (
        <ul className="decision-log" aria-label="Policy decisions">
          {rows.map((e) => (
            <li key={e.seq} className="decision-row">
              <div className="row between wrap gap-2">
                <div className="row gap-2">
                  <Badge tone={e.data.allow ? 'green' : 'red'}>{e.data.allow ? 'allow' : 'deny'}</Badge>
                  <strong>{e.actor}</strong><span className="mono">{e.data.tool}</span><span className="mono hint">{e.data.fp}</span>
                </div>
                <span className="hint">#{e.seq} · {ago(e.at)}</span>
              </div>
              <div className="row wrap gap-2 hint">
                {e.data.machine && <span>on {e.data.machine}</span>}
                <span>v{e.data.policy_version}</span>
                {e.data.ticket && <a href={`#/dev/changes/${e.data.ticket}`}>{e.data.ticket}</a>}
                {(e.data.failures ?? []).map((f) => <Badge key={f.rule} tone="outline">{f.rule}</Badge>)}
              </div>
            </li>
          ))}
        </ul>
      )}
    </Card>
  );
}
