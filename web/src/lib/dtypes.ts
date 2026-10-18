// Devagotchi API shapes used by the UI.
export type Risk = { id: string; title: string; plain_text: string; severity: string; mitigation?: string };

export type Machine = {
  id: string; label: string; os: string; kind: string; sandbox_kind: string;
  display_server: string | null; connected: boolean; online: boolean; disabled: boolean;
  gui_paused: boolean; live_grants: number; high_risk_grants: number; last_error: string | null; gui_ready: Record<string, string>;
  last_exec_ok_at: string | null; bridge_version: string; arch: string; version: number;
};

export type Grant = {
  id: string; machine_id: string; machine_label: string; principal_id: string; capability: string;
  sandbox_level: string; selector: Record<string, unknown>; duration: string; expires_at: string | null;
  approved_at: string; approved_by: string; live: boolean; paused: boolean; full_access: boolean;
  high_risk: boolean; risk_snapshot: Risk[] | null; ended_at: string | null; revoke_reason: string | null;
};

export type Request = {
  id: string; group_id: string; machine_id: string; machine_label: string; requester: string;
  requester_name: string; capability: string; sandbox_level: string; selector: Record<string, unknown>;
  duration: string; reason: string; status: string; created_at: string; expires_at: string | null;
  risk_snapshot: Risk[] | null; grant_id: string | null; uses_max: number | null;
};

export type Sub = {
  id: string; label: string; provider_id: string; kind: string; machine_id: string | null;
  enabled: boolean; secret_fp: string | null; last_test_ok_at: string | null; last_test_error: string | null;
  version: number;
};

export type Summary = {
  live_grants: number; high_risk: number; pending_requests: number; gui_active: number; unsandboxed_allowed: boolean;
};

export type Alert = {
  id: string; grant_id: string; reason: string; machine_label: string; principal_id: string;
  scope: string; sandbox_level: string; full_access: boolean; risks: Risk[] | null;
};
