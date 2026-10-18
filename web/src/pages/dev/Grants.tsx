import { useState } from 'react';
import { get, post, errText } from '../../lib/api';
import { useData } from '../../lib/useData';
import { useNow, countdown, clock } from '../../lib/time';
import type { Grant } from '../../lib/dtypes';
import { Badge } from '../../ui/Badge';
import { Button } from '../../ui/Button';
import { Card, Empty, Notice } from '../../ui/Notice';
import { SelectorText } from './risk';

// Grants: live grants with countdown and revoke; an ended tab (07).
export function Grants() {
  const [tab, setTab] = useState<'live' | 'ended'>('live');
  const grants = useData(() => get<Grant[]>(`/grants?live=${tab === 'live'}`), [tab]);
  const now = useNow();
  const [err, setErr] = useState<string | null>(null);
  const revoke = async (g: Grant) => {
    try { await post(`/grants/${g.id}/revoke`, { reason: 'revoked from the console' }); grants.reload(); }
    catch (e) { setErr(errText(e)); }
  };
  return (
    <div className="page">
      <header className="page-head"><h1>Grants</h1>
        <div className="row gap-1">
          <Button size="sm" variant={tab === 'live' ? 'primary' : 'ghost'} onPress={() => setTab('live')}>Live</Button>
          <Button size="sm" variant={tab === 'ended' ? 'primary' : 'ghost'} onPress={() => setTab('ended')}>Ended</Button>
        </div>
      </header>
      {err && <Notice tone="error">{err}</Notice>}
      {!grants.data ? <Card><p role="status">Loading…</p></Card>
        : grants.data.length === 0 ? <Empty title={tab === 'live' ? 'No live grants' : 'No ended grants'} />
        : (
        <Card className="table-card">
          <table className="table">
            <thead><tr><th>Agent</th><th>Machine</th><th>Capability</th><th>Scope</th><th>Sandbox</th><th>{tab === 'live' ? 'Expires' : 'Ended'}</th><th></th></tr></thead>
            <tbody>
              {grants.data.map((g) => (
                <tr key={g.id} className={g.high_risk ? 'row-risk' : ''}>
                  <td><strong>{g.principal_id}</strong></td>
                  <td>{g.machine_label}</td>
                  <td>{g.capability}{g.full_access && <Badge tone="red"> full access</Badge>}</td>
                  <td><SelectorText selector={g.selector} /></td>
                  <td><Badge tone={g.sandbox_level === 'unsandboxed' ? 'red' : 'neutral'}>{g.sandbox_level}</Badge></td>
                  <td aria-live="polite">{tab === 'live'
                    ? (g.expires_at ? countdown(g.expires_at, now).text : 'until revoked')
                    : (g.ended_at ? clock(g.ended_at) : '—')}</td>
                  <td>{tab === 'live' && <Button size="sm" variant="destructive"
                    aria-label={`Revoke ${g.capability} on ${g.machine_label} for ${g.principal_id}`}
                    onPress={() => revoke(g)}>Revoke</Button>}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </Card>
      )}
    </div>
  );
}
