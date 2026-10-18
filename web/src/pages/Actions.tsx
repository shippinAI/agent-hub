import { useState } from 'react';
import { useHub } from '../lib/hub';
import { useActionFeed } from '../lib/useActionFeed';
import { useData } from '../lib/useData';
import { get } from '../lib/api';
import type { RetentionView } from '../lib/actionTypes';
import { Badge } from '../ui/Badge';
import { Notice } from '../ui/Notice';
import { Select } from '../ui/Select';
import { ActionFeed } from '../ui/actions/ActionFeed';

// Live action transparency: every tool call, command, script and edit an agent makes, with its policy decision.
export function Actions() {
  const { config } = useHub();
  const [category, setCategory] = useState('');
  const [actor, setActor] = useState('');
  const feed = useActionFeed({ category, actor });
  const cats = useData(() => get<RetentionView>('/audit/retention').then((r) => r.categories), [], false);
  return (
    <div className="page">
      <div className="page-head">
        <div>
          <h1 className="page-title">Actions</h1>
          <p className="muted">What agents ran: argv, cwd, diffs, exit codes, output and who approved it. Secrets are masked before storage.</p>
        </div>
        <Badge tone={feed.live ? 'green' : 'amber'}>{feed.live ? 'live' : 'reconnecting'}</Badge>
      </div>
      <div className="toolbar">
        <Select label="Category" hideLabel value={category || '*'} onChange={(v) => setCategory(v === '*' ? '' : v)} className="w-auto"
          options={[{ id: '*', label: 'All categories' }, ...(cats.data ?? []).map((c) => ({ id: c.category, label: c.category, hint: c.enabled ? undefined : 'recording off' }))]} />
        <Select label="Actor" hideLabel value={actor || '*'} onChange={(v) => setActor(v === '*' ? '' : v)} className="w-auto"
          options={[{ id: '*', label: 'All actors' }, ...config.principals.map((p) => ({ id: p.id, label: p.display_name }))]} />
      </div>
      {feed.error && <Notice tone="error">{feed.error.message}</Notice>}
      <ActionFeed items={feed.items} />
    </div>
  );
}
