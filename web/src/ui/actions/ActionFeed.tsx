import type { ActionItem } from '../../lib/actionTypes';
import { Empty } from '../Notice';
import { ActionCard } from './ActionCard';

// The feed: newest first, one card per call. Rendering is driven only by the typed events.
export function ActionFeed({ items, emptyText = 'No agent actions yet' }: { items: ActionItem[]; emptyText?: string }) {
  if (!items.length) return <Empty title={emptyText}>Tool calls, commands, edits and policy decisions show up here as they happen.</Empty>;
  return (
    <ol className="action-feed" aria-label="Agent actions">
      {items.map((it) => <li key={it.key}><ActionCard item={it} /></li>)}
    </ol>
  );
}
