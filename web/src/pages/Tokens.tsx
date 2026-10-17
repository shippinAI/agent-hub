import { useState } from 'react';
import { del, errText, get, post } from '../lib/api';
import { ago } from '../lib/time';
import { useData } from '../lib/useData';
import type { Principal, TokenInfo } from '../lib/types';
import { Badge } from '../ui/Badge';
import { Button } from '../ui/Button';
import { TextField } from '../ui/Field';
import { Modal } from '../ui/Modal';
import { Notice } from '../ui/Notice';

// Token list + issue (secret shown once, never stored by the hub) + revoke.
export function Tokens({ principal: p }: { principal: Principal }) {
  return (
    <Modal wide title={`Tokens · ${p.display_name}`} trigger={<Button variant="outline" size="sm">Tokens</Button>}>
      {(close) => <TokenBody p={p} close={close} />}
    </Modal>
  );
}

function TokenBody({ p, close }: { p: Principal; close: () => void }) {
  const list = useData(() => get<TokenInfo[]>(`/principals/${p.id}/tokens`), [p.id]);
  const [name, setName] = useState('');
  const [secret, setSecret] = useState('');
  const [err, setErr] = useState('');
  return (
    <div className="stack-3">
      {secret && (
        <Notice tone="success">
          <strong>Copy this token now. It is shown only once.</strong>
          <div className="row gap-2 secret"><code className="mono">{secret}</code>
            <Button size="sm" onPress={() => navigator.clipboard?.writeText(secret)}>Copy</Button></div>
        </Notice>
      )}
      <ul className="token-list">
        {(list.data ?? []).map((t) => (
          <li key={t.id} className="row between gap-2">
            <div className="stack-0"><strong>{t.name}</strong>
              <span className="hint mono">{t.prefix}… · created {ago(t.created_at)} · used {ago(t.last_used_at)}</span></div>
            {t.revoked_at ? <Badge tone="red">revoked</Badge> : (
              <Button size="sm" variant="outline" onPress={async () => {
                try { await del(`/tokens/${t.id}`); list.reload(); } catch (e) { setErr(errText(e)); }
              }}>Revoke</Button>)}
          </li>
        ))}
        {list.data?.length === 0 && <li className="muted">No tokens yet.</li>}
      </ul>
      <div className="row gap-2 align-end">
        <TextField label="New token name" value={name} onChange={setName} placeholder="laptop, ci, clyffy-01" className="grow" />
        <Button variant="primary" isDisabled={!name.trim()} onPress={async () => {
          try { const r = await post<{ token: string }>(`/principals/${p.id}/tokens`, { name }); setSecret(r.token); setName(''); list.reload(); }
          catch (e) { setErr(errText(e)); }
        }}>Issue</Button>
      </div>
      {err && <Notice tone="error">{err}</Notice>}
      <div className="dialog-actions"><Button variant="ghost" onPress={() => { setSecret(''); close(); }}>Done</Button></div>
    </div>
  );
}
