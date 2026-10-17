// Shapes returned by /api/v1 (see docs/spec.md).
export type Principal = {
  id: string; display_name: string; kind: string; is_human: boolean; login: string | null; roles: string[];
  enabled: boolean; wip_limit: number | null; last_seen_at: string | null; version: number; created_at: string;
};
export type Task = {
  id: number; project: string; title: string; description: string; state: string; priority: number; labels: string[];
  assignee: string | null; holder: string | null; lease_id: string | null; lease_ttl_seconds: number | null;
  lease_expires_at: string | null; heartbeat_at: string | null; fence: number; version: number; expiry_count: number;
  last_worker: string | null; progress: unknown; created_by: string; created_at: string; updated_at: string;
  ready: boolean; lease_expired: boolean; blocked_by: number[];
};
export type Note = { id: number; task_id: number; author: string; kind: string; body: string; to_principal: string | null; fence: number | null; created_at: string };
export type HubEvent = {
  seq: number; at: string; actor: string; action: string; task_id: number | null; data: Record<string, unknown>;
  fence: number | null; request_id: string | null; prev_hash: string; hash: string;
};
export type State = { key: string; label: string; kind: string };
export type Lane = { queue: string; active: string; complete_to: string; release_to: string; distinct_from_previous?: boolean };
export type Workflow = {
  id: string; name: string; states: State[]; transitions: { from: string; to: string }[]; lanes: Lane[];
  initial_state: string; attention_state: string; version: number;
};
export type Project = { id: string; name: string; workflow_id: string; version: number };
export type Role = { id: string; description: string; version: number };
export type Policy = {
  id: string; name: string; subject: string; action: string; selector: { projects?: string[]; labels?: string[] };
  effect: 'allow' | 'deny'; priority: number; enabled: boolean; version: number;
};
export type Setting = { key: string; value: unknown; description: string; version: number };
export type HubConfig = {
  workflows: Workflow[]; projects: Project[]; roles: Role[]; principals: Principal[]; settings: Setting[];
  policies: Policy[]; state_kinds: string[];
};
export type Me = { principal: Principal; auth: string; can: Record<string, boolean>; public_url: string };
export type TokenInfo = {
  id: string; principal_id: string; name: string; prefix: string; created_at: string;
  last_used_at: string | null; expires_at: string | null; revoked_at: string | null;
};
