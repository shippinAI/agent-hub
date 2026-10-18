-- Devagotchi catalog (docs/devagotchi/01-data-model.md). Every row is data seeded from presets and edited in the UI.
CREATE TABLE preset_marks (name text PRIMARY KEY, at timestamptz NOT NULL DEFAULT now());

CREATE TABLE tenants (
  id text PRIMARY KEY, name text NOT NULL, edition text NOT NULL DEFAULT 'personal' CHECK (edition IN ('personal','cloud')),
  plan text NOT NULL DEFAULT '', version bigint NOT NULL DEFAULT 1
);
INSERT INTO tenants(id, name) VALUES ('local', 'Local hub');

CREATE TABLE shell_profiles (
  id text PRIMARY KEY, tenant_id text NOT NULL DEFAULT 'local' REFERENCES tenants(id), os text NOT NULL, label text NOT NULL,
  argv_prefix jsonb NOT NULL DEFAULT '[]', env_allow text[] NOT NULL DEFAULT '{}', cwd_default text NOT NULL DEFAULT '',
  pty boolean NOT NULL DEFAULT false, version bigint NOT NULL DEFAULT 1
);

CREATE TABLE capabilities (
  id text PRIMARY KEY, tenant_id text NOT NULL DEFAULT 'local' REFERENCES tenants(id), label text NOT NULL,
  description text NOT NULL DEFAULT '', selector_schema jsonb NOT NULL DEFAULT '{}', sandbox_levels text[] NOT NULL DEFAULT '{}',
  default_duration text NOT NULL DEFAULT 'task', version bigint NOT NULL DEFAULT 1
);

CREATE TABLE risks (
  id text PRIMARY KEY, tenant_id text NOT NULL DEFAULT 'local' REFERENCES tenants(id), title text NOT NULL,
  plain_text text NOT NULL, severity text NOT NULL CHECK (severity IN ('low','med','high')), mitigation text NOT NULL DEFAULT '',
  source_url text, version bigint NOT NULL DEFAULT 1
);

CREATE TABLE capability_risks (
  id text PRIMARY KEY, tenant_id text NOT NULL DEFAULT 'local' REFERENCES tenants(id),
  capability_id text NOT NULL REFERENCES capabilities(id) ON DELETE CASCADE,
  risk_id text NOT NULL REFERENCES risks(id) ON DELETE CASCADE, when_selector jsonb NOT NULL DEFAULT '{}',
  version bigint NOT NULL DEFAULT 1
);

CREATE TABLE grant_presets (
  id text PRIMARY KEY, tenant_id text NOT NULL DEFAULT 'local' REFERENCES tenants(id), label text NOT NULL,
  description text NOT NULL DEFAULT '', items jsonb NOT NULL DEFAULT '[]', duration jsonb NOT NULL DEFAULT '"task"',
  full_access boolean NOT NULL DEFAULT false, requires_policy text, is_default boolean NOT NULL DEFAULT false,
  version bigint NOT NULL DEFAULT 1
);
CREATE UNIQUE INDEX grant_presets_one_default ON grant_presets (tenant_id) WHERE is_default;

CREATE TABLE docker_runtime_presets (
  id text PRIMARY KEY, tenant_id text NOT NULL DEFAULT 'local' REFERENCES tenants(id), os text NOT NULL, label text NOT NULL,
  what text NOT NULL DEFAULT '', facts jsonb NOT NULL DEFAULT '[]', operator_commands jsonb NOT NULL DEFAULT '{}',
  agent_argv jsonb NOT NULL DEFAULT '[]', verify_argv jsonb NOT NULL DEFAULT '[]', hosts text[] NOT NULL DEFAULT '{}',
  risk_ids text[] NOT NULL DEFAULT '{}', licence_confirm boolean NOT NULL DEFAULT false, version bigint NOT NULL DEFAULT 1
);

CREATE TABLE router_endpoints (
  id text PRIMARY KEY, tenant_id text NOT NULL DEFAULT 'local' REFERENCES tenants(id), label text NOT NULL, url text NOT NULL,
  kind text NOT NULL CHECK (kind IN ('litellm','openrouter','direct')), secret_id uuid, version bigint NOT NULL DEFAULT 1
);

CREATE TABLE providers (
  id text PRIMARY KEY, tenant_id text NOT NULL DEFAULT 'local' REFERENCES tenants(id),
  kind text NOT NULL CHECK (kind IN ('api','subscription_cli')), label text NOT NULL, models jsonb NOT NULL DEFAULT '[]',
  router_endpoint_id text REFERENCES router_endpoints(id), cli_name text, terms_url text NOT NULL DEFAULT '',
  terms_status text NOT NULL DEFAULT 'unverified' CHECK (terms_status IN ('verified','unverified')),
  terms_note text NOT NULL DEFAULT '', version bigint NOT NULL DEFAULT 1
);

CREATE TABLE chat_apps (
  id text PRIMARY KEY, tenant_id text NOT NULL DEFAULT 'local' REFERENCES tenants(id), label text NOT NULL,
  provider_ids text[] NOT NULL DEFAULT '{}', connect_template text NOT NULL DEFAULT '', version bigint NOT NULL DEFAULT 1
);

-- Write-only secrets (D5): ciphertext is AES-256-GCM under the hub key file; only fp (8 hex of SHA-256) is ever returned.
CREATE TABLE secrets (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(), tenant_id text NOT NULL DEFAULT 'local' REFERENCES tenants(id),
  owner text NOT NULL REFERENCES principals(id), label text NOT NULL, ciphertext bytea NOT NULL, fp text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(), rotated_at timestamptz, revoked_at timestamptz
);
ALTER TABLE router_endpoints ADD FOREIGN KEY (secret_id) REFERENCES secrets(id);
