import { useState } from 'react';
import { post, errText } from '../../lib/api';
import { Button } from '../../ui/Button';
import { Modal } from '../../ui/Modal';
import { TextField } from '../../ui/Field';
import { Select } from '../../ui/Select';
import { Notice } from '../../ui/Notice';

type Enroll = { enrollment: { code: string; code_prefix: string; expires_at: string }; machine: { id: string } };

// Add machine: create the row, then show the enrollment code once (masked) with a copy button (07).
export function AddMachine({ onDone }: { onDone: () => void }) {
  return (
    <Modal trigger={<Button variant="primary">Add machine</Button>} title="Add a machine">
      {(close) => <AddForm onDone={() => { onDone(); }} close={close} />}
    </Modal>
  );
}

function AddForm({ onDone, close }: { onDone: () => void; close: () => void }) {
  const [id, setId] = useState('');
  const [label, setLabel] = useState('');
  const [os, setOs] = useState('linux');
  const [code, setCode] = useState<Enroll['enrollment'] | null>(null);
  const [reveal, setReveal] = useState(false);
  const [err, setErr] = useState<string | null>(null);
  const create = async () => {
    try {
      const r = await post<Enroll>('/machines', { id, label, os });
      setCode(r.enrollment); onDone();
    } catch (e) { setErr(errText(e)); }
  };
  if (code) {
    const masked = `${code.code_prefix}-••••••••••••`;
    return (
      <div className="stack-3">
        <Notice tone="success">Machine created. Enrol the bridge with this one-time code (shown once).</Notice>
        <div className="row gap-2">
          <code className="code-box">{reveal ? code.code : masked}</code>
          <Button size="sm" onPress={() => setReveal((v) => !v)}>{reveal ? 'Hide' : 'Reveal'}</Button>
          <Button size="sm" onPress={() => navigator.clipboard?.writeText(code.code)}>Copy</Button>
        </div>
        <p className="hint">Then run <code>devagotchi-bridge enroll --hub &lt;url&gt;</code> and paste the code.</p>
        <div className="row gap-2 end"><Button variant="primary" onPress={close}>Done</Button></div>
      </div>
    );
  }
  return (
    <div className="stack-3">
      {err && <Notice tone="error">{err}</Notice>}
      <TextField label="Machine id" value={id} onChange={setId} placeholder="laptop" isRequired />
      <TextField label="Label" value={label} onChange={setLabel} placeholder="Jesse Laptop" />
      <Select label="Operating system" value={os} onChange={setOs} options={[
        { id: 'linux', label: 'Linux' }, { id: 'macos', label: 'macOS' }, { id: 'windows', label: 'Windows' }]} />
      <div className="row gap-2 end"><Button onPress={close}>Cancel</Button><Button variant="primary" onPress={create} isDisabled={!id}>Create</Button></div>
    </div>
  );
}
