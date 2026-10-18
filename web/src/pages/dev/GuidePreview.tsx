import { useEffect, useState } from 'react';
import { post, errText } from '../../lib/api';
import { GuideRunner } from './GuideRunner';
import { Select } from '../../ui/Select';
import { Notice } from '../../ui/Notice';

const OSES = [
  { id: 'linux', label: 'Linux (native)' },
  { id: 'darwin', label: 'macOS' },
  { id: 'windows', label: 'Windows' },
];

// Preview tab: start a dry run against a sample profile (no machine) and render the real runner side
// panel, so authors see exactly what an operator would see. Needs a published version to settle steps.
export function GuidePreview({ guideId }: { guideId: string }) {
  const [os, setOs] = useState('linux');
  const [runId, setRunId] = useState<string | null>(null);
  const [err, setErr] = useState<string | null>(null);
  const start = async (forOs: string) => {
    setErr(null); setRunId(null);
    try {
      const r = await post<{ run_id: string }>('/guide-runs', {
        guide: guideId, dry_run: true, profile: { os: forOs, arch: 'arm64', kind: 'native', display_server: forOs === 'linux' ? 'x11' : '' },
      });
      setRunId(r.run_id);
    } catch (e) { setErr(errText(e)); }
  };
  useEffect(() => { start(os); /* eslint-disable-next-line */ }, []);
  return (
    <div className="split">
      <div className="stack-2">
        <Select label="Sample machine profile" options={OSES} value={os} onChange={(v) => { setOs(v); start(v); }} className="w-sm" />
        {err && <Notice tone="error">{err}</Notice>}
        {!err && !runId && <Notice tone="info">Starting preview…</Notice>}
        <p className="hint">Preview runs as a dry run: no commands touch a machine. Publish a version first if preview says there is nothing to run.</p>
      </div>
      {runId && <GuideRunner runId={runId} />}
    </div>
  );
}
