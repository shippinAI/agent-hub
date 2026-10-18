import { useEffect, useState } from 'react';
import { get, post, errText } from '../../lib/api';
import { useData } from '../../lib/useData';
import type { Guide, GuideStep } from '../../lib/dtypes';
import { Tab, TabList, TabPanel, Tabs } from 'react-aria-components';
import { Button } from '../../ui/Button';
import { TextField } from '../../ui/Field';
import { Card, Notice } from '../../ui/Notice';
import { GuideSteps } from './GuideSteps';
import { GuideVersions } from './GuideVersions';
import { GuidePreview } from './GuidePreview';

// Guide editor (07): Steps / Conditions / Preview / Versions / JSON. Saving writes a new draft version
// (POST /guides/{id}/versions) with a bumped semver; publishing happens on the Versions tab.
export function GuideEditor({ id }: { id: string }) {
  const guide = useData(() => get<Guide>(`/guides/${id}`), [id]);
  const [steps, setSteps] = useState<GuideStep[]>([]);
  const [notes, setNotes] = useState('');
  const [semver, setSemver] = useState('');
  const [msg, setMsg] = useState<string | null>(null);
  const [err, setErr] = useState<string | null>(null);
  useEffect(() => {
    if (guide.data) { setSteps((guide.data.steps as GuideStep[]) ?? []); setSemver(bump(guide.data.semver)); }
  }, [guide.data]);
  if (guide.error) return <Notice tone="error">{errText(guide.error)}</Notice>;
  if (!guide.data) return <Card><p role="status">Loading…</p></Card>;
  const g = guide.data;
  const saveDraft = async () => {
    setErr(null); setMsg(null);
    try {
      const r = await post<{ version_id: string }>(`/guides/${g.id}/versions`, {
        title: g.title, goal: g.goal, tags: g.tags, applies_to: g.applies_to, semver, change_notes: notes,
        risk_ids: g.risk_ids, steps,
      });
      setMsg(`Saved draft version (${r.version_id.slice(0, 8)}). Publish it on the Versions tab.`);
      guide.reload();
    } catch (e) { setErr(errText(e)); }
  };
  return (
    <div className="page">
      <header className="page-head"><div><h1>{g.title}</h1><p className="muted">{g.goal}</p></div>
        <a href="#/dev/guides"><Button variant="ghost">All guides</Button></a></header>
      {msg && <Notice tone="success">{msg}</Notice>}
      {err && <Notice tone="error">{err}</Notice>}
      <Tabs className="tabs">
        <TabList aria-label="Guide editor" className="tablist">
          <Tab id="steps" className="tab">Steps</Tab>
          <Tab id="conditions" className="tab">Conditions</Tab>
          <Tab id="preview" className="tab">Preview</Tab>
          <Tab id="versions" className="tab">Versions</Tab>
          <Tab id="json" className="tab">JSON</Tab>
        </TabList>
        <TabPanel id="steps" className="tabpanel">
          <GuideSteps steps={steps} onChange={setSteps} />
          <SaveBar semver={semver} setSemver={setSemver} notes={notes} setNotes={setNotes} onSave={saveDraft} />
        </TabPanel>
        <TabPanel id="conditions" className="tabpanel"><Conditions applies={g.applies_to} /></TabPanel>
        <TabPanel id="preview" className="tabpanel"><GuidePreview guideId={g.id} /></TabPanel>
        <TabPanel id="versions" className="tabpanel"><GuideVersions guideId={g.id} onChanged={guide.reload} /></TabPanel>
        <TabPanel id="json" className="tabpanel"><JsonTab steps={steps} onChange={setSteps} setErr={setErr} /></TabPanel>
      </Tabs>
    </div>
  );
}

function SaveBar({ semver, setSemver, notes, setNotes, onSave }: {
  semver: string; setSemver: (v: string) => void; notes: string; setNotes: (v: string) => void; onSave: () => void;
}) {
  return (
    <Card title="Save as draft">
      <div className="row gap-2 wrap">
        <TextField label="Semver" value={semver} onChange={setSemver} className="w-sm" />
        <TextField label="Change notes" value={notes} onChange={setNotes} className="grow" />
        <Button variant="primary" isDisabled={!semver || !notes} onPress={onSave}>Save draft</Button>
      </div>
    </Card>
  );
}

function Conditions({ applies }: { applies: Record<string, unknown> }) {
  const os = (applies.os as string[] | undefined)?.join(', ') ?? 'any';
  return <Card title="Conditions"><p>Applies to OS: <strong>{os}</strong></p>
    <p className="hint">Conditions (OS, arch, display server) gate which machines a guide offers. Edit the applies_to JSON on the JSON tab for now.</p></Card>;
}

function JsonTab({ steps, onChange, setErr }: { steps: GuideStep[]; onChange: (s: GuideStep[]) => void; setErr: (e: string | null) => void }) {
  const [text, setText] = useState(JSON.stringify(steps, null, 2));
  useEffect(() => setText(JSON.stringify(steps, null, 2)), [steps]);
  return (
    <Card title="Steps JSON (schema-validated on save)">
      <TextField label="JSON" value={text} onChange={setText} multiline rows={18} className="mono" />
      <Button onPress={() => { try { onChange(JSON.parse(text)); setErr(null); } catch (e) { setErr(`Invalid JSON: ${(e as Error).message}`); } }}>Apply to steps</Button>
    </Card>
  );
}

function bump(semver: string | null): string {
  if (!semver) return '1.0.0';
  const p = semver.split('.').map(Number);
  if (p.length === 3) { p[2] += 1; return p.join('.'); }
  return semver;
}
