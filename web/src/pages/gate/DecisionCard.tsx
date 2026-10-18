import type { Answer } from '../../lib/gtypes';
import { Badge } from '../../ui/Badge';

// One decision: allow/deny, which rules matched, and why each failing rule blocked.
export function DecisionCard({ a, title }: { a: Answer; title?: string }) {
  const d = a.decision;
  return (
    <section className={`card decision ${a.allow ? 'is-allow' : 'is-deny'}`} aria-label={title ?? 'Policy decision'}>
      <div className="row between wrap gap-2">
        <div className="row gap-2">
          <Badge tone={a.allow ? 'green' : 'red'}>{a.allow ? 'ALLOW' : 'DENY'}</Badge>
          {title && <strong>{title}</strong>}
        </div>
        <span className="hint mono">policy v{d.policy_version} · {a.tenant_id}</span>
      </div>
      <div className="row wrap gap-2">
        {(d.classes ?? []).map((c) => <Badge key={c} tone="amber">{c}</Badge>)}
        {(d.regions ?? []).map((r) => <Badge key={r} tone="red">region {r}</Badge>)}
        {(d.matched ?? []).map((m) => <Badge key={m} tone="outline">{m}</Badge>)}
      </div>
      {(d.failures ?? []).map((f) => (
        <div key={f.rule} className="failure">
          <strong className="mono">[{f.rule}]</strong> {f.message}
          <ul>{f.why.map((w, i) => <li key={i}>{w}</li>)}</ul>
        </div>
      ))}
      {a.allow && <p className="muted">{a.reason}</p>}
    </section>
  );
}
