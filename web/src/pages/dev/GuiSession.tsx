import { useState } from 'react';
import { get, post, errText } from '../../lib/api';
import { useData } from '../../lib/useData';
import type { GuiSession as Sess, GuiArtifact } from '../../lib/dtypes';
import { Badge } from '../../ui/Badge';
import { Button } from '../../ui/Button';
import { Card, Kv, Notice } from '../../ui/Notice';
import { useNow, countdown, clock, ago } from '../../lib/time';

type Detail = { session: Sess; artifacts: GuiArtifact[] };
type Ev = { seq: number; at: string; actor: string; action: string; data: Record<string, unknown> };

// GUI session live view (07): latest screenshot metadata (images are not stored, only their sha/size),
// the action log, a lease countdown, and Stop. It refreshes live via the SSE-backed useData hook.
export function GuiSession({ id }: { id: string }) {
  const d = useData(() => get<Detail>(`/gui-sessions/${id}`), [id]);
  const log = useData(() => get<Ev[]>('/events?action=gui.&order=desc&limit=100'), [id]);
  const now = useNow();
  const [err, setErr] = useState<string | null>(null);
  if (d.error) return <Notice tone="error">{errText(d.error)}</Notice>;
  if (!d.data) return <Card><p role="status">Loading…</p></Card>;
  const { session: s, artifacts } = d.data;
  const last = artifacts[0];
  const stop = async () => { setErr(null); try { await post(`/gui-sessions/${id}/end`); d.reload(); } catch (e) { setErr(errText(e)); } };
  const evs = (log.data ?? []).filter((e) => String((e.data ?? {}).session ?? (e.data ?? {}).gui_session ?? '') === id);
  const live = s.state === 'active' && !s.ended_at;
  return (
    <div className="page">
      <header className="page-head">
        <div><h1>GUI session</h1><p className="muted">{s.machine_id} · held by {s.holder}</p></div>
        <div className="row gap-2">
          <Badge tone={live ? 'green' : 'neutral'}>{live ? 'live' : s.state}</Badge>
          {live && <Button variant="destructive" onPress={stop}>Stop</Button>}
          <a href="#/dev/gui-sessions"><Button variant="ghost">All sessions</Button></a>
        </div>
      </header>
      {err && <Notice tone="error">{err}</Notice>}
      <div className="split">
        <Card title="Latest frame">
          {last ? (
            <div className="frame-meta">
              <div className="frame-placeholder" aria-hidden><span>{last.width}×{last.height}</span></div>
              <dl className="kv-grid">
                <Kv k="Captured">{clock(last.created_at)}</Kv>
                <Kv k="Size">{last.width}×{last.height}</Kv>
                <Kv k="Bytes">{last.bytes.toLocaleString()}</Kv>
                <Kv k="sha256">{last.sha256.slice(0, 16)}…</Kv>
                <Kv k="Stored">{last.has_image ? 'yes (retention)' : 'no — sent to agent only'}</Kv>
                <Kv k="Redacted">{last.redacted ? 'yes' : 'no'}</Kv>
              </dl>
            </div>
          ) : <p className="hint">No frames captured yet.</p>}
          <p className="hint">Lease ends in {countdown(s.lease_expires_at, now).text}.</p>
        </Card>
        <Card title="Action log">
          {evs.length === 0 ? <p className="hint">No GUI actions recorded for this session.</p>
            : <ul className="log-list">{evs.map((e) => (
                <li key={e.seq} className="log-row"><Badge tone="blue">{e.action}</Badge>
                  <span>{e.actor}</span><span className="hint">{ago(e.at, now)}</span></li>))}</ul>}
        </Card>
      </div>
    </div>
  );
}
