import { Fragment } from 'react';
import { get } from '../../lib/api';
import type { TicketView } from '../../lib/gtypes';
import { statusTone, verdictTone } from '../../lib/gtypes';
import { useHub } from '../../lib/hub';
import { ago } from '../../lib/time';
import { useData } from '../../lib/useData';
import { Badge } from '../../ui/Badge';
import { Card, Notice } from '../../ui/Notice';
import { DecisionCard } from './DecisionCard';
import { ReviewForm } from './ReviewForm';

// One ticket: content, evidence, reviews (stale ones marked) and what the policy would decide for its command now.
export function ChangeDetail({ id, onChange }: { id: string; onChange: () => void }) {
  const { me } = useHub();
  const v = useData(() => get<TicketView>(`/change-tickets/${encodeURIComponent(id)}`), [id]);
  if (v.error) return <Notice tone="error">{v.error.message}</Notice>;
  if (!v.data) return <Card><p role="status">Loading…</p></Card>;
  const { ticket: t, evidence, reviews, preview } = v.data;
  const canReview = t.author !== me.principal.id && (me.principal.is_human || me.principal.roles.includes('reviewer'));
  const done = () => { v.reload(); onChange(); };
  return (
    <div className="stack-3">
      <Card title={<span className="mono">{t.id}</span>} actions={<Badge tone={statusTone[t.status] ?? 'neutral'}>{t.status.replace('_', ' ')}</Badge>}>
        <h3 className="ticket-title">{t.title}</h3>
        <dl className="kv">
          <dt>Author</dt><dd>{t.author}</dd>
          <dt>Machine</dt><dd>{t.machine ?? 'any'}</dd>
          <dt>Command</dt><dd className="mono">{t.command}</dd>
          {Object.entries(t.fields ?? {}).map(([k, val]) => <Fragment key={k}><dt>{k}</dt><dd>{val}</dd></Fragment>)}
          {Object.entries(t.lists ?? {}).map(([k, l]) => <Fragment key={k}><dt>{k}</dt><dd><ul>{l.map((x) => <li key={x}>{x}</li>)}</ul></dd></Fragment>)}
          <dt>Content hash</dt><dd className="mono hint">{t.sha256.slice(0, 16)}…</dd>
        </dl>
      </Card>
      {preview && <DecisionCard a={preview} title="If the author ran it now" />}
      <Card title={`Evidence (${evidence.length})`}>
        {evidence.length === 0 ? <p className="muted">None attached.</p> : evidence.map((e) => (
          <details key={e.path + e.sha256} className="evidence">
            <summary><Badge tone="outline">{e.kind}</Badge> <span className="mono">{e.path}</span> <span className="hint">{ago(e.captured_at)}</span></summary>
            <pre className="mono">{e.excerpt}</pre>
          </details>
        ))}
      </Card>
      <Card title={`Reviews (${reviews.length})`}>
        {reviews.length === 0 ? <p className="muted">No reviews yet.</p> : (
          <ul className="review-list">
            {reviews.map((r, i) => (
              <li key={i} className={`review ${r.current ? '' : 'is-stale'}`}>
                <div className="row gap-2"><Badge tone={verdictTone[r.verdict]}>{r.verdict}</Badge><strong>{r.reviewer}</strong>
                  <span className="hint">{ago(r.created_at)}{r.current ? '' : ' · stale (ticket changed)'}</span></div>
                <p>{r.summary}</p>
                {r.findings.length > 0 && <ul>{r.findings.map((f) => <li key={f}>{f}</li>)}</ul>}
              </li>
            ))}
          </ul>
        )}
      </Card>
      {canReview && t.status !== 'draft' && <ReviewForm ticket={t} onDone={done} />}
    </div>
  );
}
