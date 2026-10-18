import type { ReactNode } from 'react';
import { Link } from 'react-aria-components';
import { token } from '../lib/api';
import { useHub } from '../lib/hub';
import { useLive } from '../lib/live';
import { Avatar, Badge } from '../ui/Badge';
import { GrantsChip } from './dev/GrantsChip';
import { Button } from '../ui/Button';

const NAV = [
  { id: 'board', href: 'board', label: 'Board', icon: 'M4 5h4v14H4zM10 5h4v9h-4zM16 5h4v6h-4z' },
  { id: 'machines', href: 'dev/machines', label: 'Machines', icon: 'M4 5h16v10H4zM8 19h8M12 15v4' },
  { id: 'requests', href: 'dev/requests', label: 'Requests', icon: 'M4 4h16v12H5l-1 4z' },
  { id: 'grants', href: 'dev/grants', label: 'Grants', icon: 'M12 2 4 6v6c0 5 8 8 8 8s8-3 8-8V6z' },
  { id: 'subs', href: 'dev/subscriptions', label: 'Subscriptions', icon: 'M3 7h18v10H3zM3 11h18' },
  { id: 'role-bindings', href: 'dev/role-bindings', label: 'Bindings', icon: 'M7 7h4v4H7zM13 13h4v4h-4zM11 9h2v6' },
  { id: 'policy', href: 'dev/policy', label: 'Policy', icon: 'M12 2 4 6v6c0 5 8 8 8 8s8-3 8-8V6zM9 12l2 2 4-4' },
  { id: 'changes', href: 'dev/changes', label: 'Changes', icon: 'M5 4h14v16H5zM8 9l2 2 4-4M8 15h8' },
  { id: 'guides', href: 'dev/guides', label: 'Guides', icon: 'M5 4h11l3 3v13H5zM9 9h6M9 13h6' },
  { id: 'gui', href: 'dev/gui-sessions', label: 'GUI', icon: 'M3 5h18v11H3zM8 20h8M12 16v4' },
  { id: 'actions', href: 'actions', label: 'Actions', icon: 'M4 6h16M4 12h10M4 18h6M18 14l3 3-3 3' },
  { id: 'audit', href: 'audit', label: 'Audit', icon: 'M5 4h14v16H5zM8 8h8M8 12h8M8 16h5' },
  { id: 'settings', href: 'settings', label: 'Settings', icon: 'M12 15a3 3 0 1 0 0-6 3 3 0 0 0 0 6zm7-3 2-1-2-4-2 1-2-1V4h-4v3l-2 1-2-1-2 4 2 1v2l-2 1 2 4 2-1 2 1v3h4v-3l2-1 2 1 2-4-2-1z' },
];

function activeId(page: string, sub?: string): string {
  if (page === 'task') return 'board';
  if (page === 'dev') {
    if (sub === 'guide' || sub === 'guides' || sub === 'guide-updates') return 'guides';
    if (sub === 'gui-sessions' || sub === 'gui-session') return 'gui';
    return sub ?? 'machines';
  }
  return page;
}

// App frame: sidebar on desktop, top bar + bottom tab bar on phones.
export function Shell({ page, sub, children }: { page: string; sub?: string; children: ReactNode }) {
  const { me } = useHub();
  const live = useLive();
  const active = activeId(page, sub);
  return (
    <div className="shell">
      <aside className="sidebar">
        <div className="brand"><span className="brand-mark" aria-hidden />agent-hub</div>
        <nav aria-label="Main" className="side-nav">
          {NAV.map((n) => (
            <Link key={n.id} href={`#/${n.href}`} className={`nav-item ${active === n.id ? 'is-active' : ''}`} aria-current={active === n.id ? 'page' : undefined}>
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
          <GrantsChip />
          <Badge tone={live.connected ? 'green' : 'amber'} title="Live event feed">{live.connected ? 'live' : 'offline'}</Badge>
          <Avatar name={me.principal.display_name} human={me.principal.is_human} />
        </div>
      </header>
      <main className="main" id="main">{children}</main>
      <nav aria-label="Main" className="tabbar">
        {NAV.map((n) => (
          <Link key={n.id} href={`#/${n.href}`} className={`tab-item ${active === n.id ? 'is-active' : ''}`} aria-current={active === n.id ? 'page' : undefined}>
            <svg viewBox="0 0 24 24" aria-hidden><path d={n.icon} /></svg><span>{n.label}</span>
          </Link>
        ))}
      </nav>
    </div>
  );
}
