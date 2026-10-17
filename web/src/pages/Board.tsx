import { useMemo, useState } from 'react';
import { GridList, GridListItem, ToggleButton, ToggleButtonGroup } from 'react-aria-components';
import { get } from '../lib/api';
import { useHub, workflowFor } from '../lib/hub';
import { go } from '../lib/route';
import { setServerNow } from '../lib/time';
import { useData } from '../lib/useData';
import type { Task } from '../lib/types';
import { Badge, kindTone } from '../ui/Badge';
import { SearchField } from '../ui/Field';
import { Empty, Notice } from '../ui/Notice';
import { Select } from '../ui/Select';
import { NewTask } from './NewTask';
import { TaskCard } from './TaskCard';

// Board: workflow columns on wide screens; state chips + one list on phones.
export function Board({ project: routeProject, state }: { project?: string; state?: string }) {
  const { config, me } = useHub();
  const project = routeProject ?? config.projects[0]?.id ?? 'general';
  const wf = workflowFor(config, project);
  const [q, setQ] = useState('');
  const [mine, setMine] = useState(false);
  const tasks = useData(async () => {
    const r = await get<{ tasks: Task[]; server_now: string }>(`/tasks?project=${encodeURIComponent(project)}&q=${encodeURIComponent(q)}${mine ? '&holder=me' : ''}`);
    setServerNow(r.server_now);
    return r.tasks;
  }, [project, q, mine]);
  const byState = useMemo(() => {
    const m: Record<string, Task[]> = {};
    for (const t of tasks.data ?? []) (m[t.state] ??= []).push(t);
    return m;
  }, [tasks.data]);
  const states = wf?.states ?? [];
  const sel = state ?? states.find((s) => (byState[s.key]?.length ?? 0) > 0 && s.kind !== 'done')?.key ?? states[0]?.key;
  const held = (tasks.data ?? []).filter((t) => t.holder).length;
  return (
    <div className="page">
      <div className="page-head">
        <div>
          <h1 className="page-title">Board</h1>
          <p className="muted">{tasks.data?.length ?? 0} tasks · {held} leased · workflow <b>{wf?.name}</b></p>
        </div>
        {me.can['task.create'] && <NewTask project={project} />}
      </div>
      <div className="toolbar">
        {config.projects.length > 1 && <Select label="Project" hideLabel value={project} onChange={(p) => go(`board/${p}`)}
          options={config.projects.map((p) => ({ id: p.id, label: p.name }))} className="w-auto" />}
        <SearchField label="Search tasks" value={q} onChange={setQ} placeholder="Search titles, notes, #id" />
        <ToggleButton isSelected={mine} onChange={setMine} className="btn btn-outline btn-md toggle">Held by me</ToggleButton>
      </div>
      {tasks.error && <Notice tone="error">{tasks.error.message}</Notice>}
      <ToggleButtonGroup aria-label="State" selectionMode="single" disallowEmptySelection className="state-chips"
        selectedKeys={sel ? [sel] : []} onSelectionChange={(k) => { const v = [...k][0]; if (v) go(`board/${project}/${v}`); }}>
        {states.map((s) => (
          <ToggleButton key={s.key} id={s.key} className="chip">
            {s.label}<span className="chip-count">{byState[s.key]?.length ?? 0}</span>
          </ToggleButton>
        ))}
      </ToggleButtonGroup>
      <div className="columns">
        {states.map((s) => {
          const list = byState[s.key] ?? [];
          if (s.kind === 'done' && list.length === 0) return null;
          return (
            <section key={s.key} className={`column ${s.key === sel ? 'is-selected' : ''}`} aria-label={s.label}>
              <header className="column-head"><Badge tone={kindTone[s.kind]}>{s.label}</Badge><span className="hint">{list.length}</span></header>
              <GridList aria-label={`${s.label} tasks`} className="card-list" items={list.slice(0, s.kind === 'done' ? 25 : 200)}
                onAction={(id) => go(`task/${id}`)} renderEmptyState={() => <Empty title="Nothing here" />}>
                {(t) => <GridListItem id={t.id} textValue={t.title} className="card-item"><TaskCard task={t} /></GridListItem>}
              </GridList>
            </section>
          );
        })}
      </div>
    </div>
  );
}
