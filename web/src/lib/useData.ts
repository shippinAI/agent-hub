import { useCallback, useEffect, useRef, useState } from 'react';
import { useLive } from './live';

// Fetches on mount, when deps change, and (debounced) whenever the live event feed moves.
export function useData<T>(fn: () => Promise<T>, deps: unknown[] = [], live = true) {
  const [data, setData] = useState<T | null>(null);
  const [error, setError] = useState<Error | null>(null);
  const { rev } = useLive();
  const fnRef = useRef(fn);
  fnRef.current = fn;
  const load = useCallback(() => {
    fnRef.current().then((d) => { setData(d); setError(null); }).catch((e) => setError(e));
  }, []);
  // eslint-disable-next-line react-hooks/exhaustive-deps
  useEffect(load, deps);
  useEffect(() => {
    if (!live || rev === 0) return;
    const t = setTimeout(load, 250);
    return () => clearTimeout(t);
  }, [rev, live, load]);
  return { data, error, reload: load };
}
