import { Tab, TabList, TabPanel, Tabs } from 'react-aria-components';
import { useHub } from '../lib/hub';
import { go } from '../lib/route';
import { Notice } from '../ui/Notice';
import { Knobs } from './Knobs';
import { Policies } from './Policies';
import { Roles } from './Roles';
import { Workflows } from './Workflows';

// Everything here is data: editing it changes behaviour without a deploy. Writes need a human admin.
export function Settings({ tab }: { tab: string }) {
  const { me } = useHub();
  return (
    <div className="page">
      <div className="page-head">
        <div><h1 className="page-title">Settings</h1>
          <p className="muted">Roles, policies, workflows and lease defaults are data. Every change is versioned and audited.</p></div>
      </div>
      {!me.can['admin.policies.put'] && <Notice tone="info">Read-only: configuration changes need a human principal with an admin policy.</Notice>}
      <Tabs selectedKey={tab} onSelectionChange={(k) => go(`settings/${k}`)} className="tabs">
        <TabList aria-label="Settings sections" className="tablist">
          <Tab id="policies" className="tab">Policies</Tab>
          <Tab id="roles" className="tab">Roles</Tab>
          <Tab id="workflows" className="tab">Workflows</Tab>
          <Tab id="lease" className="tab">Leases & limits</Tab>
        </TabList>
        <TabPanel id="policies" className="tabpanel"><Policies /></TabPanel>
        <TabPanel id="roles" className="tabpanel"><Roles /></TabPanel>
        <TabPanel id="workflows" className="tabpanel"><Workflows /></TabPanel>
        <TabPanel id="lease" className="tabpanel"><Knobs /></TabPanel>
      </Tabs>
    </div>
  );
}
