import { useEffect, useState } from 'react';
import { errText, put } from '../lib/api';
import { useHub } from '../lib/hub';
import type { Workflow } from '../lib/types';
import { Badge, kindTone } from '../ui/Badge';
import { Button } from '../ui/Button';
import { TextField } from '../ui/Field';
import { Card, Notice } from '../ui/Notice';
import { Select } from '../ui/Select';
import { LanesEditor, StatesEditor, TransitionsEditor } from './WorkflowParts';

// Workflow = states (with a kind), lanes (claim → active → complete/release) and manual transitions.
export function Workflows() {
  const { config, me, reloadConfig } = useHub();
  const edit = me.can['admin.policies.put'];
  const [id, setId] = useState(config.workflows[0]?.id ?? '');
  const src = config.workflows.find((w) => w.id === id);
  const [wf, setWf] = useState<Workflow | undefined>(src);
  const [msg, setMsg] = useState<{ tone: 'error' | 'success'; text: string } | null>(null);
  useEffect(() => setWf(src), [src]);
  if (!wf) return <Notice>No workflows.</Notice>;
  const used = config.projects.filter((p) => p.workflow_id === wf.id).map((p) => p.name);
  const stateOpts = wf.states.map((s) => ({ id: s.key, label: s.label, hint: s.kind }));
  return (
    <div className="stack-4">
      <Card title="Workflow" actions={edit && <Button variant="primary" onPress={async () => {
        setMsg(null);
        try { await put(`/workflows/${wf.id}`, wf); reloadConfig(); setMsg({ tone: 'success', text: 'Saved. New rules apply to the next request.' }); }
        catch (e) { setMsg({ tone: 'error', text: errText(e) }); }
      }}>Save workflow</Button>}>
        <div className="grid-3">
          <Select label="Workflow" value={id} onChange={setId} options={config.workflows.map((w) => ({ id: w.id, label: w.name }))} />
          <TextField label="Name" value={wf.name} onChange={(v) => setWf({ ...wf, name: v })} isDisabled={!edit} />
          <div className="field"><span className="label">Used by</span><span className="hint">{used.join(', ') || 'no projects'}</span></div>
        </div>
        <div className="grid-2">
          <Select label="Initial state" value={wf.initial_state} onChange={(v) => setWf({ ...wf, initial_state: v })} options={stateOpts} isDisabled={!edit} />
          <Select label="Attention state (after repeated lease expiry)" value={wf.attention_state}
            onChange={(v) => setWf({ ...wf, attention_state: v })} options={stateOpts} isDisabled={!edit} />
        </div>
        <div className="flow" aria-label="Workflow preview">
          {wf.states.map((s) => <Badge key={s.key} tone={kindTone[s.kind]}>{s.label}</Badge>)}
        </div>
        {msg && <Notice tone={msg.tone}>{msg.text}</Notice>}
      </Card>
      <StatesEditor wf={wf} setWf={setWf} edit={edit} kinds={config.state_kinds} />
      <LanesEditor wf={wf} setWf={setWf} edit={edit} />
      <TransitionsEditor wf={wf} setWf={setWf} edit={edit} />
    </div>
  );
}
