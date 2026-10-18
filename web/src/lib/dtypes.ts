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

export type Guide = {
  id: string; slug: string; title: string; goal: string; tags: string[]; source: string;
  overrides_guide_id: string | null; channel: string; enabled: boolean; version: number;
  current_version_id: string | null; semver: string | null; steps?: unknown[]; risk_ids: string[];
  pending_update: boolean; applies_to: Record<string, unknown>;
};

export type GuideVersion = {
  id: string; semver: string; status: string; change_notes: string; authored_by: string;
  content_sha256: string; created_at: string; steps: unknown[];
};

export type GuideStep = {
  id: string; kind: string; actor?: string; title: string; text?: string; command?: string;
  changes?: string; check?: string; question?: string; reason?: string; options?: { label: string; goto?: string }[];
  risk_ids?: string[];
};

export type GuideRunView = {
  run: { id: string; guide_id: string; semver: string; status: string; current_step: string | null;
    machine_id: string | null; dry_run: boolean; started_at: string; agent_principal: string | null; };
  outline: { id: string; title: string; kind: string }[];
  step?: GuideStep;
};

export type GuideUpdate = {
  id: string; guide_id: string; from_semver: string | null; to_semver: string; change_notes: string;
  status: string; created_at: string;
};

export type GuiSession = {
  id: string; machine_id: string; holder: string; state: string; fence: number;
  lease_expires_at: string; started_at: string; ended_at: string | null; display_server: string | null;
  last_artifact_id: string | null;
};

export type GuiArtifact = {
  id: string; kind: string; sha256: string; width: number; height: number; bytes: number;
  redacted: boolean; has_image: boolean; created_at: string;
};

export type Binding = {
  id: string; role_id: string; chat_app: string; subscription_id: string; priority: number;
  fallback_subscription_id: string | null; enabled: boolean; version: number;
};

export type ChatApp = { id: string; label: string; provider_ids: string[]; connect_template: string };
export type Diff = { step: string; change: string; before?: unknown; after?: unknown };
