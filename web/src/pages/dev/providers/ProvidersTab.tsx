import { useState } from 'react';
import { Cell, Column, Dialog, Modal as RACModal, ModalOverlay, Row, Table, TableBody, TableHeader } from 'react-aria-components';
import { get, errText } from '../../../lib/api';
import { useData } from '../../../lib/useData';
import type { ProviderRow } from '../../../lib/providerTypes';
import { Badge } from '../../../ui/Badge';
import { Button } from '../../../ui/Button';
import { Card, Empty, Notice } from '../../../ui/Notice';
import { Modal } from '../../../ui/Modal';
import { ProviderDetail } from './ProviderDetail';
import { ProviderForm } from './ProviderForm';
import { ProviderIcon } from './ProviderIcon';

// Catalog › Providers: every AI client or API as one data row (doc 34). A row opens its detail and edit form
// in a dialog (bottom sheet on phones); Add provider opens the same form empty. Keyboard: arrows move, Enter opens.
export function ProvidersTab() {
  const rows = useData(() => get<ProviderRow[]>('/providers'), []);
  const [sel, setSel] = useState<string | null>(null);
  if (rows.error) return <Notice tone="error">{errText(rows.error)}</Notice>;
  const data = rows.data;
  return (
    <div className="stack-3">
      <div className="row between wrap gap-2">
        <div className="stack-0"><h2 className="card-title">Providers</h2>
          <p className="hint">One row per client or API. Adapters: acp, codex_app_server, headless_json, openai_compatible, anthropic_compatible.</p></div>
        <Modal wide title="Add provider" trigger={<Button variant="primary" size="sm">Add provider</Button>}>
          {(close) => <ProviderForm onCancel={close} onSaved={(r) => { close(); rows.reload(); setSel(r.id); }} />}
        </Modal>
      </div>
      {!data ? <Card><p role="status">Loading…</p></Card> : data.length === 0 ? <Empty title="No providers" /> : (
        <>
          <Card className="table-card">
            <div className="table-wrap">
              <Table aria-label="Providers" className="table providers-table" onRowAction={(k) => setSel(String(k))}>
                <TableHeader>
                  <Column isRowHeader>Provider</Column><Column className="col-vendor">Vendor</Column>
                  <Column className="col-product">Product</Column><Column>Adapter</Column><Column className="col-editions">Editions</Column>
                </TableHeader>
                <TableBody items={data}>
                  {(p) => (
                    <Row id={p.id} textValue={p.label} className={`provider-row ${p.disabled ? 'is-disabled' : ''} ${sel === p.id ? 'is-open' : ''}`}>
                      <Cell><span className="row gap-2"><ProviderIcon iconRef={p.icon_ref} vendor={p.vendor} />
                        <span className="stack-0"><strong>{p.label}</strong><span className="hint mono">{p.id}</span></span>
                        {p.disabled && <Badge tone="amber">disabled</Badge>}</span></Cell>
                      <Cell className="col-vendor">{p.vendor}</Cell>
                      <Cell className="col-product">{p.product}</Cell>
                      <Cell>{p.adapter ? <Badge tone="blue">{p.adapter.kind}</Badge> : <span className="muted">MCP host</span>}
                        {p.fallback_adapters?.length ? <span className="hint"> +{p.fallback_adapters.length}</span> : null}</Cell>
                      <Cell className="col-editions"><span className="row wrap gap-1">{p.editions.map((e) => <Badge key={e} tone="outline">{e}</Badge>)}</span></Cell>
                    </Row>
                  )}
                </TableBody>
              </Table>
            </div>
          </Card>
          <ModalOverlay className="overlay" isDismissable isOpen={sel !== null} onOpenChange={(o) => { if (!o) setSel(null); }}>
            <RACModal className="modal modal-wide">
              <Dialog className="dialog">
                {({ close }) => sel && <ProviderDetail key={sel} id={sel} close={close} onChanged={rows.reload} />}
              </Dialog>
            </RACModal>
          </ModalOverlay>
        </>
      )}
    </div>
  );
}
