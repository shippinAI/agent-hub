import { useState } from 'react';
import { Form } from 'react-aria-components';
import { errText, post } from '../lib/api';
import { setting, useHub } from '../lib/hub';
import { go } from '../lib/route';
import type { Task } from '../lib/types';
import { Button } from '../ui/Button';
import { TextField } from '../ui/Field';
import { Modal } from '../ui/Modal';
import { Notice } from '../ui/Notice';
import { Select } from '../ui/Select';

export function NewTask({ project }: { project: string }) {
  const { config } = useHub();
  const prios = setting<string[]>(config, 'priority_labels', ['P0', 'P1', 'P2', 'P3']);
  const [f, setF] = useState({ title: '', description: '', labels: '', priority: String(setting(config, 'default_priority', 2)), deps: '' });
  const [err, setErr] = useState('');
  return (
    <Modal title="New task" trigger={<Button variant="primary">+ New task</Button>}>
      {(close) => (
        <Form onSubmit={async (e) => {
          e.preventDefault(); setErr('');
          try {
            const t = await post<Task>('/tasks', { project, title: f.title, description: f.description, priority: Number(f.priority),
              labels: f.labels.split(',').map((s) => s.trim()).filter(Boolean),
              depends_on: f.deps.split(/[ ,#]+/).filter(Boolean).map(Number) });
            close(); go(`task/${t.id}`);
          } catch (x) { setErr(errText(x)); }
        }}>
          <TextField label="Title" value={f.title} onChange={(v) => setF({ ...f, title: v })} isRequired autoFocus />
          <TextField label="Description" multiline rows={4} value={f.description} onChange={(v) => setF({ ...f, description: v })}
            description="Acceptance criteria help agents finish without asking." />
          <div className="grid-2">
            <Select label="Priority" value={f.priority} onChange={(v) => setF({ ...f, priority: v })}
              options={prios.map((l, i) => ({ id: String(i), label: l }))} />
            <TextField label="Labels" placeholder="api, docs" value={f.labels} onChange={(v) => setF({ ...f, labels: v })} />
          </div>
          <TextField label="Depends on" placeholder="#12, #14" value={f.deps} onChange={(v) => setF({ ...f, deps: v })} />
          {err && <Notice tone="error">{err}</Notice>}
          <div className="dialog-actions">
            <Button variant="ghost" onPress={close}>Cancel</Button>
            <Button type="submit" variant="primary">Create</Button>
          </div>
        </Form>
      )}
    </Modal>
  );
}
