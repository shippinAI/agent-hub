import { useState } from 'react';
import { errText, post } from '../../lib/api';
import type { Answer } from '../../lib/gtypes';
import { Button } from '../../ui/Button';
import { TextField } from '../../ui/Field';
import { Notice } from '../../ui/Notice';
import { DecisionCard } from './DecisionCard';

// Dry run against the current policy (not logged, ticket not bound to you): what would happen to this command?
export function TryIt() {
  const [cmd, setCmd] = useState('rm -rf /etc');
  const [machine, setMachine] = useState('');
  const [ticket, setTicket] = useState('');
  const [ans, setAns] = useState<Answer | null>(null);
  const [err, setErr] = useState('');
  const run = async () => {
    setErr('');
    try { setAns(await post<Answer>('/policy/decide', { tool: 'hub:exec', command: cmd, machine, ticket_id: ticket, dry_run: true })); }
    catch (e) { setErr(errText(e)); setAns(null); }
  };
  return (
    <div className="stack-3 try-it">
      <div className="card stack-2">
        <TextField label="Command" value={cmd} onChange={setCmd} className="mono" />
        <div className="grid-2">
          <TextField label="Machine (optional)" value={machine} onChange={setMachine} placeholder="clyffy-box" />
          <TextField label="Change ticket (optional)" value={ticket} onChange={setTicket} placeholder="CHG-0001" />
        </div>
        <div><Button variant="primary" onPress={run}>Check</Button></div>
      </div>
      {err && <Notice tone="error">{err}</Notice>}
      {ans && <DecisionCard a={ans} title={cmd} />}
    </div>
  );
}
