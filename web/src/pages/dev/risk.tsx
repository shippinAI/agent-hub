import type { Risk } from '../../lib/dtypes';
import { Badge } from '../../ui/Badge';

const tone: Record<string, 'neutral' | 'amber' | 'red'> = { low: 'neutral', med: 'amber', high: 'red', critical: 'red' };
const icon: Record<string, string> = { low: 'ℹ', med: '▲', high: '⚠', critical: '⛔' };

// Severity shown as text + icon, never colour alone (07 copy rules).
export function Severity({ level }: { level: string }) {
  return <Badge tone={tone[level] ?? 'neutral'}>{icon[level] ?? '•'} {level}</Badge>;
}

export function RiskList({ risks }: { risks: Risk[] | null }) {
  if (!risks || risks.length === 0) return <p className="muted">No specific risks flagged.</p>;
  return (
    <ul className="risk-list">
      {risks.map((r) => (
        <li key={r.id} className="risk-item">
          <div className="row gap-2"><Severity level={r.severity} /><strong>{r.title}</strong></div>
          <p className="muted">{r.plain_text}</p>
          {r.mitigation && <p className="hint">Mitigation: {r.mitigation}</p>}
        </li>
      ))}
    </ul>
  );
}

// A compact selector summary (paths, argv prefixes, hosts, apps).
export function SelectorText({ selector }: { selector: Record<string, unknown> }) {
  const parts: string[] = [];
  const s = selector as Record<string, unknown>;
  if (Array.isArray(s.paths)) parts.push(`paths: ${(s.paths as string[]).join(', ')}`);
  if (Array.isArray(s.argv_prefix)) parts.push(`commands: ${(s.argv_prefix as string[][]).map((a) => a.join(' ')).join(' | ')}`);
  if (Array.isArray(s.hosts)) parts.push(`hosts: ${(s.hosts as string[]).join(', ')}`);
  if (Array.isArray(s.apps)) parts.push(`apps: ${(s.apps as string[]).join(', ')}`);
  if (s.apps === 'any') parts.push('apps: any');
  return <code className="selector">{parts.length ? parts.join(' · ') : 'whole machine'}</code>;
}
