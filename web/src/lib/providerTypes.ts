// Provider schema v1 (docs/devagotchi/34-provider-schema.md). Mirrors server/internal/providers; the enum
// lists below are checked against the Go ones by enums_ts_test.go, so keep each on one line.
export const ADAPTER_KINDS = ['acp', 'codex_app_server', 'headless_json', 'openai_compatible', 'anthropic_compatible'] as const;
export const API_ADAPTERS = ['openai_compatible', 'anthropic_compatible'] as const;
export const ROUTER_MODES = ['direct', 'litellm', 'openrouter', 'custom'] as const;
export const AUTH_KINDS = ['client_login', 'signin_oauth', 'api_key', 'plan_key'] as const;
export const RUNNERS = ['user_device', 'hosted_workspace', 'hub'] as const;
export const EDITIONS = ['local', 'hosted'] as const;
export const OSES = ['linux', 'macos', 'windows'] as const;
export const MODEL_SOURCES = ['adapter', 'command', 'http', 'static'] as const;
export const TOKEN_SOURCES = ['stream', 'otel', 'api_response', 'none'] as const;
export const QUOTA_SOURCES = ['adapter', 'statusline', 'http', 'observed', 'none'] as const;
export const COST_BASES = ['provider_charged', 'computed', 'client_estimate'] as const;
export const STATE_READS = ['adapter', 'command', 'http', 'file', 'live_only'] as const;
export const CAPABILITIES = ['streaming', 'tools', 'mcp_client', 'mcp_host', 'resume', 'fork', 'images', 'reasoning_effort', 'approvals'] as const;

export type AdapterKind = (typeof ADAPTER_KINDS)[number];
export type RouterMode = (typeof ROUTER_MODES)[number];
export type AuthKind = (typeof AUTH_KINDS)[number];
export type Runner = (typeof RUNNERS)[number];
export type Edition = (typeof EDITIONS)[number];
export type Os = (typeof OSES)[number];
export type Capability = (typeof CAPABILITIES)[number];

export type Router = { mode: RouterMode; endpoint_id?: string };
export type Adapter = { kind: AdapterKind; config: Record<string, unknown> & { router?: Router } };

export type DetectRule = {
  bins?: string[]; dirs?: string[]; app_ids?: string[];
  version_cmd?: string[]; version_regex?: string; signed_in_cmd?: string[];
};

export type Provider = {
  schema_version: 1; id: string; vendor: string; product: string; label: string; icon_ref: string; docs_url?: string;
  auth_kinds: AuthKind[]; runner: Runner[]; editions: Edition[];
  adapter: Adapter | null; fallback_adapters?: Adapter[]; host_for?: '' | 'harness';
  models: { source: (typeof MODEL_SOURCES)[number]; ref?: string; refresh_minutes: number };
  usage: { tokens: (typeof TOKEN_SOURCES)[number]; quota: (typeof QUOTA_SOURCES)[number]; quota_ref?: string; cost_basis: (typeof COST_BASES)[number] };
  state: { read: (typeof STATE_READS)[number]; ref?: string; paths?: Partial<Record<Os, string[]>>; parser?: string };
  detect: Partial<Record<Os, DetectRule>>;
  capabilities: Record<Capability, boolean>;
  terms_url?: string; terms_note?: string;
};

// A row as the API returns it: the schema fields plus server-kept metadata.
export type ProviderRow = Provider & { version: number; disabled: boolean; disabled_at: string | null };
export type CachedModel = { model: string; label: string; fetched_at: string };
export type ProviderDetail = ProviderRow & { model_cache: CachedModel[] };

// One validation failure (server 422 validation_failed → current.errors, or the client check below).
export type FieldError = { path: string; message: string };

export const isApiAdapter = (k: string) => (API_ADAPTERS as readonly string[]).includes(k);
