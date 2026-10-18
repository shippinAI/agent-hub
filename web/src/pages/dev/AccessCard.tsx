import { useState } from 'react';
import { post, errText } from '../../lib/api';
import type { Request } from '../../lib/dtypes';
import { Badge } from '../../ui/Badge';
import { Button } from '../../ui/Button';
import { Notice } from '../../ui/Notice';
import { RiskList, SelectorText } from './risk';
import { NarrowEditor } from './NarrowEditor';

// Access card: who/what/why/risks/technical, actions Approve / Approve narrower / Deny / Not now (07).
export function AccessCard({ req, onDone }: { req: Request; onDone: () => void }) {
  const [err, setErr] = useState<string | null>(null);
  const [narrow, setNarrow] = useState(false);
  const act = async (verb: string, body?: unknown) => {
    try { await post(`/access-requests/${req.id}/${verb}`, body ?? {}); onDone(); }
    catch (e) { setErr(errText(e)); }
  };
  return (
    <section className="access-card">
      {err && <Notice tone="error">{err}</Notice>}
      <header className="stack-1">
        <div className="row gap-2"><h2>{req.requester_name}</h2><Badge tone="blue">{req.capability}</Badge>
          <Badge tone={req.sandbox_level === 'unsandboxed' ? 'red' : 'neutral'}>{req.sandbox_level}</Badge></div>
        <p className="muted">wants access on <strong>{req.machine_label}</strong> for <strong>{req.duration}</strong></p>
      </header>
      <blockquote className="reason">“{req.reason}”</blockquote>
      <div className="stack-1"><h3>What</h3><SelectorText selector={req.selector} /></div>
      <div className="stack-1"><h3>Risks</h3><RiskList risks={req.risk_snapshot} /></div>
      {narrow
        ? <NarrowEditor req={req} onCancel={() => setNarrow(false)} onApprove={(b) => act('approve', b)} />
        : (
        <div className="row gap-2 actions">
          <Button variant="primary" onPress={() => act('approve')}>Approve</Button>
          <Button onPress={() => setNarrow(true)}>Approve narrower</Button>
          <Button variant="destructive" onPress={() => act('deny', { reason: 'denied' })}>Deny</Button>
          <Button variant="ghost" onPress={onDone}>Not now</Button>
        </div>
      )}
    </section>
  );
}
