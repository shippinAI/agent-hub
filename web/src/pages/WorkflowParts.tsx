import type { Lane, Workflow } from '../lib/types';
import { Button } from '../ui/Button';
import { Switch, TextField } from '../ui/Field';
import { Card } from '../ui/Notice';
import { Select } from '../ui/Select';

type P = { wf: Workflow; setWf: (w: Workflow) => void; edit: boolean };
const opts = (wf: Workflow, star = false) => [...(star ? [{ id: '*', label: 'any state' }] : []), ...wf.states.map((s) => ({ id: s.key, label: s.label }))];

export function StatesEditor({ wf, setWf, edit, kinds }: P & { kinds: string[] }) {
  const set = (i: number, k: 'key' | 'label' | 'kind', v: string) => setWf({ ...wf, states: wf.states.map((s, j) => (j === i ? { ...s, [k]: v } : s)) });
  return (
    <Card title="States" actions={edit && <Button size="sm" onPress={() => setWf({ ...wf, states: [...wf.states, { key: `state_${wf.states.length + 1}`, label: 'New state', kind: 'queue' }] })}>+ State</Button>}>
      <p className="hint">Kind tells the hub how to treat a state: queue (waiting), active (held under a lease), review, blocked, attention, done.</p>
      <ul className="edit-rows">
        {wf.states.map((s, i) => (
          <li key={i} className="edit-row">
            <TextField label="Key" value={s.key} onChange={(v) => set(i, 'key', v)} isDisabled={!edit} />
            <TextField label="Label" value={s.label} onChange={(v) => set(i, 'label', v)} isDisabled={!edit} />
            <Select label="Kind" value={s.kind} onChange={(v) => set(i, 'kind', v)} options={kinds.map((k) => ({ id: k, label: k }))} isDisabled={!edit} />
            {edit && <Button size="icon" variant="ghost" aria-label={`Remove ${s.label}`} onPress={() => setWf({ ...wf, states: wf.states.filter((_, j) => j !== i) })}>✕</Button>}
          </li>
        ))}
      </ul>
    </Card>
  );
}

export function LanesEditor({ wf, setWf, edit }: P) {
  const set = (i: number, patch: Partial<Lane>) => setWf({ ...wf, lanes: wf.lanes.map((l, j) => (j === i ? { ...l, ...patch } : l)) });
  const o = opts(wf);
  return (
    <Card title="Lanes (claimable work)" actions={edit && <Button size="sm" onPress={() => setWf({ ...wf, lanes: [...wf.lanes, { queue: wf.initial_state, active: wf.initial_state, complete_to: wf.initial_state, release_to: wf.initial_state }] })}>+ Lane</Button>}>
      <p className="hint">claim moves queue → active; complete → “complete to”; release, handoff and lease expiry → “release to”.</p>
      <ul className="edit-rows">
        {wf.lanes.map((l, i) => (
          <li key={i} className="edit-row lane-row">
            <Select label="Queue" value={l.queue} onChange={(v) => set(i, { queue: v })} options={o} isDisabled={!edit} />
            <Select label="Active" value={l.active} onChange={(v) => set(i, { active: v })} options={o} isDisabled={!edit} />
            <Select label="Complete to" value={l.complete_to} onChange={(v) => set(i, { complete_to: v })} options={o} isDisabled={!edit} />
            <Select label="Release to" value={l.release_to} onChange={(v) => set(i, { release_to: v })} options={o} isDisabled={!edit} />
            <Switch isSelected={!!l.distinct_from_previous} onChange={(v) => set(i, { distinct_from_previous: v })} isDisabled={!edit}>Different principal than previous step</Switch>
            {edit && <Button size="icon" variant="ghost" aria-label="Remove lane" onPress={() => setWf({ ...wf, lanes: wf.lanes.filter((_, j) => j !== i) })}>✕</Button>}
          </li>
        ))}
      </ul>
    </Card>
  );
}

export function TransitionsEditor({ wf, setWf, edit }: P) {
  const set = (i: number, k: 'from' | 'to', v: string) => setWf({ ...wf, transitions: wf.transitions.map((t, j) => (j === i ? { ...t, [k]: v } : t)) });
  return (
    <Card title="Manual transitions" actions={edit && <Button size="sm" onPress={() => setWf({ ...wf, transitions: [...wf.transitions, { from: '*', to: wf.initial_state }] })}>+ Transition</Button>}>
      <p className="hint">Moves allowed outside of claims (policy <code>task.transition:FROM-&gt;TO</code> decides who may make them).</p>
      <ul className="edit-rows">
        {wf.transitions.map((t, i) => (
          <li key={i} className="edit-row trans-row">
            <Select label="From" value={t.from} onChange={(v) => set(i, 'from', v)} options={opts(wf, true)} isDisabled={!edit} />
            <span className="arrow" aria-hidden>→</span>
            <Select label="To" value={t.to} onChange={(v) => set(i, 'to', v)} options={opts(wf)} isDisabled={!edit} />
            {edit && <Button size="icon" variant="ghost" aria-label="Remove transition" onPress={() => setWf({ ...wf, transitions: wf.transitions.filter((_, j) => j !== i) })}>✕</Button>}
          </li>
        ))}
      </ul>
    </Card>
  );
}
