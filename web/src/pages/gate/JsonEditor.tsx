import { useMemo, useState } from 'react';
import type { DocRow, PolicyDoc } from '../../lib/gtypes';
import { TextField } from '../../ui/Field';
import { Notice } from '../../ui/Notice';
import { SaveBar } from './SaveBar';

// Full document editor (classes, regions, rules). The hub validates on save (bad regex, unknown check -> 400).
export function JsonEditor({ row, canEdit, onSaved }: { row: DocRow; canEdit: boolean; onSaved: () => void }) {
  const base = useMemo(() => JSON.stringify(row.doc, null, 2), [row]);
  const [text, setText] = useState(base);
  const parsed = useMemo(() => { try { return { doc: JSON.parse(text) as PolicyDoc, err: '' }; } catch (e) { return { doc: null, err: String(e) }; } }, [text]);
  return (
    <div className="stack-3">
      <TextField label={`Policy document v${row.version}`} multiline rows={22} value={text} onChange={setText} isDisabled={!canEdit} className="mono json-edit" />
      {parsed.err && <Notice tone="error">{parsed.err}</Notice>}
      {canEdit && <SaveBar row={row} doc={parsed.doc} dirty={text !== base && !parsed.err} onSaved={onSaved} />}
    </div>
  );
}
