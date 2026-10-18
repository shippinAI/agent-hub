-- Devagotchi guides (docs/devagotchi/12) and subscriptions / role bindings (05).
CREATE TABLE guides (
  id text PRIMARY KEY, tenant_id text REFERENCES tenants(id), slug text NOT NULL, title text NOT NULL, goal text NOT NULL DEFAULT '',
  tags text[] NOT NULL DEFAULT '{}', applies_to jsonb NOT NULL DEFAULT '{}',
  source text NOT NULL DEFAULT 'tenant' CHECK (source IN ('shippin','tenant')), overrides_guide_id text REFERENCES guides(id),
  current_version_id uuid, channel text NOT NULL DEFAULT 'stable' CHECK (channel IN ('stable','beta')),
  enabled boolean NOT NULL DEFAULT true, version bigint NOT NULL DEFAULT 1
);

CREATE TABLE guide_versions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(), guide_id text NOT NULL REFERENCES guides(id) ON DELETE CASCADE,
  semver text NOT NULL, schema_version int NOT NULL DEFAULT 1, steps jsonb NOT NULL, change_notes text NOT NULL DEFAULT '',
  risk_ids text[] NOT NULL DEFAULT '{}', authored_by text NOT NULL, signed_by text, signature text, content_sha256 text NOT NULL,
  status text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','published','retired')),
  created_at timestamptz NOT NULL DEFAULT now(), published_at timestamptz, UNIQUE (guide_id, semver)
);
ALTER TABLE guides ADD FOREIGN KEY (current_version_id) REFERENCES guide_versions(id);

-- Published versions are immutable (01/12): only status may move published -> retired.
CREATE FUNCTION guide_versions_immutable() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF OLD.status <> 'draft' AND (NEW.steps IS DISTINCT FROM OLD.steps OR NEW.semver IS DISTINCT FROM OLD.semver
      OR NEW.content_sha256 IS DISTINCT FROM OLD.content_sha256 OR NEW.change_notes IS DISTINCT FROM OLD.change_notes) THEN
    RAISE EXCEPTION 'guide version % is % and immutable', OLD.id, OLD.status;
  END IF;
  IF OLD.status = 'retired' AND NEW.status <> 'retired' THEN RAISE EXCEPTION 'retired guide versions stay retired'; END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER guide_versions_immutable BEFORE UPDATE ON guide_versions FOR EACH ROW EXECUTE FUNCTION guide_versions_immutable();

CREATE TABLE guide_runs (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(), guide_version_id uuid NOT NULL REFERENCES guide_versions(id),
  parent_run_id uuid REFERENCES guide_runs(id), machine_id text REFERENCES machines(id) ON DELETE SET NULL, task_id bigint,
  started_by text NOT NULL, agent_principal text, vars jsonb NOT NULL DEFAULT '{}', step_log jsonb NOT NULL DEFAULT '[]',
  current_step text, retries jsonb NOT NULL DEFAULT '{}', dry_run boolean NOT NULL DEFAULT false,
  status text NOT NULL DEFAULT 'running' CHECK (status IN ('running','paused','done','failed','abandoned')),
  started_at timestamptz NOT NULL DEFAULT now(), ended_at timestamptz
);

CREATE TABLE guide_updates (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(), guide_id text NOT NULL, from_semver text, to_semver text NOT NULL,
  bundle_sha256 text NOT NULL, change_notes text NOT NULL DEFAULT '', payload jsonb NOT NULL,
  status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','accepted','skipped','merged')),
  decided_by text, decided_at timestamptz, created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE subscriptions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(), tenant_id text NOT NULL DEFAULT 'local' REFERENCES tenants(id),
  owner text NOT NULL REFERENCES principals(id), provider_id text NOT NULL REFERENCES providers(id), label text NOT NULL,
  kind text NOT NULL CHECK (kind IN ('api_key','cli_login')), secret_id uuid REFERENCES secrets(id),
  worker_principal_id text REFERENCES principals(id), machine_id text REFERENCES machines(id) ON DELETE SET NULL,
  budget jsonb, spend jsonb NOT NULL DEFAULT '{}', last_test_ok_at timestamptz, last_test_error text,
  enabled boolean NOT NULL DEFAULT true, version bigint NOT NULL DEFAULT 1,
  CHECK (kind <> 'cli_login' OR secret_id IS NULL)
);

CREATE TABLE role_bindings (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(), tenant_id text NOT NULL DEFAULT 'local' REFERENCES tenants(id),
  owner text NOT NULL REFERENCES principals(id), role_id text NOT NULL REFERENCES roles(id), chat_app text NOT NULL REFERENCES chat_apps(id),
  subscription_id uuid NOT NULL REFERENCES subscriptions(id) ON DELETE CASCADE, priority int NOT NULL DEFAULT 0,
  fallback_subscription_id uuid REFERENCES subscriptions(id) ON DELETE SET NULL, enabled boolean NOT NULL DEFAULT true,
  version bigint NOT NULL DEFAULT 1
);
