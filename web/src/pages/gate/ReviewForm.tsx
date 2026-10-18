import { useState } from 'react';
import { Radio, RadioGroup, Label } from 'react-aria-components';
import { errText, post } from '../../lib/api';
import type { Ticket } from '../../lib/gtypes';
import { Button } from '../../ui/Button';
import { TextField } from '../../ui/Field';
import { Card, Notice } from '../../ui/Notice';

// A human reviewer's verdict, bound to the hash shown (a changed ticket refuses it as stale).
export function ReviewForm({ ticket, onDone }: { ticket: Ticket; onDone: () => void }) {
  const [verdict, setVerdict] = useState('PASS');
  const [summary, setSummary] = useState('');
  const [findings, setFindings] = useState('');
  const [err, setErr] = useState('');
  const send = async () => {
    setErr('');
    try {
      await post(`/change-tickets/${encodeURIComponent(ticket.id)}/reviews`, { verdict, summary, ticket_sha256: ticket.sha256,
        findings: findings.split('\n').map((s) => s.trim()).filter(Boolean) });
      setSummary(''); setFindings(''); onDone();
    } catch (e) { setErr(errText(e)); }
  };
  return (
    <Card title="Your review">
      <RadioGroup value={verdict} onChange={setVerdict} className="field radio-row" orientation="horizontal">
        <Label className="label">Verdict</Label>
        <div className="row gap-2">{['PASS', 'FINDINGS', 'BLOCK'].map((v) => <Radio key={v} value={v} className="radio">{v}</Radio>)}</div>
      </RadioGroup>
      <TextField label="Summary" value={summary} onChange={setSummary} />
      <TextField label="Findings (one per line)" multiline rows={3} value={findings} onChange={setFindings} />
      {err && <Notice tone="error">{err}</Notice>}
      <div><Button variant="primary" isDisabled={summary.trim().length < 3} onPress={send}>Submit {verdict}</Button></div>
    </Card>
  );
}
