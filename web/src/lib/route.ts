import { useEffect, useState } from 'react';

// Tiny hash router: #/board, #/task/12, #/agents, #/settings/policies, #/audit
export function useRoute(): string[] {
  const read = () => (location.hash.replace(/^#\/?/, '') || 'board').split('/');
  const [r, setR] = useState(read);
  useEffect(() => {
    const on = () => setR(read());
    window.addEventListener('hashchange', on);
    return () => window.removeEventListener('hashchange', on);
  }, []);
  return r;
}

export const go = (path: string) => { location.hash = `#/${path}`; };
