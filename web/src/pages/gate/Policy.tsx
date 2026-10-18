import { Tab, TabList, TabPanel, Tabs } from 'react-aria-components';
import { get } from '../../lib/api';
import type { DocRow } from '../../lib/gtypes';
import { useHub } from '../../lib/hub';
import { go } from '../../lib/route';
import { useData } from '../../lib/useData';
import { Badge } from '../../ui/Badge';
import { Notice } from '../../ui/Notice';
import { Decisions } from './Decisions';
import { JsonEditor } from './JsonEditor';
import { RulesEditor } from './RulesEditor';
import { TryIt } from './TryIt';
import { Versions } from './Versions';

// Change policy: the tenant's versioned rule document, a try-it box, history and the decision log (14).
export function Policy({ tab }: { tab: string }) {
  const { me } = useHub();
  const doc = useData(() => get<DocRow>('/policy/doc'), []);
  const d = doc.data;
  const canEdit = me.principal.is_human && me.principal.roles.includes('admin');
  return (
    <div className="page">
      <div className="page-head">
        <div><h1 className="page-title">Change policy</h1>
          <p className="muted">Rules are data, versioned per tenant. Every bridge call is checked before its grant, and an error denies it.</p></div>
        {d && <div className="row gap-2"><Badge tone="blue">v{d.version}</Badge><Badge tone="outline" title={d.sha256}>tenant {d.tenant_id}</Badge></div>}
      </div>
      {doc.error && <Notice tone="error">{doc.error.message}</Notice>}
      {!canEdit && <Notice tone="info">Read-only: editing the policy needs a human admin.</Notice>}
      <Tabs selectedKey={tab} onSelectionChange={(k) => go(`dev/policy/${k}`)} className="tabs">
        <TabList aria-label="Policy sections" className="tablist">
          <Tab id="rules" className="tab">Rules</Tab>
          <Tab id="try" className="tab">Try it</Tab>
          <Tab id="decisions" className="tab">Decisions</Tab>
          <Tab id="versions" className="tab">Versions</Tab>
          <Tab id="json" className="tab">JSON</Tab>
        </TabList>
        <TabPanel id="rules" className="tabpanel">{d && <RulesEditor row={d} canEdit={canEdit} onSaved={doc.reload} />}</TabPanel>
        <TabPanel id="try" className="tabpanel"><TryIt /></TabPanel>
        <TabPanel id="decisions" className="tabpanel"><Decisions /></TabPanel>
        <TabPanel id="versions" className="tabpanel">{d && <Versions current={d} canEdit={canEdit} onSaved={doc.reload} />}</TabPanel>
        <TabPanel id="json" className="tabpanel">{d && <JsonEditor key={d.version} row={d} canEdit={canEdit} onSaved={doc.reload} />}</TabPanel>
      </Tabs>
    </div>
  );
}
