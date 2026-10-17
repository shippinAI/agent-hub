import { useState } from 'react';
import { del, errText, put } from '../lib/api';
import { useHub } from '../lib/hub';
import { Button } from '../ui/Button';
import { TextField } from '../ui/Field';
import { Card, Notice } from '../ui/Notice';

export function Roles() {
  const { config, me, reloadConfig } = useHub();
  const edit = me.can['admin.policies.put'];
  const [draft, setDraft] = useState<Record<string, string>>({});
  const [nid, setNid] = useState('');
  const [err, setErr] = useState('');
  const run = async (fn: () => Promise<unknown>) => { setErr(''); try { await fn(); reloadConfig(); } catch (e) { setErr(errText(e)); } };
  return (
    <Card title="Roles">
      <p className="hint">Roles are labels that policies refer to (<code>role:ID</code>). A role in use cannot be deleted.</p>
      <ul className="role-list">
        {config.roles.map((r) => {
          const members = config.principals.filter((p) => p.roles.includes(r.id)).map((p) => p.display_name);
          return (
            <li key={r.id} className="role-row">
              <div className="stack-0"><strong className="mono">{r.id}</strong><span className="hint">{members.join(', ') || 'no members'}</span></div>
              <TextField label="Description" value={draft[r.id] ?? r.description} onChange={(v) => setDraft({ ...draft, [r.id]: v })} isDisabled={!edit} className="grow" />
              {edit && <div className="row gap-2">
                <Button size="sm" isDisabled={(draft[r.id] ?? r.description) === r.description}
                  onPress={() => run(() => put(`/roles/${r.id}`, { ...r, description: draft[r.id] }))}>Save</Button>
                <Button size="sm" variant="ghost" isDisabled={members.length > 0} onPress={() => run(() => del(`/roles/${r.id}`))}>Delete</Button>
              </div>}
            </li>
          );
        })}
      </ul>
      {edit && <div className="row gap-2 align-end">
        <TextField label="New role id" value={nid} onChange={setNid} placeholder="deployer" className="grow" />
        <Button variant="primary" isDisabled={!nid.trim()} onPress={() => run(async () => { await put(`/roles/${nid}`, { id: nid, description: '', version: 0 }); setNid(''); })}>Add role</Button>
      </div>}
      {err && <Notice tone="error">{err}</Notice>}
    </Card>
  );
}
