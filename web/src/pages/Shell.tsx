import type { ReactNode } from 'react';
import { Link } from 'react-aria-components';
import { token } from '../lib/api';
import { useHub } from '../lib/hub';
import { useLive } from '../lib/live';
import { Avatar, Badge } from '../ui/Badge';
import { Button } from '../ui/Button';

const NAV = [
  { id: 'board', label: 'Board', icon: 'M4 5h4v14H4zM10 5h4v9h-4zM16 5h4v6h-4z' },
  { id: 'agents', label: 'Agents', icon: 'M12 12a4 4 0 1 0 0-8 4 4 0 0 0 0 8zm-7 8a7 7 0 0 1 14 0' },
  { id: 'audit', label: 'Audit', icon: 'M5 4h14v16H5zM8 8h8M8 12h8M8 16h5' },
  { id: 'settings', label: 'Settings', icon: 'M12 15a3 3 0 1 0 0-6 3 3 0 0 0 0 6zm7-3 2-1-2-4-2 1-2-1V4h-4v3l-2 1-2-1-2 4 2 1v2l-2 1 2 4 2-1 2 1v3h4v-3l2-1 2 1 2-4-2-1z' },
];

// App frame: sidebar on desktop, top bar + bottom tab bar on phones.
export function Shell({ page, children }: { page: string; children: ReactNode }) {
  const { me } = useHub();
  const live = useLive();
  const active = page === 'task' ? 'board' : page;
  return (
    <div className="shell">
      <aside className="sidebar">
        <div className="brand"><span className="brand-mark" aria-hidden />agent-hub</div>
        <nav aria-label="Main" className="side-nav">
          {NAV.map((n) => (
            <Link key={n.id} href={`#/${n.id}`} className={`nav-item ${active === n.id ? 'is-active' : ''}`} aria-current={active === n.id ? 'page' : undefined}>
              <svg viewBox="0 0 24 24" aria-hidden><path d={n.icon} /></svg>{n.label}
            </Link>
          ))}
        </nav>
        <div className="side-foot">
          <div className="row gap-2"><Avatar name={me.principal.display_name} human={me.principal.is_human} />
            <div className="stack-0"><strong>{me.principal.display_name}</strong><span className="hint">{me.auth} · {me.principal.roles.join(', ')}</span></div>
          </div>
          {me.auth === 'token' && <Button variant="ghost" size="sm" onPress={() => { token.clear(); location.reload(); }}>Sign out</Button>}
        </div>
      </aside>
      <header className="topbar">
        <div className="brand"><span className="brand-mark" aria-hidden />agent-hub</div>
        <div className="row gap-2">
          <Badge tone={live.connected ? 'green' : 'amber'} title="Live event feed">{live.connected ? 'live' : 'offline'}</Badge>
          <Avatar name={me.principal.display_name} human={me.principal.is_human} />
        </div>
      </header>
      <main className="main" id="main">{children}</main>
      <nav aria-label="Main" className="tabbar">
        {NAV.map((n) => (
          <Link key={n.id} href={`#/${n.id}`} className={`tab-item ${active === n.id ? 'is-active' : ''}`} aria-current={active === n.id ? 'page' : undefined}>
            <svg viewBox="0 0 24 24" aria-hidden><path d={n.icon} /></svg><span>{n.label}</span>
          </Link>
        ))}
      </nav>
    </div>
  );
}
