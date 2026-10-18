import { useState } from 'react';
import { get, post, errText } from '../../lib/api';
import { useData } from '../../lib/useData';
import type { GuideUpdate, GuideVersion, Diff } from '../../lib/dtypes';
import { Badge } from '../../ui/Badge';
import { Button } from '../../ui/Button';
import { Card, Empty, Notice } from '../../ui/Notice';
import { ago } from '../../lib/time';
import { DiffList } from './GuideVersions';

// Guide updates inbox (07): Shippin-published updates waiting on a human. Each shows change notes, an
// optional diff, and Accept / Skip / Merge-into-my-override (all human-gated by the server).
export function GuideUpdates() {
  const ups = useData(() => get<GuideUpdate[]>('/guide-updates'), []);
  const [err, setErr] = useState<string | null>(null);
  const [open, setOpen] = useState<string | null>(null);
  const [diff, setDiff] = useState<Record<string, Diff[]>>({});
  if (ups.error) return <Notice tone="error">{errText(ups.error)}</Notice>;
  const rows = ups.data;
  const decide = async (u: GuideUpdate, d: string) => {
    setErr(null); try { await post(`/guide-updates/${u.id}/${d}`); ups.reload(); } catch (e) { setErr(errText(e)); }
  };
  const viewDiff = async (u: GuideUpdate) => {
    setOpen(open === u.id ? null : u.id);
    if (diff[u.id]) return;
    try {
      const vers = await get<GuideVersion[]>(`/guides/${u.guide_id}/versions`);
      const vid = (s: string | null) => vers.find((v) => v.semver === s)?.id ?? '';
      setDiff((d) => ({ ...d, [u.id]: [] }));
      const out = await get<Diff[]>(`/guides/${u.guide_id}/diff?from=${vid(u.from_semver)}&to=${vid(u.to_semver)}`);
      setDiff((d) => ({ ...d, [u.id]: out }));
    } catch (e) { setErr(errText(e)); }
  };
  return (
    <div className="page">
      <header className="page-head"><h1>Guide updates</h1><a href="#/dev/guides"><Button variant="ghost">Guides</Button></a></header>
      {err && <Notice tone="error">{err}</Notice>}
      {!rows ? <Card><p role="status">Loading…</p></Card>
        : rows.length === 0 ? <Empty title="No updates waiting">Shippin publishes guide updates here for your review.</Empty>
        : rows.map((u) => (
          <Card key={u.id}>
            <div className="row between">
              <div><strong>{u.guide_id}</strong>{' '}
                <Badge tone="neutral">{u.from_semver ? `v${u.from_semver} → ` : ''}v{u.to_semver}</Badge>{' '}
                <Badge tone={u.status === 'pending' ? 'amber' : 'neutral'}>{u.status}</Badge></div>
              <span className="hint">{ago(u.created_at)}</span>
            </div>
            <p>{u.change_notes}</p>
            <div className="row gap-1">
              {u.status === 'pending' && <>
                <Button size="sm" variant="primary" onPress={() => decide(u, 'accept')}>Accept</Button>
                <Button size="sm" onPress={() => decide(u, 'merge')}>Merge into my override</Button>
                <Button size="sm" variant="ghost" onPress={() => decide(u, 'skip')}>Skip</Button>
              </>}
              <Button size="sm" variant="ghost" onPress={() => viewDiff(u)}>{open === u.id ? 'Hide diff' : 'View diff'}</Button>
            </div>
            {open === u.id && (diff[u.id] ? <DiffList diff={diff[u.id]} /> : <p className="hint">Loading diff…</p>)}
          </Card>
        ))}
    </div>
  );
}
