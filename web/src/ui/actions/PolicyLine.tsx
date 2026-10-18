import { Badge } from '../Badge';
import type { ActionPolicy } from '../../lib/actionTypes';

// Who allowed (or refused) the call, under which grant and sandbox level.
export function PolicyLine({ p }: { p?: ActionPolicy }) {
  if (!p) return null;
  return (
    <div className="policy-line">
      <Badge tone={p.decision === 'allow' ? 'green' : 'red'}>{p.decision === 'allow' ? 'allowed' : 'denied'}</Badge>
      {p.approver && <span>approved by <strong>{p.approver}</strong></span>}
      {p.grant_id && <span className="muted mono" title={p.grant_id}>grant {p.grant_id.slice(0, 8)}</span>}
      {p.sandbox_level && <Badge tone="outline">{p.sandbox_level}</Badge>}
      {p.code && <code>{p.code}</code>}
      {p.reason && p.decision === 'deny' && <span className="muted">{p.reason}</span>}
    </div>
  );
}
