import type { ReactNode } from 'react';

export function Notice({ tone = 'info', children }: { tone?: 'info' | 'error' | 'success' | 'warn'; children: ReactNode }) {
  return <div className={`notice notice-${tone}`} role={tone === 'error' ? 'alert' : 'status'}>{children}</div>;
}

export function Empty({ title, children }: { title: string; children?: ReactNode }) {
  return <div className="empty"><p className="empty-title">{title}</p>{children && <p className="muted">{children}</p>}</div>;
}

export function Card({ title, actions, children, className = '' }: { title?: ReactNode; actions?: ReactNode; children: ReactNode; className?: string }) {
  return (
    <section className={`card ${className}`}>
      {(title || actions) && <header className="card-head"><h2 className="card-title">{title}</h2><div className="row gap-2">{actions}</div></header>}
      {children}
    </section>
  );
}

export function Kv({ k, children }: { k: string; children: ReactNode }) {
  return <div className="kv"><dt>{k}</dt><dd>{children}</dd></div>;
}
