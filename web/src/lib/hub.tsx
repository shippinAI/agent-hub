import { createContext, ReactNode, useContext } from 'react';
import { get } from './api';
import { useData } from './useData';
import type { HubConfig, Me, Principal, Workflow } from './types';

type Hub = { me: Me; config: HubConfig; reloadConfig: () => void };
const Ctx = createContext<Hub | null>(null);

export function useHub(): Hub {
  const h = useContext(Ctx);
  if (!h) throw new Error('useHub outside HubProvider');
  return h;
}

// Loads /me + /config once and again whenever the event feed moves (config edits are events too).
export function HubProvider({ me, children }: { me: Me; children: ReactNode }) {
  const cfg = useData(() => get<HubConfig>('/config'));
  if (!cfg.data) return <div className="page-loading" role="status">{cfg.error ? cfg.error.message : 'Loading hub…'}</div>;
  return <Ctx.Provider value={{ me, config: cfg.data, reloadConfig: cfg.reload }}>{children}</Ctx.Provider>;
}

export function workflowFor(cfg: HubConfig, project: string): Workflow | undefined {
  const p = cfg.projects.find((x) => x.id === project);
  return cfg.workflows.find((w) => w.id === p?.workflow_id);
}

export function setting<T>(cfg: HubConfig, key: string, fallback: T): T {
  const s = cfg.settings.find((x) => x.key === key);
  return (s?.value as T) ?? fallback;
}

export function principalName(cfg: HubConfig, id: string | null): string {
  if (!id) return '';
  return cfg.principals.find((p: Principal) => p.id === id)?.display_name ?? id;
}

export function stateKind(wf: Workflow | undefined, key: string): string {
  return wf?.states.find((s) => s.key === key)?.kind ?? 'queue';
}
