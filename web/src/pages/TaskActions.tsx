import { useState } from 'react';
import { errText, patch, post } from '../lib/api';
import { useHub } from '../lib/hub';
import type { Task, Workflow } from '../lib/types';
import { Button } from '../ui/Button';
import { TextField } from '../ui/Field';
import { Modal } from '../ui/Modal';
import { Notice } from '../ui/Notice';
import { Select } from '../ui/Select';

// Human actions on a task. Agents normally use MCP/CLI; the server enforces every rule regardless.
export function TaskActions({ task: t, wf, onChanged }: { task: Task; wf?: Workflow; onChanged: () => void }) {
  const { me } = useHub();
  const [err, setErr] = useState('');
  const mine = t.holder === me.principal.id;
  const run = async (fn: () => Promise<unknown>) => { setErr(''); try { await fn(); onChanged(); } catch (e) { setErr(errText(e)); } };
  const moves = (wf?.transitions ?? []).filter((x) => (x.from === t.state || x.from === '*') && x.to !== t.state)
    .map((x) => wf!.states.find((s) => s.key === x.to)).filter((s) => s && s.kind !== 'active');
  return (
    <div className="stack-2 actions">
      {err && <Notice tone="error">{err}</Notice>}
      <div className="row wrap gap-2">
        {t.ready && <Button variant="primary" onPress={() => run(() => post(`/tasks/${t.id}/claim`, {}))}>Claim</Button>}
        {mine && <Button onPress={() => run(() => post(`/tasks/${t.id}/heartbeat`, { fence: t.fence }))}>Renew lease</Button>}
        {mine && <Button variant="primary" onPress={() => run(() => post(`/tasks/${t.id}/complete`, { fence: t.fence, summary: '' }))}>Complete step</Button>}
        {mine && <Button variant="outline" onPress={() => run(() => post(`/tasks/${t.id}/release`, { fence: t.fence, reason: 'released from UI' }))}>Release</Button>}
        {t.holder && !mine && me.can['task.force_release'] && <ForceRelease task={t} onDone={onChanged} />}
      </div>
      {!t.holder && moves.length > 0 && (
        <Select label="Move to" value={null} onChange={(to) => run(() => patch(`/tasks/${t.id}`, { version: t.version, state: to }))}
          options={moves.map((s) => ({ id: s!.key, label: s!.label, hint: s!.kind }))} />
      )}
    </div>
  );
}

// Force release: humans only (code rule) + policy; mandatory reason; shown as an alertdialog.
function ForceRelease({ task, onDone }: { task: Task; onDone: () => void }) {
  const [reason, setReason] = useState('');
  const [err, setErr] = useState('');
  return (
    <Modal alert title={`Force-release #${task.id}?`} trigger={<Button variant="destructive">Force release</Button>}>
      {(close) => (
        <>
          <p className="muted">{task.holder} loses the lease immediately. Its fence ({task.fence}) becomes stale, so any write it still
            sends is rejected. The reason is recorded in the audit log.</p>
          <TextField label="Reason" multiline value={reason} onChange={setReason} isRequired autoFocus
            validate={(v) => (v.trim().length < 3 ? 'Give a reason (3+ characters).' : null)} />
          {err && <Notice tone="error">{err}</Notice>}
          <div className="dialog-actions">
            <Button variant="ghost" onPress={close}>Cancel</Button>
            <Button variant="destructive" isDisabled={reason.trim().length < 3} onPress={async () => {
              try { await post(`/tasks/${task.id}/force-release`, { reason }); close(); onDone(); } catch (e) { setErr(errText(e)); }
            }}>Force release</Button>
          </div>
        </>
      )}
    </Modal>
  );
}
