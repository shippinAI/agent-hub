import { Cell, Column, Row, Table, TableBody, TableHeader } from 'react-aria-components';
import { useHub } from '../lib/hub';
import type { Policy } from '../lib/types';
import { Badge } from '../ui/Badge';
import { Card } from '../ui/Notice';
import { PolicyEditor } from './PolicyEditor';

const sel = (p: Policy) => [...(p.selector.projects ?? []).map((x) => `project:${x}`), ...(p.selector.labels ?? []).map((x) => `label:${x}`)].join(' ') || 'any';

// Rules are evaluated highest priority first; at equal priority deny wins; nothing matching = deny.
export function Policies() {
  const { config, me } = useHub();
  const edit = me.can['admin.policies.put'];
  return (
    <Card title="Policies" actions={edit && <PolicyEditor />}>
      <p className="hint">Subjects: <code>*</code> <code>human</code> <code>agent</code> <code>role:ID</code> <code>principal:ID</code> <code>kind:K</code>.
        Actions are globs, e.g. <code>task.claim:ready</code>, <code>task.transition:review-&gt;done</code>, <code>admin.*</code>.
        Code-level safety rules (humans-only admin and force-release, fencing, versions) apply on top.</p>
      <ul className="policy-cards only-mobile">
        {config.policies.map((p) => (
          <li key={p.id} className={`policy-card ${p.enabled ? '' : 'is-disabled'}`}>
            <div className="row between gap-2"><strong>{p.name}</strong><Badge tone={p.effect === 'allow' ? 'green' : 'red'}>{p.effect}</Badge></div>
            <div className="row wrap gap-1"><code>{p.subject}</code><span className="hint">→</span><code>{p.action}</code></div>
            <div className="row between gap-2"><span className="hint">{sel(p)} · priority {p.priority}</span>{edit && <PolicyEditor policy={p} />}</div>
          </li>
        ))}
      </ul>
      <div className="table-wrap only-desktop">
        <Table aria-label="Policies" className="table responsive">
          <TableHeader>
            <Column isRowHeader>Name</Column><Column>Subject</Column><Column>Action</Column><Column>Selector</Column>
            <Column>Effect</Column><Column>Priority</Column><Column>{''}</Column>
          </TableHeader>
          <TableBody items={config.policies}>
            {(p) => (
              <Row id={p.id} className={p.enabled ? '' : 'is-disabled'}>
                <Cell><span data-label="Name">{p.name}</span></Cell>
                <Cell><code data-label="Subject">{p.subject}</code></Cell>
                <Cell><code data-label="Action">{p.action}</code></Cell>
                <Cell><span className="hint" data-label="Selector">{sel(p)}</span></Cell>
                <Cell><Badge tone={p.effect === 'allow' ? 'green' : 'red'}>{p.effect}</Badge></Cell>
                <Cell><span className="mono" data-label="Priority">{p.priority}</span></Cell>
                <Cell>{edit ? <PolicyEditor policy={p} /> : null}</Cell>
              </Row>
            )}
          </TableBody>
        </Table>
      </div>
    </Card>
  );
}
