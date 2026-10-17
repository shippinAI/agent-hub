-- Helpers that read workflow data (states/lanes are JSON in workflows), used for readiness.
CREATE FUNCTION hub_state_kind(p text, st text) RETURNS text LANGUAGE sql STABLE AS $$
  SELECT s->>'kind' FROM projects pr JOIN workflows w ON w.id = pr.workflow_id,
    jsonb_array_elements(w.states) s WHERE pr.id = p AND s->>'key' = st LIMIT 1 $$;

CREATE FUNCTION hub_is_queue(p text, st text) RETURNS boolean LANGUAGE sql STABLE AS $$
  SELECT EXISTS (SELECT 1 FROM projects pr JOIN workflows w ON w.id = pr.workflow_id,
    jsonb_array_elements(w.lanes) l WHERE pr.id = p AND l->>'queue' = st) $$;

CREATE FUNCTION hub_open_deps(tid bigint) RETURNS bigint[] LANGUAGE sql STABLE AS $$
  SELECT coalesce(array_agg(d.depends_on ORDER BY d.depends_on), '{}') FROM task_deps d
    JOIN tasks t2 ON t2.id = d.depends_on
    WHERE d.task_id = tid AND hub_state_kind(t2.project, t2.state) IS DISTINCT FROM 'done' $$;
