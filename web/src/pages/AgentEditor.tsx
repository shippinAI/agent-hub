import { useState } from 'react';
import { Form } from 'react-aria-components';
import { errText, post, put } from '../lib/api';
import { setting, useHub } from '../lib/hub';
import type { Principal } from '../lib/types';
import { Button } from '../ui/Button';
import { CheckboxGroup, NumberField, Switch, TextField } from '../ui/Field';
import { Modal } from '../ui/Modal';
import { Notice } from '../ui/Notice';
import { Select } from '../ui/Select';

const blank: Principal = { id: '', display_name: '', kind: 'cli', is_human: false, login: null, roles: ['worker'], enabled: true,
  wip_limit: null, last_seen_at: null, version: 0, created_at: '' };

// Create or edit a principal. Kinds and roles are data (settings.principal_kinds, roles table).
export function AgentEditor({ principal }: { principal?: Principal }) {
  const { config, reloadConfig, me } = useHub();
  const [f, setF] = useState<Principal>(principal ?? blank);
  const [err, setErr] = useState('');
  const kinds = setting<string[]>(config, 'principal_kinds', ['cli']);
  const self = principal?.id === me.principal.id;
  return (
    <Modal title={principal ? `Edit ${principal.display_name}` : 'New principal'}
      trigger={<Button variant={principal ? 'outline' : 'primary'} size={principal ? 'sm' : 'md'}>{principal ? 'Edit' : '+ New agent'}</Button>}>
      {(close) => (
        <Form onSubmit={async (e) => {
          e.preventDefault(); setErr('');
          try {
            if (principal) await put(`/principals/${f.id}`, f); else await post('/principals', f);
            reloadConfig(); close();
          } catch (x) { setErr(errText(x)); }
        }}>
          <div className="grid-2">
            <TextField label="ID" value={f.id} onChange={(v) => setF({ ...f, id: v })} isRequired isDisabled={!!principal}
              description="lowercase, used in tokens and audit" />
            <TextField label="Display name" value={f.display_name} onChange={(v) => setF({ ...f, display_name: v })} isRequired />
          </div>
          <div className="grid-2">
            <Select label="Kind" value={f.kind} onChange={(v) => setF({ ...f, kind: v })} options={kinds.map((k) => ({ id: k, label: k }))} />
            <NumberField label="WIP limit" value={f.wip_limit ?? NaN} minValue={0}
              onChange={(v) => setF({ ...f, wip_limit: Number.isNaN(v) ? null : v })} description="empty = hub default" />
          </div>
          <CheckboxGroup label="Roles" value={f.roles} onChange={(roles) => setF({ ...f, roles })}
            options={config.roles.map((r) => ({ id: r.id, label: r.id }))} />
          {self && <p className="hint">You can't change your own roles, human flag or enabled state.</p>}
          <TextField label="External login (humans)" value={f.login ?? ''} onChange={(v) => setF({ ...f, login: v || null })}
            description="tailnet login or proxy identity that maps to this principal" />
          <div className="row wrap gap-4">
            <Switch isSelected={f.is_human} onChange={(v) => setF({ ...f, is_human: v })}>Human</Switch>
            <Switch isSelected={f.enabled} onChange={(v) => setF({ ...f, enabled: v })}>Enabled</Switch>
          </div>
          {err && <Notice tone="error">{err}</Notice>}
          <div className="dialog-actions">
            <Button variant="ghost" onPress={close}>Cancel</Button>
            <Button type="submit" variant="primary">Save</Button>
          </div>
        </Form>
      )}
    </Modal>
  );
}
