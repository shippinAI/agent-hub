-- Evidence gate (docs/factory/plan.md, increment 2). A task carries acceptance criteria; evidence rows record
-- proof (a test run, a CI check, an eval, a review) against the lease that produced it. A lane may list
-- evidence kinds it requires (workflow data); complete refuses while the latest row of a required kind, for the
-- current fence, is missing or failing. Evidence is append-only: a re-run appends, it never rewrites.
ALTER TABLE tasks ADD COLUMN acceptance text[] NOT NULL DEFAULT '{}';

CREATE TABLE task_evidence (
  id bigserial PRIMARY KEY,
  task_id bigint NOT NULL REFERENCES tasks(id),
  kind text NOT NULL CHECK (kind ~ '^[a-z][a-z0-9_.-]{1,40}$'),
  status text NOT NULL CHECK (status IN ('pass','fail')),
  summary text NOT NULL DEFAULT '' CHECK (length(summary) <= 4000),
  ref text NOT NULL DEFAULT '' CHECK (length(ref) <= 2000),
  sha256 text CHECK (sha256 ~ '^[0-9a-f]{64}$'),
  data jsonb NOT NULL DEFAULT '{}',
  author text NOT NULL REFERENCES principals(id),
  fence bigint NOT NULL,
  created_at timestamptz NOT NULL DEFAULT clock_timestamp()
);
CREATE INDEX task_evidence_task ON task_evidence (task_id, fence, kind, id DESC);
CREATE TRIGGER task_evidence_ro BEFORE UPDATE OR DELETE ON task_evidence FOR EACH ROW EXECUTE FUNCTION hub_append_only();

-- Installs seeded before this migration get the same allow the presets now carry (deny-by-default otherwise).
-- A fresh database has no policies yet; the seed adds it from presets/default.json.
INSERT INTO policies(name, subject, action, effect, priority)
SELECT 'anyone attaches evidence', '*', 'task.evidence:*', 'allow', 0 WHERE EXISTS (SELECT 1 FROM policies);
