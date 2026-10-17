import { get } from '../lib/api';
import { setting, useHub } from '../lib/hub';
import { ago, useNow } from '../lib/time';
import { useData } from '../lib/useData';
import type { Principal, Task } from '../lib/types';
import { Avatar, Badge } from '../ui/Badge';
import { Card } from '../ui/Notice';
import { AgentEditor } from './AgentEditor';
import { Connect } from './Connect';
import { Tokens } from './Tokens';

// Agents and humans: presence (last request), current claims vs WIP limit, roles, tokens.
export function Agents() {
  const { config, me } = useHub();
  const now = useNow(5000);
  const held = useData(() => get<{ tasks: Task[] }>('/tasks?limit=1000').then((r) => r.tasks.filter((t) => t.holder)));
  const wipDefault = setting<number>(config, 'wip_limit_default', 0);
  const admin = me.can['admin.principals.update'];
  return (
    <div className="page">
      <div className="page-head">
        <div><h1 className="page-title">Agents</h1><p className="muted">{config.principals.length} principals · identity comes from each one's token</p></div>
        {admin && <AgentEditor />}
      </div>
      <div className="agent-grid">
        {config.principals.map((p: Principal) => {
          const claims = (held.data ?? []).filter((t) => t.holder === p.id);
          const seen = p.last_seen_at ? (Date.now() - new Date(p.last_seen_at).getTime()) / 1000 : Infinity;
          const presence = !p.enabled ? 'disabled' : seen < 120 ? 'online' : seen < 3600 ? 'idle' : 'offline';
          const limit = p.wip_limit ?? wipDefault;
          return (
            <Card key={p.id} className={`agent ${p.enabled ? '' : 'is-disabled'}`}>
              <div className="row between gap-2">
                <div className="row gap-2">
                  <span className={`presence presence-${presence}`}><Avatar name={p.display_name} human={p.is_human} /></span>
                  <div className="stack-0"><strong>{p.display_name}</strong><span className="hint mono">{p.id}</span></div>
                </div>
                <Badge tone={p.is_human ? 'violet' : 'blue'}>{p.kind}</Badge>
              </div>
              <div className="row wrap gap-1">{p.roles.map((r) => <Badge key={r} tone="outline">{r}</Badge>)}</div>
              <dl className="kvs">
                <div className="kv"><dt>Seen</dt><dd><span className={`hint presence-text-${presence}`}>{presence}</span> · {ago(p.last_seen_at, now)}</dd></div>
                <div className="kv"><dt>WIP</dt><dd>{claims.length} / {limit || '∞'}{p.wip_limit != null ? ' (own limit)' : ''}</dd></div>
              </dl>
              {claims.length > 0 && (
                <ul className="claims">{claims.map((t) => <li key={t.id}><a href={`#/task/${t.id}`}>#{t.id} {t.title}</a></li>)}</ul>
              )}
              {admin && (
                <div className="row wrap gap-2">
                  <AgentEditor principal={p} />
                  <Tokens principal={p} />
                  {!p.is_human && <Connect principal={p} />}
                </div>
              )}
            </Card>
          );
        })}
      </div>
    </div>
  );
}
