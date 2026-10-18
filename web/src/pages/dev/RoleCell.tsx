import type { Binding, Sub } from '../../lib/dtypes';
import { Button } from '../../ui/Button';
import { Select } from '../../ui/Select';

// One cell of the role-bindings matrix: the subscriptions bound to a role on one chat app, ordered by
// priority with up/down, each a Select, plus a fallback Select, plus a control to add another.
export function RoleCell({ role, app, bindings, subs, onPut, onDel }: {
  role: string; app: string; bindings: Binding[]; subs: Sub[];
  onPut: (b: Partial<Binding>) => void; onDel: (id: string) => void;
}) {
  const subOpts = subs.filter((s) => s.kind === app || true).map((s) => ({ id: s.id, label: s.label }));
  const fbOpts = [{ id: '', label: 'No fallback' }, ...subOpts];
  const move = (b: Binding, d: number) => onPut({ id: b.id, role_id: role, chat_app: app, subscription_id: b.subscription_id, priority: b.priority + d, fallback_subscription_id: b.fallback_subscription_id, enabled: b.enabled, version: b.version });
  return (
    <div className="role-cell">
      {bindings.map((b) => (
        <div key={b.id} className="role-bind">
          <div className="row gap-1">
            <div className="role-prio">
              <button className="prio-btn" aria-label="Higher priority" onClick={() => move(b, -1)}>↑</button>
              <span className="hint">{b.priority}</span>
              <button className="prio-btn" aria-label="Lower priority" onClick={() => move(b, 1)}>↓</button>
            </div>
            <Select label="Subscription" hideLabel options={subOpts} value={b.subscription_id}
              onChange={(v) => onPut({ ...b, subscription_id: v })} className="grow" />
            <Button size="sm" variant="ghost" onPress={() => onDel(b.id)}>✕</Button>
          </div>
          <Select label="Fallback" hideLabel options={fbOpts} value={b.fallback_subscription_id ?? ''}
            onChange={(v) => onPut({ ...b, fallback_subscription_id: v || null })} className="role-fallback" />
        </div>
      ))}
      {subOpts.length > 0 && (
        <Select label={`Add to ${app}`} hideLabel options={[{ id: '', label: '+ Add subscription' }, ...subOpts]} value=""
          onChange={(v) => v && onPut({ role_id: role, chat_app: app, subscription_id: v, priority: bindings.length, enabled: true, version: 0 })} />
      )}
    </div>
  );
}
