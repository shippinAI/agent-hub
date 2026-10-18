import { Tab, TabList, TabPanel, Tabs } from 'react-aria-components';
import type { ActionEvent } from '../../lib/actionTypes';

// stdout / stderr as tabs; a running call's panel is a polite live region so new output is announced.
export function OutputStream({ e }: { e: ActionEvent }) {
  const streams = [['stdout', e.stdout], ['stderr', e.stderr]].filter(([, v]) => v) as [string, string][];
  if (!streams.length) return <p className="muted">{e.status === 'running' ? 'Waiting for output…' : 'No output captured.'}</p>;
  return (
    <Tabs className="tabs output-tabs" defaultSelectedKey={e.stderr && e.status === 'error' ? 'stderr' : streams[0][0]}>
      <TabList aria-label="Output streams" className="tablist">
        {streams.map(([k]) => <Tab key={k} id={k} className="tab">{k}</Tab>)}
      </TabList>
      {streams.map(([k, v]) => (
        <TabPanel key={k} id={k} className="tabpanel">
          <pre className={`output output-${k}`} aria-live={e.status === 'running' ? 'polite' : 'off'}>{v}</pre>
        </TabPanel>
      ))}
      {e.truncated && <p className="hint">Truncated to the category's output cap{e.out_sha256 ? ` · full output sha256 ${e.out_sha256.slice(0, 16)}…` : ''}.</p>}
    </Tabs>
  );
}
