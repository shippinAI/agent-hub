import type { ReactNode } from 'react';

type Tone = 'neutral' | 'blue' | 'green' | 'amber' | 'red' | 'violet' | 'outline';

export function Badge({ tone = 'neutral', children, title }: { tone?: Tone; children: ReactNode; title?: string }) {
  return <span className={`badge badge-${tone}`} title={title}>{children}</span>;
}

// Colour per workflow state kind (kinds are the fixed vocabulary; state names are data).
export const kindTone: Record<string, Tone> = {
  queue: 'neutral', active: 'blue', review: 'violet', blocked: 'amber', attention: 'red', done: 'green',
};

// Initials plus a stable hue per name, so two agents with the same initials still look different.
export function Avatar({ name, human }: { name: string; human?: boolean }) {
  const words = name.split(/[\s_-]+/).filter(Boolean);
  const initials = (words.length > 1 ? words[0][0] + words[1][0] : name.slice(0, 2)).toUpperCase();
  let h = 0;
  for (const c of name) h = (h * 31 + c.charCodeAt(0)) % 360;
  const style = human ? undefined : { color: `hsl(${h} 65% 42%)`, background: `hsl(${h} 80% 94%)` };
  return <span className={`avatar ${human ? 'avatar-human' : ''}`} style={style} aria-hidden>{initials}</span>;
}
