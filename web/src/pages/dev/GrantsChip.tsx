import { get, post } from '../../lib/api';
import { useData } from '../../lib/useData';
import type { Grant, Summary } from '../../lib/dtypes';
import { Badge } from '../../ui/Badge';
import { Button } from '../../ui/Button';
import { MenuTrigger, Popover, Menu, MenuItem, Button as RACButton } from 'react-aria-components';

// Header grants chip: "N live · M high risk", a menu listing each grant with Revoke, plus Revoke all (07).
// Also surfaces the "Unsandboxed allowed" marker whenever the policy is on.
export function GrantsChip() {
  const summary = useData(() => get<Summary>('/access-summary'), []);
  const grants = useData(() => get<Grant[]>('/grants?live=true'), []);
  const s = summary.data;
  const revokeAll = async () => { for (const g of grants.data ?? []) await post(`/grants/${g.id}/revoke`, { reason: 'revoke all' }); grants.reload(); summary.reload(); };
  const revoke = async (id: string) => { await post(`/grants/${id}/revoke`, { reason: 'revoked' }); grants.reload(); summary.reload(); };
  if (!s) return null;
  return (
    <div className="row gap-2">
      {s.unsandboxed_allowed && <Badge tone="red" title="The unsandboxed policy is enabled">unsandboxed allowed</Badge>}
      {s.gui_active > 0 && <Badge tone="violet">GUI active</Badge>}
      {s.live_grants > 0 && (
        <MenuTrigger>
          <RACButton className="chip" data-testid="grants-chip">
            {s.live_grants} live{s.high_risk > 0 ? ` · ${s.high_risk} high risk` : ''}
          </RACButton>
          <Popover className="popover"><Menu className="listbox menu">
            {(grants.data ?? []).map((g) => (
              <MenuItem key={g.id} className="option" onAction={() => revoke(g.id)}>
                Revoke {g.capability} · {g.machine_label} · {g.principal_id}
              </MenuItem>
            ))}
            <MenuItem className="option option-danger" onAction={revokeAll}>Revoke all</MenuItem>
          </Menu></Popover>
        </MenuTrigger>
      )}
      {s.pending_requests > 0 && <a href="#/dev/requests"><Button size="sm" variant="outline" data-testid="requests-badge">{s.pending_requests} requests</Button></a>}
    </div>
  );
}
