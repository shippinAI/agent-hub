import { useState } from 'react';
import { Form } from 'react-aria-components';
import { del, errText, post, put } from '../lib/api';
import { useHub } from '../lib/hub';
import type { Policy } from '../lib/types';
import { Button } from '../ui/Button';
import { NumberField, Switch, TextField } from '../ui/Field';
import { Modal } from '../ui/Modal';
import { Notice } from '../ui/Notice';
import { Select } from '../ui/Select';

const blank: Policy = { id: '', name: '', subject: 'agent', action: 'task.claim:ready', selector: {}, effect: 'allow', priority: 10, enabled: true, version: 0 };
const list = (s: string) => s.split(',').map((x) => x.trim()).filter(Boolean);

export function PolicyEditor({ policy }: { policy?: Policy }) {
  const { reloadConfig, config } = useHub();
  const [f, setF] = useState<Policy>(policy ?? blank);
  const [err, setErr] = useState('');
  const subjects = ['*', 'human', 'agent', ...config.roles.map((r) => `role:${r.id}`), ...config.principals.map((p) => `principal:${p.id}`)];
  return (
    <Modal title={policy ? 'Edit policy' : 'New policy'}
      trigger={<Button size="sm" variant={policy ? 'ghost' : 'primary'}>{policy ? 'Edit' : '+ Policy'}</Button>}>
      {(close) => (
        <Form onSubmit={async (e) => {
          e.preventDefault(); setErr('');
          try { if (policy) await put(`/policies/${f.id}`, f); else await post('/policies', f); reloadConfig(); close(); }
          catch (x) { setErr(errText(x)); }
        }}>
          <TextField label="Name" value={f.name} onChange={(v) => setF({ ...f, name: v })} isRequired />
          <div className="grid-2">
            <Select label="Subject" value={f.subject} onChange={(v) => setF({ ...f, subject: v })} options={subjects.map((s) => ({ id: s, label: s }))} />
            <TextField label="Action (glob)" value={f.action} onChange={(v) => setF({ ...f, action: v })} isRequired />
          </div>
          <div className="grid-2">
            <TextField label="Projects" placeholder="any" value={(f.selector.projects ?? []).join(', ')}
              onChange={(v) => setF({ ...f, selector: { ...f.selector, projects: list(v) } })} />
            <TextField label="Labels (all required)" placeholder="any" value={(f.selector.labels ?? []).join(', ')}
              onChange={(v) => setF({ ...f, selector: { ...f.selector, labels: list(v) } })} />
          </div>
          <div className="grid-2">
            <Select label="Effect" value={f.effect} onChange={(v) => setF({ ...f, effect: v as Policy['effect'] })}
              options={[{ id: 'allow', label: 'allow' }, { id: 'deny', label: 'deny' }]} />
            <NumberField label="Priority" value={f.priority} onChange={(v) => setF({ ...f, priority: v || 0 })} />
          </div>
          <Switch isSelected={f.enabled} onChange={(v) => setF({ ...f, enabled: v })}>Enabled</Switch>
          {err && <Notice tone="error">{err}</Notice>}
          <div className="dialog-actions">
            {policy && <Button variant="destructive" onPress={async () => {
              try { await del(`/policies/${f.id}`); reloadConfig(); close(); } catch (x) { setErr(errText(x)); }
            }}>Delete</Button>}
            <span className="grow" />
            <Button variant="ghost" onPress={close}>Cancel</Button>
            <Button type="submit" variant="primary">Save</Button>
          </div>
        </Form>
      )}
    </Modal>
  );
}
