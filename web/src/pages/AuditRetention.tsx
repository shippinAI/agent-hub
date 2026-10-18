import { useState } from 'react';
import { Cell, Column, Row, Table, TableBody, TableHeader } from 'react-aria-components';
import { errText, get, post } from '../lib/api';
import type { PruneReport, RetentionView } from '../lib/actionTypes';
import { fmtBytes } from '../lib/bytes';
import { useData } from '../lib/useData';
import { Badge } from '../ui/Badge';
import { Button } from '../ui/Button';
import { Card, Notice } from '../ui/Notice';
import { RetentionCategoryEditor } from './RetentionCategoryEditor';
import { RetentionKnobs } from './RetentionKnobs';

// Admin screen for auditing: per-category switches and limits, global caps, partitions, archives, prune/flush.
export function AuditRetention() {
  const v = useData(() => get<RetentionView>('/audit/retention'));
  const [msg, setMsg] = useState<{ tone: 'success' | 'error' | 'warn'; text: string } | null>(null);
  const d = v.data;
  const total = (d?.partitions ?? []).reduce((n, p) => n + p.bytes, 0);
  async function run(what: 'prune' | 'flush') {
    try {
      const r = await post<PruneReport & { archived: unknown[] }>(`/audit/${what}`);
      const warn = r.warnings?.length ? ` · ${r.warnings.length} warning(s): ${r.warnings.join('; ')}` : '';
      setMsg({ tone: warn ? 'warn' : 'success', text: what === 'prune'
        ? `Pruned: ${r.dropped.length} partition(s) dropped, ${r.deleted_rows} row(s) trimmed, ${r.archived.length} segment(s) archived${warn}`
        : `Flushed: ${r.archived.length} segment(s) archived` });
      v.reload();
    } catch (e) { setMsg({ tone: 'error', text: errText(e) }); }
  }
  if (!d) return v.error ? <Notice tone="error">{v.error.message}</Notice> : <p role="status">Loading…</p>;
  return (
    <div className="stack gap-4">
      {!d.settings.audit_archive_dir && <Notice tone="warn">No archive folder is set: pruned rows are not archived first.</Notice>}
      {msg && <Notice tone={msg.tone}>{msg.text}</Notice>}
      <Card title="Categories" actions={<>
        <Badge tone="outline">{fmtBytes(total)} of {fmtBytes(d.settings.audit_total_max_bytes)}</Badge>
        <Button size="sm" variant="outline" onPress={() => run('flush')}>Flush to archive</Button>
        <Button size="sm" variant="primary" onPress={() => run('prune')}>Prune now</Button></>}>
        <Table aria-label="Audit categories" className="table">
          <TableHeader>
            <Column isRowHeader>Category</Column><Column>Recording</Column><Column>Keep</Column><Column>Max rows</Column>
            <Column>Max size</Column><Column>Archive</Column><Column>On disk</Column><Column>Edit</Column>
          </TableHeader>
          <TableBody items={d.categories}>
            {(c) => (
              <Row id={c.category}>
                <Cell><strong>{c.category}</strong><div className="hint">{c.description}</div></Cell>
                <Cell><Badge tone={c.enabled ? 'green' : 'neutral'}>{c.enabled ? 'on' : 'off'}</Badge></Cell>
                <Cell>{c.retain_days ? `${c.retain_days} d` : '∞'}</Cell>
                <Cell>{c.max_rows || '∞'}</Cell>
                <Cell>{c.max_bytes ? fmtBytes(c.max_bytes) : '∞'}</Cell>
                <Cell>{c.archive ? 'yes' : 'no'}</Cell>
                <Cell>{fmtBytes(d.partitions.filter((p) => p.category === c.category).reduce((n, p) => n + p.bytes, 0))}</Cell>
                <Cell><RetentionCategoryEditor c={c} onSaved={v.reload} /></Cell>
              </Row>
            )}
          </TableBody>
        </Table>
      </Card>
      <Card title="Global limits"><RetentionKnobs /><p className="hint">Archive signing: {d.settings.signing ? 'on (ed25519)' : 'off (hash-chained only)'}</p></Card>
      <Card title="Archives">
        {(d.archives ?? []).length === 0 ? <p className="muted">No archive segments yet.</p> : (
          <ul className="plain-list">{(d.archives ?? []).map((a) => (
            <li key={a.category}><strong>{a.category}</strong>: {a.segments} segment(s), {a.rows} rows, {fmtBytes(a.bytes)}</li>))}</ul>
        )}
      </Card>
      <Card title={`Partitions (${d.partitions.length})`}>
        <ul className="plain-list mono small">{d.partitions.map((p) => <li key={p.name}>{p.name} · {fmtBytes(p.bytes)} · ~{p.est_rows} rows</li>)}</ul>
      </Card>
    </div>
  );
}
