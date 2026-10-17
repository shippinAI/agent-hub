// Thin REST client. A pasted token (token login) lives in sessionStorage only.
const TOKEN_KEY = 'agent-hub-token';

export class HttpError extends Error {
  constructor(public status: number, public code: string, msg: string, public current?: unknown) { super(msg); }
}

export const token = {
  get: () => sessionStorage.getItem(TOKEN_KEY),
  set: (t: string) => sessionStorage.setItem(TOKEN_KEY, t.trim()),
  clear: () => sessionStorage.removeItem(TOKEN_KEY),
};

export function authHeaders(): Record<string, string> {
  const h: Record<string, string> = { 'X-Hub-Client': 'ui' };
  const t = token.get();
  if (t) h.Authorization = `Bearer ${t}`;
  return h;
}

export async function call<T>(method: string, path: string, body?: unknown, extra?: Record<string, string>): Promise<T> {
  const headers: Record<string, string> = { ...authHeaders(), 'Content-Type': 'application/json', ...extra };
  if (method !== 'GET' && !headers['Idempotency-Key']) headers['Idempotency-Key'] = `ui-${crypto.randomUUID()}`;
  const r = await fetch(`/api/v1${path}`, { method, headers, body: body === undefined ? undefined : JSON.stringify(body) });
  const data = await r.json().catch(() => ({}));
  if (!r.ok) {
    const e = data.error ?? {};
    throw new HttpError(r.status, e.code ?? 'error', e.message ?? r.statusText, e.current);
  }
  return data as T;
}

export const get = <T,>(p: string) => call<T>('GET', p);
export const post = <T,>(p: string, b?: unknown) => call<T>('POST', p, b ?? {});
export const put = <T,>(p: string, b: unknown) => call<T>('PUT', p, b);
export const patch = <T,>(p: string, b: unknown) => call<T>('PATCH', p, b);
export const del = <T,>(p: string) => call<T>('DELETE', p);

export function errText(e: unknown): string {
  if (e instanceof HttpError) return `${e.code}: ${e.message}`;
  return e instanceof Error ? e.message : String(e);
}
