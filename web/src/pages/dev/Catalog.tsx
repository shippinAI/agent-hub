import { useState } from 'react';
import { get, put, errText } from '../../lib/api';
import { useData } from '../../lib/useData';
import { Tab, TabList, TabPanel, Tabs } from 'react-aria-components';
import { SPECS } from './catalogSpec';
import { CatalogKind } from './CatalogKind';
import { ProvidersTab } from './providers/ProvidersTab';
import { Button } from '../../ui/Button';
import { TextField } from '../../ui/Field';
import { Card, Notice } from '../../ui/Notice';

// Catalog settings (06/07): editors for every editable catalog kind plus the devagotchi settings. The
// safety rules live in code; everything here is data seeded as presets and editable by an admin.
export function Catalog() {
  return (
    <Tabs className="tabs">
      <TabList aria-label="Catalog" className="tablist">
        <Tab id="providers" className="tab">Providers</Tab>
        {SPECS.map((s) => <Tab key={s.kind} id={s.kind} className="tab">{s.label}</Tab>)}
        <Tab id="settings" className="tab">Settings</Tab>
      </TabList>
      <TabPanel id="providers" className="tabpanel"><ProvidersTab /></TabPanel>
      {SPECS.map((s) => (
        <TabPanel key={s.kind} id={s.kind} className="tabpanel"><CatalogKind spec={s} /></TabPanel>
      ))}
      <TabPanel id="settings" className="tabpanel"><SettingsKnobs /></TabPanel>
    </Tabs>
  );
}

type Setting = { key: string; value: unknown; description?: string; version: number };

function SettingsKnobs() {
  const cfg = useData(() => get<{ settings: Setting[] }>('/config'), []);
  if (cfg.error) return <Notice tone="error">{errText(cfg.error)}</Notice>;
  const settings = (cfg.data?.settings ?? []).filter((s) => s.key.startsWith('devagotchi.') || s.key.startsWith('guide') || s.key.startsWith('gui.'));
  return (
    <Card title="Devagotchi settings">
      {!cfg.data ? <p role="status">Loading…</p>
        : settings.length === 0 ? <p className="hint">No devagotchi settings found.</p>
        : <ul className="knobs">{settings.map((s) => <Knob key={s.key} s={s} onSaved={cfg.reload} />)}</ul>}
    </Card>
  );
}

function Knob({ s, onSaved }: { s: Setting; onSaved: () => void }) {
  const isList = Array.isArray(s.value);
  const show = isList ? (s.value as unknown[]).join(', ') : JSON.stringify(s.value);
  const [v, setV] = useState(show);
  const [err, setErr] = useState('');
  const parse = () => (isList ? v.split(',').map((x) => x.trim()).filter(Boolean) : JSON.parse(v));
  return (
    <li className="knob">
      <div className="stack-0 grow"><code>{s.key}</code>{s.description && <span className="hint">{s.description}</span>}</div>
      <TextField label={s.key} value={v} onChange={setV} className="knob-input" />
      <Button size="sm" isDisabled={v === show} onPress={async () => {
        setErr(''); try { await put(`/settings/${s.key}`, { value: parse(), version: s.version }); onSaved(); } catch (e) { setErr(errText(e)); }
      }}>Save</Button>
      {err && <Notice tone="error">{err}</Notice>}
    </li>
  );
}
