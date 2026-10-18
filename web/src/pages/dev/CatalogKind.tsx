import { useState } from 'react';
import { get, post, put, del, errText } from '../../lib/api';
import { useData } from '../../lib/useData';
import type { Spec, Field } from './catalogSpec';
import { Button } from '../../ui/Button';
import { TextField, Switch } from '../../ui/Field';
import { Card, Empty, Notice } from '../../ui/Notice';
import { Modal } from '../../ui/Modal';

type Row = Record<string, unknown>;

// Generic list + editor for one catalog kind (capabilities, risks, shell profiles, …). Each row is an
// editable preset; JSON and list fields round-trip as text. Nothing here is hardcoded policy.
export function CatalogKind({ spec }: { spec: Spec }) {
  const rows = useData(() => get<Row[]>(`/${spec.kind}`), [spec.kind]);
  const [err, setErr] = useState<string | null>(null);
  if (rows.error) return <Notice tone="error">{errText(rows.error)}</Notice>;
  const data = rows.data;
  const remove = async (id: string) => { setErr(null); try { await del(`/${spec.kind}/${id}`); rows.reload(); } catch (e) { setErr(errText(e)); } };
  return (
    <div className="stack-2">
      <div className="row between"><h2 className="card-title">{spec.label}</h2>
        <Modal trigger={<Button variant="primary" size="sm">Add</Button>} title={`New ${spec.label}`}>
          {(close) => <EditRow spec={spec} row={{}} close={close} onSaved={rows.reload} onErr={setErr} />}
        </Modal></div>
      {err && <Notice tone="error">{err}</Notice>}
      {!data ? <Card><p role="status">Loading…</p></Card>
        : data.length === 0 ? <Empty title={`No ${spec.label.toLowerCase()}`} />
        : (
        <Card className="table-card">
          <table className="table"><thead><tr><th>{spec.title}</th><th>ID</th><th></th></tr></thead>
            <tbody>{data.map((r) => (
              <tr key={String(r.id)}>
                <td><strong>{String(r[spec.title] ?? r.id)}</strong></td>
                <td className="hint mono">{String(r.id)}</td>
                <td><div className="row gap-1">
                  <Modal trigger={<Button size="sm">Edit</Button>} title={`Edit ${spec.label}`}>
                    {(close) => <EditRow spec={spec} row={r} close={close} onSaved={rows.reload} onErr={setErr} />}
                  </Modal>
                  <Button size="sm" variant="ghost" onPress={() => remove(String(r.id))}>Delete</Button>
                </div></td>
              </tr>))}</tbody>
          </table>
        </Card>
      )}
    </div>
  );
}

function fieldValue(row: Row, f: Field): string {
  const v = row[f.key];
  if (v == null) return '';
  if (f.type === 'json' || f.type === 'list') return typeof v === 'string' ? v : JSON.stringify(v);
  return String(v);
}

function EditRow({ spec, row, close, onSaved, onErr }: { spec: Spec; row: Row; close: () => void; onSaved: () => void; onErr: (e: string) => void }) {
  const [draft, setDraft] = useState<Record<string, string>>(() =>
    Object.fromEntries(spec.fields.map((f) => [f.key, fieldValue(row, f)])));
  const [bools, setBools] = useState<Record<string, boolean>>(() =>
    Object.fromEntries(spec.fields.filter((f) => f.type === 'bool').map((f) => [f.key, Boolean(row[f.key])])));
  const save = async () => {
    const body: Row = {};
    try {
      for (const f of spec.fields) {
        if (f.type === 'bool') { body[f.key] = bools[f.key]; continue; }
        const raw = draft[f.key] ?? '';
        if (f.type === 'json') body[f.key] = raw ? JSON.parse(raw) : null;
        else if (f.type === 'list') body[f.key] = raw ? raw.split(',').map((s) => s.trim()).filter(Boolean) : [];
        else body[f.key] = raw;
      }
    } catch (e) { onErr(`Invalid JSON: ${(e as Error).message}`); return; }
    try { const id = String(body.id ?? ''); if (row.id) await put(`/${spec.kind}/${id}`, body); else await post(`/${spec.kind}`, body); onSaved(); close(); }
    catch (e) { onErr(errText(e)); }
  };
  return (
    <div className="stack-1">
      {spec.fields.map((f) => f.type === 'bool'
        ? <Switch key={f.key} isSelected={bools[f.key]} onChange={(v) => setBools((b) => ({ ...b, [f.key]: v }))}>{f.key}</Switch>
        : <TextField key={f.key} label={f.key} value={draft[f.key] ?? ''} isDisabled={f.key === 'id' && !!row.id}
            onChange={(v) => setDraft((d) => ({ ...d, [f.key]: v }))}
            multiline={f.type === 'area' || f.type === 'json'} rows={f.type === 'json' ? 4 : 2}
            className={f.type === 'json' ? 'mono' : ''} />)}
      <div className="row end gap-2"><Button variant="ghost" onPress={close}>Cancel</Button><Button variant="primary" onPress={save}>Save</Button></div>
    </div>
  );
}
