import type { FieldError, Provider } from './providerTypes';
import { ADAPTER_KINDS, AUTH_KINDS, EDITIONS, ROUTER_MODES, RUNNERS, isApiAdapter } from './providerTypes';

// Quick client-side check so the form can flag obvious mistakes before a round trip. The hub's Go
// validator (server/internal/providers) is the authority; its errors are shown the same way.
const ID_RE = /^[a-z0-9]+(-[a-z0-9]+)+$/;

export function checkProvider(p: Provider): FieldError[] {
  const errs: FieldError[] = [];
  const add = (path: string, message: string) => errs.push({ path, message });
  if (!ID_RE.test(p.id)) add('id', 'must be vendor-product in lower case (e.g. anthropic-claude-code)');
  for (const k of ['vendor', 'product', 'label'] as const) if (!p[k]?.trim()) add(k, 'is required');
  if (!/^[a-z0-9][a-z0-9/_.-]*$/.test(p.icon_ref)) add('icon_ref', 'must be a lower-case icon path such as icons/xai');
  const sets: [keyof Provider, readonly string[]][] = [['auth_kinds', AUTH_KINDS], ['runner', RUNNERS], ['editions', EDITIONS]];
  for (const [k, allowed] of sets) {
    const v = p[k] as string[];
    if (!v?.length) add(k, 'needs at least one value');
    else v.forEach((x, i) => !allowed.includes(x) && add(`${k}[${i}]`, `unknown value ${x}`));
  }
  if (p.adapter) {
    if (!(ADAPTER_KINDS as readonly string[]).includes(p.adapter.kind)) add('adapter.kind', 'is not one of the five adapters');
    const cfg = p.adapter.config ?? {};
    if (!isApiAdapter(p.adapter.kind) && !(Array.isArray(cfg.argv) && cfg.argv.length)) add('adapter.config.argv', 'needs at least the program name');
    if (isApiAdapter(p.adapter.kind)) {
      const mode = cfg.router?.mode ?? 'direct';
      if (!(ROUTER_MODES as readonly string[]).includes(mode)) add('adapter.config.router.mode', `unknown router ${mode}`);
      if (mode === 'direct' && !cfg.base_url) add('adapter.config.base_url', 'is required when router.mode is direct');
      if (mode !== 'direct' && !cfg.router?.endpoint_id) add('adapter.config.router.endpoint_id', `is required when router.mode is ${mode}`);
    }
  } else if (!p.capabilities?.mcp_host) add('adapter', 'is required unless the row is an MCP host only');
  if (p.runner?.includes('hub') && !(p.adapter && isApiAdapter(p.adapter.kind))) add('runner', 'hub is only for API adapters');
  if ((p.models.source === 'command' || p.models.source === 'http') && !p.models.ref) add('models.ref', `is required when models.source is ${p.models.source}`);
  if (p.models.refresh_minutes < 1 || p.models.refresh_minutes > 10080) add('models.refresh_minutes', 'must be between 1 and 10080');
  if ((p.state.read === 'command' || p.state.read === 'http') && !p.state.ref) add('state.ref', `is required when state.read is ${p.state.read}`);
  return errs;
}
