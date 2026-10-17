import { createContext, ReactNode, useContext, useEffect, useState } from 'react';
import { authHeaders } from './api';
import type { HubEvent } from './types';

type Live = { rev: number; connected: boolean; recent: HubEvent[] };
const Ctx = createContext<Live>({ rev: 0, connected: false, recent: [] });
export const useLive = () => useContext(Ctx);

// Reads the SSE feed with fetch (so the bearer header works) and reconnects from the last seq.
export function LiveProvider({ children }: { children: ReactNode }) {
  const [state, setState] = useState<Live>({ rev: 0, connected: false, recent: [] });
  useEffect(() => {
    let stop = false;
    let last = 0;
    const ctl = new AbortController();
    async function run() {
      while (!stop) {
        try {
          const q = last ? `after_seq=${last}` : 'from=now';
          const r = await fetch(`/api/v1/stream?${q}`, { headers: authHeaders(), signal: ctl.signal });
          if (!r.ok || !r.body) throw new Error(String(r.status));
          setState((s) => ({ ...s, connected: true }));
          const reader = r.body.pipeThrough(new TextDecoderStream()).getReader();
          let buf = '';
          for (;;) {
            const { value, done } = await reader.read();
            if (done) break;
            buf += value;
            const parts = buf.split('\n\n');
            buf = parts.pop() ?? '';
            const evs = parts.flatMap(parseEvent);
            if (evs.length) {
              last = evs[evs.length - 1].seq;
              setState((s) => ({ rev: s.rev + 1, connected: true, recent: [...evs.reverse(), ...s.recent].slice(0, 200) }));
            }
          }
        } catch { /* reconnect below */ }
        setState((s) => ({ ...s, connected: false }));
        if (!stop) await new Promise((res) => setTimeout(res, 2000));
      }
    }
    run();
    return () => { stop = true; ctl.abort(); };
  }, []);
  return <Ctx.Provider value={state}>{children}</Ctx.Provider>;
}

function parseEvent(block: string): HubEvent[] {
  const data = block.split('\n').filter((l) => l.startsWith('data: ')).map((l) => l.slice(6)).join('\n');
  if (!data) return [];
  try { return [JSON.parse(data) as HubEvent]; } catch { return []; }
}
