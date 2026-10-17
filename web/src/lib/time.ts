import { useEffect, useState } from 'react';

// Ticks every second so countdowns stay live; offset corrects for client/server clock skew.
export function useNow(intervalMs = 1000) {
  const [now, setNow] = useState(Date.now());
  useEffect(() => { const t = setInterval(() => setNow(Date.now()), intervalMs); return () => clearInterval(t); }, [intervalMs]);
  return now;
}

let skew = 0;
export const setServerNow = (iso?: string) => { if (iso) skew = new Date(iso).getTime() - Date.now(); };
export const serverNow = (now: number) => now + skew;

export function countdown(untilIso: string, now: number): { text: string; secs: number } {
  const secs = Math.round((new Date(untilIso).getTime() - serverNow(now)) / 1000);
  if (secs <= 0) return { text: 'expired', secs };
  const h = Math.floor(secs / 3600), m = Math.floor((secs % 3600) / 60), s = secs % 60;
  return { text: h ? `${h}h ${m}m` : m ? `${m}m ${String(s).padStart(2, '0')}s` : `${s}s`, secs };
}

export function ago(iso: string | null, now = Date.now()): string {
  if (!iso) return 'never';
  const s = Math.max(0, Math.round((serverNow(now) - new Date(iso).getTime()) / 1000));
  if (s < 60) return `${s}s ago`;
  if (s < 3600) return `${Math.floor(s / 60)}m ago`;
  if (s < 86400) return `${Math.floor(s / 3600)}h ago`;
  return `${Math.floor(s / 86400)}d ago`;
}

export const clock = (iso: string) => new Date(iso).toLocaleString([], { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit', second: '2-digit' });
