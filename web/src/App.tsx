import { get, HttpError } from './lib/api';
import { HubProvider } from './lib/hub';
import { LiveProvider } from './lib/live';
import { useRoute } from './lib/route';
import { useData } from './lib/useData';
import type { Me } from './lib/types';
import { Shell } from './pages/Shell';
import { Login } from './pages/Login';
import { Board } from './pages/Board';
import { TaskDetail } from './pages/TaskDetail';
import { Agents } from './pages/Agents';
import { Settings } from './pages/Settings';
import { Audit } from './pages/Audit';
import { Machines } from './pages/dev/Machines';
import { MachineDetail } from './pages/dev/MachineDetail';
import { Requests } from './pages/dev/Requests';
import { Grants } from './pages/dev/Grants';
import { Subscriptions } from './pages/dev/Subscriptions';
import { Guides } from './pages/dev/Guides';
import { GuideEditor } from './pages/dev/GuideEditor';
import { GuideUpdates } from './pages/dev/GuideUpdates';
import { GuiSessions } from './pages/dev/GuiSessions';
import { GuiSession } from './pages/dev/GuiSession';
import { RoleBindings } from './pages/dev/RoleBindings';
import { ResumeAlert } from './pages/dev/ResumeAlert';
import { Policy } from './pages/gate/Policy';
import { Changes } from './pages/gate/Changes';

function devPage(a: string | undefined, b: string | undefined) {
  switch (a) {
    case 'machine': return <MachineDetail id={String(b)} />;
    case 'requests': return <Requests />;
    case 'grants': return <Grants />;
    case 'subscriptions': return <Subscriptions />;
    case 'guides': return <Guides />;
    case 'guide': return <GuideEditor id={String(b)} />;
    case 'guide-updates': return <GuideUpdates />;
    case 'gui-sessions': return <GuiSessions />;
    case 'gui-session': return <GuiSession id={String(b)} />;
    case 'role-bindings': return <RoleBindings />;
    case 'policy': return <Policy tab={b ?? 'rules'} />;
    case 'changes': return <Changes id={b} />;
    default: return <Machines />;
  }
}

export function App() {
  const me = useData(() => get<Me>('/me'), [], false);
  if (me.error instanceof HttpError && me.error.status === 401) return <Login message={me.error.message} />;
  if (!me.data) return <div className="page-loading" role="status">{me.error ? me.error.message : 'Connecting…'}</div>;
  return (
    <LiveProvider>
      <HubProvider me={me.data}>
        <Routes />
      </HubProvider>
    </LiveProvider>
  );
}

function Routes() {
  const [page, a, b] = useRoute();
  const body = (() => {
    switch (page) {
      case 'task': return <TaskDetail id={Number(a)} />;
      case 'agents': return <Agents />;
      case 'settings': return <Settings tab={a ?? 'policies'} />;
      case 'audit': return <Audit />;
      case 'dev': return devPage(a, b);
      default: return <Board project={a} state={b} />;
    }
  })();
  return <Shell page={page} sub={page === 'dev' ? (a ?? 'machines') : undefined}><ResumeAlert />{body}</Shell>;
}
