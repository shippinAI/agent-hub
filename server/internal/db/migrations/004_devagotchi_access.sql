-- Devagotchi machines, grants and live sessions (docs/devagotchi/01, 03, 04, 11).
CREATE TABLE machines (
  id text PRIMARY KEY CHECK (id ~ '^[a-z0-9][a-z0-9-]{0,40}$'), tenant_id text NOT NULL DEFAULT 'local' REFERENCES tenants(id),
  principal_id text NOT NULL UNIQUE REFERENCES principals(id), label text NOT NULL, os text NOT NULL,
  kind text NOT NULL DEFAULT 'native', host_machine_id text REFERENCES machines(id), arch text NOT NULL DEFAULT '',
  bridge_version text NOT NULL DEFAULT '', sandbox_kind text NOT NULL DEFAULT 'unknown', display_server text,
  gui_ready jsonb NOT NULL DEFAULT '{}', shell_profile_ids text[] NOT NULL DEFAULT '{}', enrolled_by text NOT NULL,
  enrolled_at timestamptz, sign_key_ct bytea, last_exec_ok_at timestamptz, last_exec_err_at timestamptz, last_error text,
  last_poll_at timestamptz, connected boolean NOT NULL DEFAULT false, disabled boolean NOT NULL DEFAULT false,
  gui_paused boolean NOT NULL DEFAULT false, created_at timestamptz NOT NULL DEFAULT now(), version bigint NOT NULL DEFAULT 1
);

-- One-time enrollment codes: "<pub>-<secret>"; only the secret's SHA-256 is stored; 5 tries, 10 minutes, single use.
CREATE TABLE machine_enroll_codes (
  pub text PRIMARY KEY, machine_id text NOT NULL REFERENCES machines(id) ON DELETE CASCADE, hash bytea NOT NULL,
  tries int NOT NULL DEFAULT 0, expires_at timestamptz NOT NULL, used_at timestamptz, created_by text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE sandbox_policies (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(), tenant_id text NOT NULL DEFAULT 'local' REFERENCES tenants(id),
  machine_id text REFERENCES machines(id) ON DELETE CASCADE, unsandboxed_allowed boolean NOT NULL DEFAULT false,
  enabled_by text NOT NULL, enabled_at timestamptz NOT NULL DEFAULT now(), acknowledged_risk_ids text[] NOT NULL DEFAULT '{}',
  expires_at timestamptz, version bigint NOT NULL DEFAULT 1
);
CREATE UNIQUE INDEX sandbox_policies_scope ON sandbox_policies (tenant_id, coalesce(machine_id, ''));

CREATE TABLE access_requests (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(), group_id uuid NOT NULL, tenant_id text NOT NULL DEFAULT 'local' REFERENCES tenants(id),
  requester text NOT NULL REFERENCES principals(id), task_id bigint REFERENCES tasks(id), guide_run_id uuid,
  machine_id text NOT NULL REFERENCES machines(id) ON DELETE CASCADE, capability text NOT NULL REFERENCES capabilities(id),
  selector jsonb NOT NULL DEFAULT '{}', sandbox_level text NOT NULL, duration jsonb NOT NULL, uses_max int,
  preset_id text, reason text NOT NULL CHECK (length(reason) >= 20), risk_snapshot jsonb NOT NULL DEFAULT '[]',
  collapsed boolean NOT NULL DEFAULT false,
  status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','approved','narrowed','denied','cancelled','expired')),
  decided_by text, decided_at timestamptz, decision_note text, grant_id uuid,
  created_at timestamptz NOT NULL DEFAULT now(), expires_at timestamptz NOT NULL
);
CREATE INDEX access_requests_pending ON access_requests (status, created_at);

CREATE TABLE grants (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(), tenant_id text NOT NULL DEFAULT 'local' REFERENCES tenants(id),
  principal_id text NOT NULL REFERENCES principals(id), machine_id text NOT NULL REFERENCES machines(id) ON DELETE CASCADE,
  capability text NOT NULL, selector jsonb NOT NULL DEFAULT '{}', sandbox_level text NOT NULL, preset_id text,
  full_access boolean NOT NULL DEFAULT false, duration jsonb NOT NULL, task_id bigint REFERENCES tasks(id), task_lease_id uuid,
  guide_run_id uuid, expires_at timestamptz, uses_max int, uses int NOT NULL DEFAULT 0, approved_by text NOT NULL,
  approved_at timestamptz NOT NULL DEFAULT now(), risk_snapshot jsonb NOT NULL DEFAULT '[]',
  request_id uuid REFERENCES access_requests(id), paused boolean NOT NULL DEFAULT false,
  revoked_at timestamptz, revoked_by text, revoke_reason text, ended_at timestamptz, end_reason text,
  version bigint NOT NULL DEFAULT 1
);
CREATE INDEX grants_live ON grants (machine_id, principal_id) WHERE ended_at IS NULL;

CREATE TABLE resume_acks (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(), grant_id uuid NOT NULL REFERENCES grants(id) ON DELETE CASCADE,
  principal_id text NOT NULL REFERENCES principals(id), ui_session_id text, reason text NOT NULL,
  shown_at timestamptz NOT NULL DEFAULT now(), acknowledged_at timestamptz,
  action text CHECK (action IN ('continue','revoke','pause'))
);

CREATE TABLE proc_leases (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(), machine_id text NOT NULL REFERENCES machines(id) ON DELETE CASCADE,
  grant_id uuid NOT NULL REFERENCES grants(id), holder text NOT NULL REFERENCES principals(id), task_id bigint,
  argv_fp text NOT NULL, pid_hint int, fence bigint NOT NULL, ttl_seconds int NOT NULL, lease_expires_at timestamptz NOT NULL,
  heartbeat_at timestamptz NOT NULL DEFAULT now(),
  state text NOT NULL DEFAULT 'running' CHECK (state IN ('running','exited','killed','lost')),
  exit_code int, started_at timestamptz NOT NULL DEFAULT now(), ended_at timestamptz
);

CREATE TABLE gui_sessions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(), machine_id text NOT NULL REFERENCES machines(id) ON DELETE CASCADE,
  grant_ids uuid[] NOT NULL, holder text NOT NULL REFERENCES principals(id), task_id bigint, display_server text,
  monitors jsonb NOT NULL DEFAULT '[]', fence bigint NOT NULL, ttl_seconds int NOT NULL, lease_expires_at timestamptz NOT NULL,
  state text NOT NULL DEFAULT 'starting' CHECK (state IN ('starting','active','paused','ended')),
  started_at timestamptz NOT NULL DEFAULT now(), ended_at timestamptz, end_reason text, last_artifact_id uuid
);
CREATE UNIQUE INDEX gui_sessions_one_live ON gui_sessions (machine_id) WHERE state <> 'ended';

CREATE TABLE gui_artifacts (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(), gui_session_id uuid NOT NULL REFERENCES gui_sessions(id) ON DELETE CASCADE,
  kind text NOT NULL CHECK (kind IN ('screenshot','thumbnail')), sha256 text NOT NULL, width int NOT NULL, height int NOT NULL,
  bytes int NOT NULL DEFAULT 0, redacted boolean NOT NULL DEFAULT false, blob_ct bytea, expires_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now()
);
