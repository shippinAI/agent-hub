import { get } from '../../lib/api';
import { useData } from '../../lib/useData';
import { ago } from '../../lib/time';
import type { Sub } from '../../lib/dtypes';
import { Badge } from '../../ui/Badge';
import { Card, Empty, Notice } from '../../ui/Notice';

// Subscriptions: provider, kind, machine, verified status, secret shown as fingerprint only (07).
export function Subscriptions() {
  const subs = useData(() => get<Sub[]>('/subscriptions'), []);
  if (subs.error) return <Notice tone="error">{String(subs.error)}</Notice>;
  const rows = subs.data;
  return (
    <div className="page">
      <header className="page-head"><h1>Subscriptions</h1></header>
      {!rows ? <Card><p role="status">Loading…</p></Card>
        : rows.length === 0 ? <Empty title="No subscriptions">Add an API key or CLI login a worker uses.</Empty>
        : (
        <Card className="table-card">
          <table className="table">
            <thead><tr><th>Subscription</th><th>Provider</th><th>Kind</th><th>Machine</th><th>Status</th><th>Secret</th></tr></thead>
            <tbody>
              {rows.map((s) => (
                <tr key={s.id}>
                  <td><strong>{s.label}</strong></td>
                  <td>{s.provider_id}</td>
                  <td><Badge tone="neutral">{s.kind === 'cli_login' ? 'CLI login' : 'API key'}</Badge></td>
                  <td>{s.machine_id ?? '—'}</td>
                  <td>{s.last_test_error
                    ? <Badge tone="red" title={s.last_test_error}>unverified</Badge>
                    : s.last_test_ok_at ? <Badge tone="green">verified · {ago(s.last_test_ok_at)}</Badge> : <Badge tone="amber">untested</Badge>}</td>
                  <td>{s.secret_fp ? <code className="hint">fp {s.secret_fp}</code> : <span className="muted">none</span>}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </Card>
      )}
    </div>
  );
}
