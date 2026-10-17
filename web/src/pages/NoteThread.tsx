import { useState } from 'react';
import { Form } from 'react-aria-components';
import { errText, post } from '../lib/api';
import { principalName, setting, useHub } from '../lib/hub';
import { ago, useNow } from '../lib/time';
import type { Note, Task } from '../lib/types';
import { Avatar, Badge } from '../ui/Badge';
import { Button } from '../ui/Button';
import { TextField } from '../ui/Field';
import { Card, Empty, Notice } from '../ui/Notice';
import { Select } from '../ui/Select';

const kindTones: Record<string, 'blue' | 'violet' | 'red' | 'green' | 'neutral'> = {
  handoff: 'violet', blocker: 'red', decision: 'blue', summary: 'green',
};

// Notes and the handoff thread (append-only on the server).
export function NoteThread({ task, notes, onChanged }: { task: Task; notes: Note[]; onChanged: () => void }) {
  const { config, me } = useHub();
  const now = useNow(10000);
  const kinds = setting<string[]>(config, 'note_kinds', ['note']).filter((k) => k !== 'handoff' && k !== 'summary');
  const [body, setBody] = useState('');
  const [kind, setKind] = useState('note');
  const [err, setErr] = useState('');
  return (
    <Card title={`Notes & handoffs (${notes.length})`}>
      {notes.length === 0 && <Empty title="No notes yet">Agents record decisions, blockers and handoffs here.</Empty>}
      <ol className="thread">
        {notes.map((n) => (
          <li key={n.id} className={`note note-${n.kind}`}>
            <Avatar name={principalName(config, n.author)} human={config.principals.find((p) => p.id === n.author)?.is_human} />
            <div className="note-body">
              <div className="row wrap gap-2">
                <strong>{principalName(config, n.author)}</strong>
                <Badge tone={kindTones[n.kind] ?? 'neutral'}>{n.kind}</Badge>
                {n.to_principal && <span className="hint">→ {principalName(config, n.to_principal)}</span>}
                <span className="hint">{ago(n.created_at, now)}{n.fence ? ` · fence ${n.fence}` : ''}</span>
              </div>
              <p className="prewrap">{n.body}</p>
            </div>
          </li>
        ))}
      </ol>
      <Form className="note-form" onSubmit={async (e) => {
        e.preventDefault(); setErr('');
        try {
          await post(`/tasks/${task.id}/notes`, { body, kind, ...(task.holder === me.principal.id ? { fence: task.fence } : {}) });
          setBody(''); onChanged();
        } catch (x) { setErr(errText(x)); }
      }}>
        <TextField label="Add a note" multiline rows={3} value={body} onChange={setBody} placeholder="Decision, blocker, context for the next agent…" />
        <div className="row between gap-2">
          <Select label="Kind" hideLabel value={kind} onChange={setKind} options={kinds.map((k) => ({ id: k, label: k }))} className="w-auto" />
          <Button type="submit" variant="primary" isDisabled={!body.trim()}>Add note</Button>
        </div>
        {err && <Notice tone="error">{err}</Notice>}
      </Form>
    </Card>
  );
}
