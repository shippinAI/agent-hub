import { Badge } from '../Badge';
import type { ActionEvent } from '../../lib/actionTypes';

const tone = { running: 'blue', ok: 'green', error: 'red', denied: 'amber' } as const;

// Status of a call; the exit code rides along when it is not 0 so failures read at a glance.
export function StatusBadge({ e }: { e: ActionEvent }) {
  const exit = e.exit_code != null && e.exit_code !== 0 ? ` · exit ${e.exit_code}` : '';
  return <Badge tone={tone[e.status] ?? 'neutral'} title={e.error || undefined}>{e.status}{exit}</Badge>;
}
