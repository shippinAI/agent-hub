-- Git provenance (docs/factory/plan.md, increment 3). A task links to the git objects that carry its work:
-- the branch, its commits, the pull request and the check runs. Links are written by the agent (link_git) or
-- by the GitHub webhook ingest, which also turns completed check runs into evidence (kind ci.<check name>).
-- head_sha on a branch link is the newest commit seen; CI evidence for an older commit no longer counts.
CREATE TABLE task_links (
  id bigserial PRIMARY KEY,
  task_id bigint NOT NULL REFERENCES tasks(id),
  kind text NOT NULL CHECK (kind IN ('branch','commit','pr','check_run')),
  repo text NOT NULL CHECK (repo ~ '^[A-Za-z0-9_.-]{1,100}/[A-Za-z0-9_.-]{1,100}$'),
  ref text NOT NULL CHECK (length(ref) BETWEEN 1 AND 255),
  url text NOT NULL DEFAULT '' CHECK (length(url) <= 2000),
  state text NOT NULL DEFAULT '' CHECK (length(state) <= 40),
  head_sha text NOT NULL DEFAULT '' CHECK (head_sha = '' OR head_sha ~ '^[0-9a-f]{40}$'),
  data jsonb NOT NULL DEFAULT '{}',
  author text NOT NULL REFERENCES principals(id),
  created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  UNIQUE (task_id, kind, repo, ref)
);
CREATE INDEX task_links_lookup ON task_links (repo, kind, ref);
CREATE INDEX task_links_sha ON task_links (repo, head_sha) WHERE head_sha <> '';

-- Webhook deliveries seen in the last week, so a redelivery is applied once.
CREATE TABLE webhook_deliveries (
  id text PRIMARY KEY CHECK (length(id) BETWEEN 1 AND 200),
  source text NOT NULL,
  event text NOT NULL,
  received_at timestamptz NOT NULL DEFAULT clock_timestamp()
);
CREATE INDEX webhook_deliveries_age ON webhook_deliveries (received_at);

-- The webhook acts as this principal; it holds no token and no role.
INSERT INTO principals(id, display_name, kind, roles, enabled) VALUES ('github', 'GitHub', 'service', '{}', true)
ON CONFLICT DO NOTHING;

INSERT INTO policies(name, subject, action, effect, priority)
SELECT 'anyone links git work', '*', 'task.link', 'allow', 0 WHERE EXISTS (SELECT 1 FROM policies);
