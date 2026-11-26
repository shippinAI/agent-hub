-- Run records (docs/factory/plan.md, increment 7). A run is one agent attempt at a task under one lease:
-- who ran it, with what adapter and model, how it ended, and its full transcript. The transcript is stored in
-- ordered chunks; when the run finishes its sha256 is written to the run and to the run.finished event, so the
-- hash chain vouches for the transcript too. Chunks are append-only and a finished run is frozen.
CREATE TABLE runs (
  id bigserial PRIMARY KEY,
  task_id bigint NOT NULL REFERENCES tasks(id),
  fence bigint NOT NULL,
  principal_id text NOT NULL REFERENCES principals(id),
  seat text NOT NULL DEFAULT '' CHECK (length(seat) <= 100),
  adapter text NOT NULL CHECK (adapter ~ '^[a-z0-9][a-z0-9._-]{0,40}$'),
  model text NOT NULL DEFAULT '' CHECK (length(model) <= 200),
  session_id text NOT NULL DEFAULT '' CHECK (length(session_id) <= 200),
  branch text NOT NULL DEFAULT '' CHECK (length(branch) <= 255),
  head_sha text NOT NULL DEFAULT '' CHECK (head_sha = '' OR head_sha ~ '^[0-9a-f]{40}$'),
  status text NOT NULL DEFAULT 'running' CHECK (status IN ('running','completed','handed_off','lost','failed')),
  reason text NOT NULL DEFAULT '' CHECK (length(reason) <= 4000),
  turns int NOT NULL DEFAULT 0 CHECK (turns >= 0),
  transcript_bytes bigint NOT NULL DEFAULT 0,
  transcript_chunks int NOT NULL DEFAULT 0,
  transcript_sha256 text NOT NULL DEFAULT '' CHECK (transcript_sha256 = '' OR transcript_sha256 ~ '^[0-9a-f]{64}$'),
  truncated boolean NOT NULL DEFAULT false,
  started_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  ended_at timestamptz,
  CHECK ((status = 'running') = (ended_at IS NULL))
);
CREATE INDEX runs_task ON runs (task_id, id);
CREATE INDEX runs_open ON runs (task_id) WHERE status = 'running';

CREATE TABLE run_chunks (
  run_id bigint NOT NULL REFERENCES runs(id),
  seq int NOT NULL CHECK (seq >= 0),
  body text NOT NULL CHECK (length(body) BETWEEN 1 AND 262144),
  created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  PRIMARY KEY (run_id, seq)
);

CREATE FUNCTION hub_run_frozen() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF TG_TABLE_NAME = 'run_chunks' THEN
    RAISE EXCEPTION 'run_chunks is append-only';
  END IF;
  IF TG_OP = 'DELETE' OR OLD.status <> 'running' THEN
    RAISE EXCEPTION 'a finished run cannot change';
  END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER runs_frozen BEFORE UPDATE OR DELETE ON runs FOR EACH ROW EXECUTE FUNCTION hub_run_frozen();
CREATE TRIGGER run_chunks_ro BEFORE UPDATE OR DELETE ON run_chunks FOR EACH ROW EXECUTE FUNCTION hub_run_frozen();

INSERT INTO policies(name, subject, action, effect, priority)
SELECT 'anyone records runs', '*', 'task.run', 'allow', 0 WHERE EXISTS (SELECT 1 FROM policies);
