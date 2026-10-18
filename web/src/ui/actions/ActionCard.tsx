import type { ReactNode } from 'react';
import { Button, Disclosure, DisclosurePanel, Heading } from 'react-aria-components';
import type { ActionItem } from '../../lib/actionTypes';
import { clock } from '../../lib/time';
import { Badge } from '../Badge';
import { ArgvView } from './ArgvView';
import { DiffView } from './DiffView';
import { OutputStream } from './OutputStream';
import { PolicyLine } from './PolicyLine';
import { StatusBadge } from './StatusBadge';

function Section({ title, children, open }: { title: string; children: ReactNode; open?: boolean }) {
  return (
    <Disclosure className="disclosure" defaultExpanded={open}>
      <Heading level={4} className="disclosure-head">
        <Button slot="trigger" className="disclosure-trigger">
          <svg viewBox="0 0 24 24" className="chev" aria-hidden><path d="m9 6 6 6-6 6" /></svg>{title}
        </Button>
      </Heading>
      <DisclosurePanel className="disclosure-panel">{children}</DisclosurePanel>
    </Disclosure>
  );
}

// One agent action, rendered entirely from its typed events: what ran, where, who allowed it, what came out.
export function ActionCard({ item }: { item: ActionItem }) {
  const e = { ...item.first, ...item.last, policy: item.last.policy ?? item.first.policy };
  const title = e.tool ? `${e.tool}` : e.op;
  const args = e.args ? JSON.stringify(e.args, null, 2) : '';
  return (
    <article className={`action-card action-${e.status}`} aria-label={`${title} by ${e.actor}: ${e.status}`}>
      <header className="action-head">
        <StatusBadge e={e} />
        <strong className="action-title">{title}</strong>
        <Badge tone="outline">{e.category}</Badge>
        <span className="muted">{e.actor}{e.machine ? ` → ${e.machine}` : ''}</span>
        <span className="action-meta muted">
          <time dateTime={e.at}>{clock(e.at)}</time>
          {e.duration_ms != null && e.status !== 'running' && <> · {e.duration_ms} ms</>}
          {!!e.redactions && <> · <span title="Secret values masked before storage">{e.redactions} redacted</span></>}
        </span>
      </header>
      <ArgvView argv={e.argv} cwd={e.cwd} />
      {!e.argv?.length && e.paths?.length ? <p className="mono small">{e.paths.join(', ')}</p> : null}
      <PolicyLine p={e.policy} />
      {e.error && e.status !== 'denied' && <p className="action-error" role="note">{e.error}</p>}
      {e.diff && <Section title="Diff" open><DiffView diff={e.diff} /></Section>}
      {(e.stdout || e.stderr || e.status === 'running') && <Section title="Output" open={e.status === 'error'}><OutputStream e={e} /></Section>}
      {args && args !== '{}' && <Section title="Arguments"><pre className="output">{args}</pre></Section>}
    </article>
  );
}
