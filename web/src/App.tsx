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
      default: return <Board project={a} state={b} />;
    }
  })();
  return <Shell page={page}>{body}</Shell>;
}
