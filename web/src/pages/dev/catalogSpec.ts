// Column specs for the Catalog editors. Kinds and their columns mirror internal/catalog/kinds.go; the
// data itself is seeded presets, nothing hardcoded in the UI beyond which fields a row carries.
// Providers have their own schema and editor (pages/dev/providers, doc 34), not a generic spec here.
export type FieldType = 'text' | 'area' | 'json' | 'bool' | 'list';
export type Field = { key: string; type: FieldType; label?: string };
export type Spec = { kind: string; label: string; title: string; fields: Field[] };

const f = (key: string, type: FieldType = 'text'): Field => ({ key, type });

export const SPECS: Spec[] = [
  { kind: 'capabilities', label: 'Capabilities', title: 'label', fields: [
    f('id'), f('label'), f('description', 'area'), f('selector_schema', 'json'), f('sandbox_levels', 'list'), f('default_duration')] },
  { kind: 'risks', label: 'Risks', title: 'title', fields: [
    f('id'), f('title'), f('plain_text', 'area'), f('severity'), f('mitigation', 'area'), f('source_url')] },
  { kind: 'capability-risks', label: 'Capability risks', title: 'id', fields: [
    f('id'), f('capability_id'), f('risk_id'), f('when_selector', 'json')] },
  { kind: 'shell-profiles', label: 'Shell profiles', title: 'label', fields: [
    f('id'), f('os'), f('label'), f('argv_prefix', 'json'), f('env_allow', 'list'), f('cwd_default'), f('pty', 'bool')] },
  { kind: 'router-endpoints', label: 'Router endpoints', title: 'label', fields: [
    f('id'), f('label'), f('url'), f('kind')] },
  { kind: 'chat-apps', label: 'Chat apps', title: 'label', fields: [
    f('id'), f('label'), f('provider_ids', 'list'), f('connect_template', 'area')] },
];
