-- agent-hub schema v1. Data tables are editable; events/notes are append-only (enforced below).
CREATE SEQUENCE IF NOT EXISTS fence_seq;

CREATE TABLE principals (
  id text PRIMARY KEY CHECK (id ~ '^[a-z0-9][a-z0-9._-]{0,62}$'),
  display_name text NOT NULL,
  kind text NOT NULL,
  is_human boolean NOT NULL DEFAULT false,
  login text UNIQUE,
  roles text[] NOT NULL DEFAULT '{}',
  enabled boolean NOT NULL DEFAULT true,
  wip_limit int CHECK (wip_limit IS NULL OR wip_limit >= 0),
  last_seen_at timestamptz,
  version bigint NOT NULL DEFAULT 1,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE tokens (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  principal_id text NOT NULL REFERENCES principals(id) ON DELETE CASCADE,
  name text NOT NULL,
  hash bytea NOT NULL UNIQUE,
  prefix text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  last_used_at timestamptz,
  expires_at timestamptz,
  revoked_at timestamptz
);

CREATE TABLE roles (id text PRIMARY KEY, description text NOT NULL DEFAULT '', version bigint NOT NULL DEFAULT 1);

CREATE TABLE policies (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  name text NOT NULL,
  subject text NOT NULL,
  action text NOT NULL,
  selector jsonb NOT NULL DEFAULT '{}',
  effect text NOT NULL CHECK (effect IN ('allow','deny')),
  priority int NOT NULL DEFAULT 0,
  enabled boolean NOT NULL DEFAULT true,
  version bigint NOT NULL DEFAULT 1
);

CREATE TABLE workflows (
  id text PRIMARY KEY,
  name text NOT NULL,
  states jsonb NOT NULL,
  transitions jsonb NOT NULL DEFAULT '[]',
  lanes jsonb NOT NULL DEFAULT '[]',
  initial_state text NOT NULL,
  attention_state text NOT NULL,
  version bigint NOT NULL DEFAULT 1
);

CREATE TABLE projects (
  id text PRIMARY KEY CHECK (id ~ '^[a-z0-9][a-z0-9._-]{0,62}$'),
  name text NOT NULL,
  workflow_id text NOT NULL REFERENCES workflows(id),
  version bigint NOT NULL DEFAULT 1
);

CREATE TABLE settings (key text PRIMARY KEY, value jsonb NOT NULL, description text NOT NULL DEFAULT '', version bigint NOT NULL DEFAULT 1);

CREATE TABLE tasks (
  id bigserial PRIMARY KEY,
  project text NOT NULL REFERENCES projects(id),
  title text NOT NULL CHECK (length(title) BETWEEN 1 AND 300),
  description text NOT NULL DEFAULT '',
  state text NOT NULL,
  priority int NOT NULL DEFAULT 2,
  labels text[] NOT NULL DEFAULT '{}',
  assignee text REFERENCES principals(id),
  holder text REFERENCES principals(id),
  lease_id uuid,
  lease_ttl_seconds int,
  lease_expires_at timestamptz,
  heartbeat_at timestamptz,
  fence bigint NOT NULL DEFAULT 0,
  version bigint NOT NULL DEFAULT 1,
  expiry_count int NOT NULL DEFAULT 0,
  last_worker text,
  progress jsonb,
  created_by text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CHECK ((holder IS NULL) = (lease_id IS NULL))
);
CREATE INDEX tasks_queue ON tasks (state, priority, created_at) WHERE holder IS NULL;
CREATE INDEX tasks_lease ON tasks (lease_expires_at) WHERE holder IS NOT NULL;
CREATE INDEX tasks_fts ON tasks USING gin (to_tsvector('simple', title || ' ' || description));

CREATE TABLE task_deps (
  task_id bigint NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
  depends_on bigint NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
  PRIMARY KEY (task_id, depends_on),
  CHECK (task_id <> depends_on)
);

CREATE TABLE notes (
  id bigserial PRIMARY KEY,
  task_id bigint NOT NULL REFERENCES tasks(id),
  author text NOT NULL,
  kind text NOT NULL,
  body text NOT NULL CHECK (length(body) BETWEEN 1 AND 20000),
  to_principal text,
  fence bigint,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX notes_task ON notes (task_id, id);

CREATE TABLE idempotency_keys (
  principal_id text NOT NULL,
  key text NOT NULL,
  op text NOT NULL,
  request_hash text NOT NULL,
  response jsonb,
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (principal_id, key)
);

CREATE TABLE events (
  seq bigint PRIMARY KEY,
  at timestamptz NOT NULL,
  actor text NOT NULL,
  action text NOT NULL,
  task_id bigint,
  data jsonb NOT NULL DEFAULT '{}',
  fence bigint,
  request_id text,
  prev_hash text NOT NULL,
  hash text NOT NULL
);
CREATE INDEX events_task ON events (task_id, seq);

CREATE FUNCTION hub_append_only() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION '% is append-only', TG_TABLE_NAME; END $$;
CREATE TRIGGER events_ro BEFORE UPDATE OR DELETE ON events FOR EACH ROW EXECUTE FUNCTION hub_append_only();
CREATE TRIGGER events_nt BEFORE TRUNCATE ON events EXECUTE FUNCTION hub_append_only();
CREATE TRIGGER notes_ro BEFORE UPDATE OR DELETE ON notes FOR EACH ROW EXECUTE FUNCTION hub_append_only();
CREATE TRIGGER notes_nt BEFORE TRUNCATE ON notes EXECUTE FUNCTION hub_append_only();
