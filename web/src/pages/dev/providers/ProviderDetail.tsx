import { useEffect, useRef, useState } from 'react';
import { Heading } from 'react-aria-components';
import { get, post, errText } from '../../../lib/api';
import { useData } from '../../../lib/useData';
import type { ProviderDetail as Detail, ProviderRow } from '../../../lib/providerTypes';
import { CAPABILITIES } from '../../../lib/providerTypes';
import { Badge } from '../../../ui/Badge';
import { Button } from '../../../ui/Button';
import { Kv, Notice } from '../../../ui/Notice';
import { ProviderForm } from './ProviderForm';
import { ProviderIcon } from './ProviderIcon';

const list = (xs?: string[]) => (xs?.length ? xs.map((x) => <Badge key={x} tone="outline">{x.replace(/_/g, ' ')}</Badge>) : <span className="muted">—</span>);
const js = (v: unknown) => <pre className="code-block">{JSON.stringify(v, null, 2)}</pre>;

// One row in full, shown inside the providers dialog (its Heading names the dialog), with Edit (inline form)
// and Disable/Enable. Rows are never deleted from here.
export function ProviderDetail({ id, onChanged, close }: { id: string; onChanged: () => void; close: () => void }) {
  const row = useData(() => get<Detail>(`/providers/${encodeURIComponent(id)}`), [id]);
  const [editing, setEditing] = useState(false);
  const [err, setErr] = useState('');
  const editBtn = useRef<HTMLButtonElement>(null);
  const wasEditing = useRef(false);
  useEffect(() => { // leaving the form unmounts the focused element; put focus back on Edit, inside the dialog
    if (wasEditing.current && !editing) editBtn.current?.focus();
    wasEditing.current = editing;
  }, [editing, row.data]);
  if (row.error) return <><Heading slot="title" className="dialog-title">Provider</Heading><Notice tone="error">{errText(row.error)}</Notice></>;
  const p = row.data;
  if (!p) return <><Heading slot="title" className="dialog-title">Provider</Heading><p role="status">Loading…</p></>;
  const toggle = async () => {
    setErr('');
    try { await post<ProviderRow>(`/providers/${encodeURIComponent(id)}/${p.disabled ? 'enable' : 'disable'}`); row.reload(); onChanged(); }
    catch (e) { setErr(errText(e)); }
  };
  if (editing) {
    return (
      <>
        <Heading slot="title" className="dialog-title">Edit {p.label}</Heading>
        <ProviderForm row={p} onCancel={() => setEditing(false)} onSaved={() => { setEditing(false); row.reload(); onChanged(); }} />
      </>
    );
  }
  const router = p.adapter?.config?.router;
  return (
    <div className="stack-3 provider-detail">
      <header className="row between wrap gap-2">
        <Heading slot="title" className="dialog-title row gap-2"><ProviderIcon iconRef={p.icon_ref} vendor={p.vendor} />{p.label}
          {p.disabled && <Badge tone="amber">disabled</Badge>}</Heading>
        <div className="row gap-1"><Button ref={editBtn} size="sm" onPress={() => setEditing(true)}>Edit</Button>
          <Button size="sm" variant={p.disabled ? 'secondary' : 'ghost'} onPress={toggle}>{p.disabled ? 'Enable' : 'Disable'}</Button>
          <Button size="sm" variant="ghost" onPress={close}>Close</Button></div>
      </header>
      {err && <Notice tone="error">{err}</Notice>}
      <dl className="stack-2 provider-kv">
        <Kv k="ID"><code>{p.id}</code> <span className="hint">schema v{p.schema_version} · row v{p.version}</span></Kv>
        <Kv k="Vendor">{p.vendor} · {p.product}</Kv>
        <Kv k="Sign-in"><span className="row wrap gap-1">{list(p.auth_kinds)}</span></Kv>
        <Kv k="Runs on"><span className="row wrap gap-1">{list(p.runner)}</span></Kv>
        <Kv k="Editions"><span className="row wrap gap-1">{list(p.editions)}</span></Kv>
        <Kv k="Adapter">{p.adapter ? <><Badge tone="blue">{p.adapter.kind}</Badge>{router && <span className="hint"> router {router.mode}{router.endpoint_id ? ` → ${router.endpoint_id}` : ''}</span>}</>
          : <span className="muted">none (MCP host only)</span>}{p.host_for && <Badge tone="violet">in a harness</Badge>}</Kv>
        <Kv k="Fallbacks"><span className="row wrap gap-1">{list(p.fallback_adapters?.map((a) => a.kind))}</span></Kv>
        <Kv k="Models">{p.models.source}{p.models.ref && <code> {p.models.ref}</code>} <span className="hint">every {p.models.refresh_minutes} min</span></Kv>
        <Kv k="Cached">{p.model_cache.length ? <span className="row wrap gap-1">{p.model_cache.map((m) => <code key={m.model}>{m.model}</code>)}</span> : <span className="muted">none yet</span>}</Kv>
        <Kv k="Usage">tokens {p.usage.tokens} · quota {p.usage.quota}{p.usage.quota_ref && <code> {p.usage.quota_ref}</code>} · {p.usage.cost_basis.replace(/_/g, ' ')}</Kv>
        <Kv k="History">{p.state.read}{p.state.ref && <code> {p.state.ref}</code>}{p.state.parser && <span className="hint"> parser {p.state.parser}</span>}</Kv>
        <Kv k="Can"><span className="row wrap gap-1">{list(CAPABILITIES.filter((c) => p.capabilities[c]))}</span></Kv>
        {p.docs_url && <Kv k="Docs"><a href={p.docs_url} target="_blank" rel="noreferrer">{p.docs_url}</a></Kv>}
        {(p.terms_url || p.terms_note) && <Kv k="Terms"><span>{p.terms_note} {p.terms_url && <a href={p.terms_url} target="_blank" rel="noreferrer">terms</a>}</span></Kv>}
      </dl>
      <details className="provider-raw"><summary>Adapter config, detection and paths</summary>
        {p.adapter && js(p.adapter.config)}{p.fallback_adapters?.length ? js(p.fallback_adapters) : null}{js(p.detect)}{p.state.paths && js(p.state.paths)}
      </details>
    </div>
  );
}
