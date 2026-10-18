import { useState } from 'react';
import { get, post, errText } from '../../lib/api';
import { useData } from '../../lib/useData';
import type { GuideRunView } from '../../lib/dtypes';
import { Badge } from '../../ui/Badge';
import { Button } from '../../ui/Button';
import { Notice } from '../../ui/Notice';

const actorLabel: Record<string, string> = { ai: 'the assistant', agent: 'the agent', operator: 'you', none: '' };

// Guide runner side panel (07): title + version, numbered steps with who does each, the current step
// expanded with live check results, Pause / Skip / Report. Used live and in the editor Preview (dry run).
export function GuideRunner({ runId, onClose }: { runId: string; onClose?: () => void }) {
  const run = useData(() => get<GuideRunView>(`/guide-runs/${runId}`), [runId]);
  const [err, setErr] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  if (run.error) return <Notice tone="error">{errText(run.error)}</Notice>;
  if (!run.data) return <aside className="runner" role="status">Loading…</aside>;
  const { run: r, outline, step } = run.data;
  const step2 = async (body: Record<string, unknown>) => {
    setBusy(true); setErr(null);
    try { await post(`/guide-runs/${runId}/step`, { step_id: r.current_step, ...body }); run.reload(); }
    catch (e) { setErr(errText(e)); } finally { setBusy(false); }
  };
  return (
    <aside className="runner" aria-label="Guide runner">
      <header className="runner-head">
        <div><strong>{r.guide_id}</strong> <Badge tone="neutral">v{r.semver}</Badge>
          {r.dry_run && <Badge tone="violet">dry run</Badge>}</div>
        {onClose && <Button size="sm" variant="ghost" onPress={onClose}>Close</Button>}
      </header>
      <ol className="runner-steps">
        {outline.map((o) => (
          <li key={o.id} className={`runner-step ${o.id === r.current_step ? 'is-current' : ''} ${step && outline.findIndex((x) => x.id === r.current_step) > outline.findIndex((x) => x.id === o.id) ? 'is-done' : ''}`}>
            <span className="runner-step-title">{o.title}</span><span className="hint">{o.kind}</span>
          </li>
        ))}
      </ol>
      {err && <Notice tone="error">{err}</Notice>}
      {r.status === 'done' ? <Notice tone="success">Run complete.</Notice>
        : step ? (
        <div className="runner-current">
          <div className="row gap-2"><Badge tone="blue">{step.kind}</Badge>{step.actor && <span className="hint">{actorLabel[step.actor]}</span>}</div>
          <h3>{step.title}</h3>
          {step.text && <p>{step.text}</p>}
          {step.command && <pre className="code-box">{step.command}</pre>}
          {step.check && <p className="hint">Check: {step.check}</p>}
          <StepActions step={step} busy={busy} onStep={step2} />
          <div className="row gap-1">
            <Button size="sm" variant="ghost" isDisabled={busy} onPress={() => step2({ outcome: 'pause' })}>Pause</Button>
            <Button size="sm" variant="ghost" isDisabled={busy} onPress={() => step2({ outcome: 'skip' })}>Skip</Button>
          </div>
        </div>
      ) : <Notice tone="info">Waiting for the next step…</Notice>}
    </aside>
  );
}

function StepActions({ step, busy, onStep }: { step: GuideRunView['step'] & object; busy: boolean; onStep: (b: Record<string, unknown>) => void }) {
  if (!step) return null;
  if (step.options && step.options.length) return (
    <div className="row gap-1">{step.options.map((o, i) => (
      <Button key={i} size="sm" isDisabled={busy} onPress={() => onStep({ value: o.goto ?? o.label })}>{o.label}</Button>))}</div>
  );
  const verb = step.kind === 'operator_action' ? 'Done' : step.kind === 'confirm' ? 'Confirm' : 'Continue';
  return <Button variant="primary" size="sm" isDisabled={busy} onPress={() => onStep({ outcome: 'ok' })}>{verb}</Button>;
}
