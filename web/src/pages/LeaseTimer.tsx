import { countdown, useNow } from '../lib/time';
import type { Task } from '../lib/types';

// Live lease countdown; turns amber under a third of the TTL and red when expired.
export function LeaseTimer({ task }: { task: Task }) {
  const now = useNow();
  if (!task.holder || !task.lease_expires_at) return null;
  const { text, secs } = countdown(task.lease_expires_at, now);
  const ttl = task.lease_ttl_seconds ?? 900;
  const tone = secs <= 0 ? 'red' : secs < ttl / 3 ? 'amber' : 'green';
  const pct = Math.max(0, Math.min(100, (secs / ttl) * 100));
  return (
    <span className={`lease lease-${tone}`} title={`lease until ${new Date(task.lease_expires_at).toLocaleTimeString()}`}>
      <span className="lease-bar" aria-hidden><span style={{ width: `${pct}%` }} /></span>
      <span className="lease-text" aria-label={`lease ${text}`}>{text}</span>
    </span>
  );
}
