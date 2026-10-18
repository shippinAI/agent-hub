import { useState } from 'react';
import { get, post, patch, del, errText } from '../../lib/api';
import { useData } from '../../lib/useData';
import type { Binding, Sub, ChatApp } from '../../lib/dtypes';
import { Card, Empty, Notice } from '../../ui/Notice';
import { RoleCell } from './RoleCell';

type Role = { id: string; description?: string };
type Cfg = { roles: Role[] };

// Role-bindings matrix (07): roles down, chat apps across. Each cell routes a role on one chat app to an
// ordered list of subscriptions with a fallback. Resolution picks the first enabled, healthy one.
export function RoleBindings() {
  const cfg = useData(() => get<Cfg>('/config'), []);
  const apps = useData(() => get<ChatApp[]>('/chat-apps'), []);
  const subs = useData(() => get<Sub[]>('/subscriptions'), []);
  const binds = useData(() => get<Binding[]>('/role-bindings'), []);
  const [err, setErr] = useState<string | null>(null);
  const err0 = cfg.error || apps.error || subs.error || binds.error;
  if (err0) return <Notice tone="error">{errText(err0)}</Notice>;
  if (!cfg.data || !apps.data || !subs.data || !binds.data) return <Card><p role="status">Loading…</p></Card>;
  const roles = cfg.data.roles, chatApps = apps.data, allBinds = binds.data;
  const put = async (b: Partial<Binding>) => {
    setErr(null);
    try { if (b.id) await patch(`/role-bindings/${b.id}`, b); else await post('/role-bindings', b); binds.reload(); }
    catch (e) { setErr(errText(e)); }
  };
  const remove = async (id: string) => { setErr(null); try { await del(`/role-bindings/${id}`); binds.reload(); } catch (e) { setErr(errText(e)); } };
  const cell = (role: string, app: string) => allBinds.filter((b) => b.role_id === role && b.chat_app === app).sort((a, b) => a.priority - b.priority);
  return (
    <div className="page">
      <header className="page-head"><h1>Role bindings</h1></header>
      {err && <Notice tone="error">{err}</Notice>}
      {chatApps.length === 0 ? <Empty title="No chat apps">Add a chat app in Catalog first.</Empty>
        : (
        <Card className="table-card matrix-card">
          <table className="table matrix">
            <thead><tr><th>Role</th>{chatApps.map((a) => <th key={a.id}>{a.label}</th>)}</tr></thead>
            <tbody>
              {roles.map((r) => (
                <tr key={r.id}>
                  <th scope="row"><strong>{r.id}</strong></th>
                  {chatApps.map((a) => (
                    <td key={a.id}><RoleCell role={r.id} app={a.id} bindings={cell(r.id, a.id)} subs={subs.data!}
                      onPut={put} onDel={remove} /></td>
                  ))}
                </tr>
              ))}
            </tbody>
          </table>
        </Card>
      )}
    </div>
  );
}
