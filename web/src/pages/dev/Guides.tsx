import { get, patch, errText } from '../../lib/api';
import { useData } from '../../lib/useData';
import type { Guide } from '../../lib/dtypes';
import { Badge } from '../../ui/Badge';
import { Button } from '../../ui/Button';
import { Switch } from '../../ui/Field';
import { Card, Empty, Notice } from '../../ui/Notice';
import { useState } from 'react';

// Guides admin list (07): guide, channel, published version, tenant override?, pending update, last run.
export function Guides() {
  const guides = useData(() => get<Guide[]>('/guides'), []);
  const [err, setErr] = useState<string | null>(null);
  if (guides.error) return <Notice tone="error">{errText(guides.error)}</Notice>;
  const rows = guides.data;
  const toggle = async (g: Guide, on: boolean) => {
    try { await patch(`/guides/${g.id}`, { enabled: on }); guides.reload(); } catch (e) { setErr(errText(e)); }
  };
  const pending = rows?.some((g) => g.pending_update);
  return (
    <div className="page">
      <header className="page-head"><h1>Guides</h1>
        <a href="#/dev/guide-updates"><Button variant={pending ? 'primary' : 'outline'}>Updates{pending ? ' •' : ''}</Button></a>
      </header>
      {err && <Notice tone="error">{err}</Notice>}
      {!rows ? <Card><p role="status">Loading…</p></Card>
        : rows.length === 0 ? <Empty title="No guides" />
        : (
        <Card className="table-card">
          <table className="table">
            <thead><tr><th>Guide</th><th>Channel</th><th>Published</th><th>Source</th><th>Enabled</th><th></th></tr></thead>
            <tbody>
              {rows.map((g) => (
                <tr key={g.id}>
                  <td><a href={`#/dev/guide/${g.id}`}><strong>{g.title}</strong></a><div className="hint">{g.goal}</div></td>
                  <td><Badge tone="neutral">{g.channel}</Badge></td>
                  <td>{g.semver ? `v${g.semver}` : <span className="muted">unpublished</span>}{g.pending_update && <Badge tone="amber" title="A Shippin update is waiting"> update</Badge>}</td>
                  <td>{g.overrides_guide_id ? <Badge tone="violet">override</Badge> : g.source}</td>
                  <td><Switch isSelected={g.enabled} onChange={(v) => toggle(g, v)}>{g.enabled ? 'on' : 'off'}</Switch></td>
                  <td><a href={`#/dev/guide/${g.id}`}><Button size="sm">Edit</Button></a></td>
                </tr>
              ))}
            </tbody>
          </table>
        </Card>
      )}
    </div>
  );
}
