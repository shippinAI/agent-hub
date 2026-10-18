import { useMemo, useState } from 'react';
import type { DocRow, PolicyDoc, Rule } from '../../lib/gtypes';
import { Badge } from '../../ui/Badge';
import { NumberField, Switch } from '../../ui/Field';
import { SaveBar } from './SaveBar';

const effectTone: Record<string, 'red' | 'violet' | 'neutral' | 'green' | 'blue'> = { deny: 'red', require: 'violet', collect: 'blue', allow: 'green', audit: 'neutral' };

function when(r: Rule): string {
  const w = r.when;
  const parts = [w.classes?.length && `class ${w.classes.join('/')}`, w.regions?.length && `region ${w.regions.join('/')}`,
    w.command_re && `matches /${w.command_re.slice(0, 40)}${w.command_re.length > 40 ? '…' : ''}/`, w.events?.length && `on ${w.events.join('/')}`];
  return parts.filter(Boolean).join(' · ') || 'every call';
}

// Rules as cards: toggle and re-prioritise in place; the JSON tab edits everything else.
export function RulesEditor({ row, canEdit, onSaved }: { row: DocRow; canEdit: boolean; onSaved: () => void }) {
  const [doc, setDoc] = useState<PolicyDoc>(() => structuredClone(row.doc!));
  const base = useMemo(() => JSON.stringify(row.doc), [row]);
  const dirty = JSON.stringify(doc) !== base;
  const set = (i: number, patch: Partial<Rule>) => setDoc((d) => ({ ...d, rules: d.rules.map((r, j) => (j === i ? { ...r, ...patch } : r)) }));
  return (
    <div className="stack-3">
      <ul className="rule-list" aria-label="Policy rules">
        {doc.rules.map((r, i) => (
          <li key={r.id} className={`card rule ${r.enabled ? '' : 'is-off'}`}>
            <div className="row between wrap gap-2">
              <div className="stack-0">
                <strong>{r.name}</strong>
                <span className="hint mono">{r.id} · hook {r.hook}</span>
              </div>
              <div className="row gap-2">
                <Badge tone={effectTone[r.effect] ?? 'neutral'}>{r.effect}</Badge>
                <Switch isSelected={r.enabled} isDisabled={!canEdit} onChange={(v) => set(i, { enabled: v })}>{r.enabled ? 'On' : 'Off'}</Switch>
              </div>
            </div>
            <p className="muted">{r.message}</p>
            <div className="row wrap gap-2">
              <span className="hint">When: {when(r)}</span>
              {(r.require ?? []).map((c, k) => <Badge key={k} tone="outline">{c.check}{c.field ? `:${c.field}` : ''}{c.role ? ` (${c.role})` : ''}</Badge>)}
            </div>
            <div className="rule-prio"><NumberField label="Priority" value={r.priority} minValue={0} maxValue={1000} onChange={(v) => set(i, { priority: v })} /></div>
          </li>
        ))}
      </ul>
      {canEdit && <SaveBar row={row} doc={doc} dirty={dirty} onSaved={onSaved} />}
    </div>
  );
}
