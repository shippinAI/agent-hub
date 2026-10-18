import { useState } from 'react';
import { get, post, errText } from '../../lib/api';
import { useData } from '../../lib/useData';
import type { GuideVersion, Diff } from '../../lib/dtypes';
import { Badge } from '../../ui/Badge';
import { Button } from '../../ui/Button';
import { Select } from '../../ui/Select';
import { Card, Notice } from '../../ui/Notice';

const tone: Record<string, 'green' | 'amber' | 'neutral'> = { published: 'green', draft: 'amber', retired: 'neutral' };

// Versions tab: full history with status, publish a draft, retire a published version, and diff any two.
export function GuideVersions({ guideId, onChanged }: { guideId: string; onChanged: () => void }) {
  const vers = useData(() => get<GuideVersion[]>(`/guides/${guideId}/versions`), [guideId]);
  const [from, setFrom] = useState<string | null>(null);
  const [to, setTo] = useState<string | null>(null);
  const [diff, setDiff] = useState<Diff[] | null>(null);
  const [err, setErr] = useState<string | null>(null);
  if (vers.error) return <Notice tone="error">{errText(vers.error)}</Notice>;
  const rows = vers.data ?? [];
  const act = async (fn: () => Promise<unknown>) => {
    setErr(null); try { await fn(); vers.reload(); onChanged(); } catch (e) { setErr(errText(e)); }
  };
  const runDiff = async () => {
    setErr(null); try { setDiff(await get<Diff[]>(`/guides/${guideId}/diff?from=${from}&to=${to}`)); } catch (e) { setErr(errText(e)); }
  };
  const opts = rows.map((v) => ({ id: v.id, label: `v${v.semver} (${v.status})` }));
  return (
    <div className="stack-2">
      {err && <Notice tone="error">{err}</Notice>}
      <Card className="table-card">
        <table className="table">
          <thead><tr><th>Version</th><th>Status</th><th>Change notes</th><th>Author</th><th></th></tr></thead>
          <tbody>
            {rows.map((v) => (
              <tr key={v.id}>
                <td><strong>v{v.semver}</strong></td>
                <td><Badge tone={tone[v.status] ?? 'neutral'}>{v.status}</Badge></td>
                <td>{v.change_notes}</td>
                <td className="hint">{v.authored_by}</td>
                <td><div className="row gap-1">
                  {v.status === 'draft' && <Button size="sm" variant="primary" onPress={() => act(() => post(`/guides/${guideId}/versions/${v.id}/publish`))}>Publish</Button>}
                  {v.status === 'published' && <Button size="sm" variant="ghost" onPress={() => act(() => post(`/guides/${guideId}/versions/${v.id}/retire`))}>Retire</Button>}
                </div></td>
              </tr>
            ))}
          </tbody>
        </table>
      </Card>
      <Card title="Diff two versions">
        <div className="row gap-2 wrap">
          <Select label="From" options={opts} value={from} onChange={setFrom} className="w-sm" />
          <Select label="To" options={opts} value={to} onChange={setTo} className="w-sm" />
          <Button isDisabled={!from || !to} onPress={runDiff}>Compare</Button>
        </div>
        {diff && <DiffList diff={diff} />}
      </Card>
    </div>
  );
}

export function DiffList({ diff }: { diff: Diff[] }) {
  if (diff.length === 0) return <p className="hint">No differences.</p>;
  return (
    <ul className="diff-list">
      {diff.map((d, i) => (
        <li key={i} className={`diff-row diff-${d.change}`}>
          <Badge tone={d.change === 'added' ? 'green' : d.change === 'removed' ? 'red' : 'amber'}>{d.change}</Badge>
          <code>{d.step}</code>
          {d.before != null && <pre className="code-box">- {JSON.stringify(d.before)}</pre>}
          {d.after != null && <pre className="code-box">+ {JSON.stringify(d.after)}</pre>}
        </li>
      ))}
    </ul>
  );
}
