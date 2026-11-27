-- Outcome loop: each run remembers the lane it ran in (the task's active state at start), so a later run in the
-- same lane on the same task marks the earlier completed run as reworked. Scores and routing are computed from
-- runs, events and usage at query time; nothing here is a stored verdict.
ALTER TABLE runs ADD COLUMN lane text NOT NULL DEFAULT '';
CREATE INDEX runs_scores ON runs(principal_id, lane, started_at);

INSERT INTO settings(key, value, description)
SELECT k, v, d FROM (VALUES
  ('route_min_runs', '0'::jsonb, 'claim_next skips tasks in a project and lane where the caller has at least this many scored runs and a success rate below route_min_success_pct (0 = routing off)'),
  ('route_min_success_pct', '50'::jsonb, 'Success rate (accepted runs out of scored runs, percent) below which claim_next routes a lane''s work away from a principal')
) AS s(k, v, d) WHERE EXISTS (SELECT 1 FROM settings)
ON CONFLICT (key) DO NOTHING;
