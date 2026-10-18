import type { GuideStep } from '../../lib/dtypes';
import { Badge } from '../../ui/Badge';
import { Button } from '../../ui/Button';
import { TextField } from '../../ui/Field';
import { Select } from '../../ui/Select';
import { Card } from '../../ui/Notice';

const KINDS = ['explain', 'check', 'request_access', 'agent_action', 'operator_action', 'confirm', 'branch', 'verify', 'link_guide'];
const ACTORS = ['ai', 'agent', 'operator', 'none'];

// Steps tab of the guide editor: ordered list with up/down/delete and a per-step form. Each step's
// visible fields follow its kind (command for agent_action, check for verify, question for branch, …).
export function GuideSteps({ steps, onChange }: { steps: GuideStep[]; onChange: (s: GuideStep[]) => void }) {
  const set = (i: number, patch: Partial<GuideStep>) => onChange(steps.map((s, j) => (j === i ? { ...s, ...patch } : s)));
  const move = (i: number, d: number) => {
    const j = i + d; if (j < 0 || j >= steps.length) return;
    const next = steps.slice(); [next[i], next[j]] = [next[j], next[i]]; onChange(next);
  };
  const add = () => onChange([...steps, { id: `step-${steps.length + 1}`, kind: 'explain', actor: 'ai', title: 'New step' }]);
  return (
    <div className="stack-2">
      {steps.map((s, i) => (
        <Card key={i} className="step-card">
          <div className="row between">
            <div className="row gap-2"><Badge tone="neutral">{i + 1}</Badge><Badge tone="blue">{s.kind}</Badge></div>
            <div className="row gap-1">
              <Button size="sm" variant="ghost" isDisabled={i === 0} onPress={() => move(i, -1)}>↑</Button>
              <Button size="sm" variant="ghost" isDisabled={i === steps.length - 1} onPress={() => move(i, 1)}>↓</Button>
              <Button size="sm" variant="ghost" onPress={() => onChange(steps.filter((_, j) => j !== i))}>Delete</Button>
            </div>
          </div>
          <div className="row gap-2 wrap">
            <TextField label="ID" value={s.id} onChange={(v) => set(i, { id: v })} className="w-sm" />
            <Select label="Kind" value={s.kind} options={KINDS.map((k) => ({ id: k, label: k }))} onChange={(v) => set(i, { kind: v })} className="w-sm" />
            <Select label="Actor" value={s.actor ?? 'none'} options={ACTORS.map((k) => ({ id: k, label: k }))} onChange={(v) => set(i, { actor: v })} className="w-sm" />
          </div>
          <TextField label="Title" value={s.title} onChange={(v) => set(i, { title: v })} />
          <TextField label="Text" value={s.text ?? ''} onChange={(v) => set(i, { text: v })} multiline rows={2} />
          {(s.kind === 'agent_action' || s.kind === 'operator_action') &&
            <TextField label="Command" value={s.command ?? ''} onChange={(v) => set(i, { command: v })} className="mono" />}
          {(s.kind === 'check' || s.kind === 'verify') &&
            <TextField label="Check" value={s.check ?? ''} onChange={(v) => set(i, { check: v })} className="mono" />}
          {s.kind === 'branch' &&
            <TextField label="Question" value={s.question ?? ''} onChange={(v) => set(i, { question: v })} />}
        </Card>
      ))}
      <Button onPress={add}>Add step</Button>
    </div>
  );
}
