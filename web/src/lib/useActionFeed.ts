import { useEffect, useMemo, useState } from 'react';
import { get } from './api';
import type { ActionEvent, ActionItem } from './actionTypes';

type Filter = { category: string; actor: string };

// Tails /actions with long-polls (wait=25s, resumes from the last id) and folds events by call_id.
export function useActionFeed(filter: Filter, limit = 300) {
  const [events, setEvents] = useState<ActionEvent[]>([]);
  const [error, setError] = useState<Error | null>(null);
  const [live, setLive] = useState(false);
  useEffect(() => {
    let stop = false;
    const ctl = new AbortController();
    const q = `category=${encodeURIComponent(filter.category)}&actor=${encodeURIComponent(filter.actor)}`;
    setEvents([]);
    async function run() {
      let after = 0;
      try {
        const first = await get<{ actions: ActionEvent[]; head: number }>(`/actions?order=desc&limit=${limit}&${q}`);
        const initial = [...first.actions].reverse();
        after = initial.length ? initial[initial.length - 1].id : first.head;
        setEvents(initial);
        setError(null);
      } catch (e) { setError(e as Error); }
      while (!stop) {
        try {
          setLive(true);
          const r = await get<{ actions: ActionEvent[] }>(`/actions?after_id=${after}&wait=25&limit=500&${q}`);
          if (r.actions.length) {
            after = r.actions[r.actions.length - 1].id;
            setEvents((cur) => [...cur, ...r.actions].slice(-limit * 2));
          }
        } catch (e) {
          setLive(false);
          if (!stop) { setError(e as Error); await new Promise((res) => setTimeout(res, 3000)); }
        }
      }
    }
    run();
    return () => { stop = true; ctl.abort(); };
  }, [filter.category, filter.actor, limit]);
  const items = useMemo(() => fold(events), [events]);
  return { items, error, live };
}

export function fold(events: ActionEvent[]): ActionItem[] {
  const byKey = new Map<string, ActionItem>();
  for (const e of events) {
    const key = e.call_id ? `call:${e.call_id}` : `ev:${e.id}`;
    const cur = byKey.get(key);
    if (cur) { cur.events.push(e); cur.last = e; } else byKey.set(key, { key, first: e, last: e, events: [e] });
  }
  return [...byKey.values()].sort((a, b) => b.last.id - a.last.id);
}
