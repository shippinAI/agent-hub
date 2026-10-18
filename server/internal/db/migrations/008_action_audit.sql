-- Action feed + configurable audit retention (docs/devagotchi/39-action-feed.md, 40-audit-retention.md).
-- action_events is LIST-partitioned by category, each category RANGE-partitioned by day, so retention is a
-- partition DROP (no bulk DELETE, no table bloat). Leaf partitions are created by the hub (auditkeep), not here,
-- because categories are data rows. The governance chain (events) is untouched: archives anchor into it.

CREATE TABLE audit_categories (
  category text PRIMARY KEY CHECK (category ~ '^[a-z][a-z0-9_]{0,30}$'),
  enabled boolean NOT NULL DEFAULT true,
  retain_days int NOT NULL DEFAULT 30 CHECK (retain_days >= 0),        -- 0 = no time limit
  max_rows bigint NOT NULL DEFAULT 0 CHECK (max_rows >= 0),             -- 0 = no row limit
  max_bytes bigint NOT NULL DEFAULT 0 CHECK (max_bytes >= 0),           -- 0 = no size limit
  archive boolean NOT NULL DEFAULT true,                                -- archive before anything is dropped
  capture_output boolean NOT NULL DEFAULT true,
  max_output_bytes int NOT NULL DEFAULT 65536 CHECK (max_output_bytes BETWEEN 0 AND 16777216),
  description text NOT NULL DEFAULT '',
  version bigint NOT NULL DEFAULT 1,
  updated_at timestamptz NOT NULL DEFAULT now(),
  updated_by text NOT NULL DEFAULT 'system'
);

CREATE SEQUENCE action_events_id_seq;
CREATE TABLE action_events (
  id bigint NOT NULL DEFAULT nextval('action_events_id_seq'),
  at timestamptz NOT NULL DEFAULT clock_timestamp(),
  category text NOT NULL,
  kind text NOT NULL,
  call_id text NOT NULL DEFAULT '',
  actor text NOT NULL,
  machine text NOT NULL DEFAULT '',
  status text NOT NULL DEFAULT '',
  data jsonb NOT NULL,
  PRIMARY KEY (category, at, id)
) PARTITION BY LIST (category);
CREATE INDEX action_events_id_idx ON action_events (id);
CREATE INDEX action_events_call_idx ON action_events (call_id);

-- One row per archive segment; each manifest hash chains to the previous segment of its category.
CREATE TABLE audit_archives (
  id bigserial PRIMARY KEY,
  category text NOT NULL,
  from_id bigint NOT NULL, to_id bigint NOT NULL, rows bigint NOT NULL,
  from_at timestamptz, to_at timestamptz,
  location text NOT NULL, bytes bigint NOT NULL, sha256 text NOT NULL,
  rows_hash text NOT NULL, prev_hash text NOT NULL, manifest_hash text NOT NULL, signature text NOT NULL DEFAULT '',
  key_id text NOT NULL DEFAULT '', created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX audit_archives_cat_idx ON audit_archives (category, to_id DESC);

INSERT INTO audit_categories(category, retain_days, max_rows, max_bytes, description) VALUES
  ('tool', 30, 0, 0, 'MCP tool calls made by agents (name, redacted arguments, result status)'),
  ('proc', 30, 0, 0, 'Commands and scripts run on machines: argv, cwd, exit code, output'),
  ('fs', 30, 0, 0, 'File reads, writes and edits on machines, with diffs for edits'),
  ('gui', 14, 0, 0, 'GUI observe/input actions on machines'),
  ('net', 30, 0, 0, 'Network fetches made through the bridge'),
  ('policy', 90, 0, 0, 'Policy decisions and denials for machine calls');
