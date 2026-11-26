-- Usage ledger and budgets (docs/factory/plan.md increment 4; design in docs/devagotchi/30 and 33).
-- First slice: an append-only ledger attributed to principal, task and lease; a price catalog (data, empty by
-- default: prices change, so a human or an import adds them with their source); and budgets that stop new claims.
-- Money is integer micro-USD. Tokens are stored by kind; cache_read is a subset of input, so
-- uncached input = input_tokens - cache_read_tokens and nothing counts twice. reasoning is likewise a subset of output.
CREATE TABLE price_catalog (
  id text PRIMARY KEY CHECK (id ~ '^[a-z0-9][a-z0-9._-]{0,80}$'),
  provider_id text NOT NULL CHECK (provider_id ~ '^[a-z0-9][a-z0-9._-]{0,62}$'),
  model text NOT NULL CHECK (length(model) BETWEEN 1 AND 200),
  input_micros_per_mtok bigint NOT NULL DEFAULT 0 CHECK (input_micros_per_mtok >= 0),
  cache_read_micros_per_mtok bigint NOT NULL DEFAULT 0 CHECK (cache_read_micros_per_mtok >= 0),
  cache_write_micros_per_mtok bigint NOT NULL DEFAULT 0 CHECK (cache_write_micros_per_mtok >= 0),
  output_micros_per_mtok bigint NOT NULL DEFAULT 0 CHECK (output_micros_per_mtok >= 0),
  effective_from timestamptz NOT NULL,
  source text NOT NULL DEFAULT 'user' CHECK (source IN ('user','vendor_page','litellm','openrouter')),
  source_url text NOT NULL DEFAULT '' CHECK (length(source_url) <= 2000),
  version bigint NOT NULL DEFAULT 1,
  UNIQUE (provider_id, model, effective_from)
);

CREATE TABLE usage_events (
  id bigserial PRIMARY KEY,
  occurred_at timestamptz NOT NULL,
  recorded_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  principal_id text NOT NULL REFERENCES principals(id),
  task_id bigint REFERENCES tasks(id),
  fence bigint,
  project text,
  provider_id text NOT NULL CHECK (provider_id ~ '^[a-z0-9][a-z0-9._-]{0,62}$'),
  model text NOT NULL DEFAULT '' CHECK (length(model) <= 200),
  billing_kind text NOT NULL CHECK (billing_kind IN ('api_key','subscription','router')),
  input_tokens bigint NOT NULL DEFAULT 0 CHECK (input_tokens >= 0),
  cache_read_tokens bigint NOT NULL DEFAULT 0 CHECK (cache_read_tokens >= 0 AND cache_read_tokens <= input_tokens),
  cache_write_tokens bigint NOT NULL DEFAULT 0 CHECK (cache_write_tokens >= 0),
  output_tokens bigint NOT NULL DEFAULT 0 CHECK (output_tokens >= 0),
  reasoning_tokens bigint NOT NULL DEFAULT 0 CHECK (reasoning_tokens >= 0 AND reasoning_tokens <= output_tokens),
  requests int NOT NULL DEFAULT 1 CHECK (requests >= 0),
  cost_micros bigint CHECK (cost_micros IS NULL OR cost_micros >= 0),
  cost_basis text CHECK (cost_basis IN ('provider_charged','computed','client_estimate')),
  price_id text REFERENCES price_catalog(id) ON DELETE SET NULL,
  source text NOT NULL CHECK (source IN ('stream','otel','app_server','api_response','import','manual')),
  external_ref text NOT NULL DEFAULT '' CHECK (length(external_ref) <= 300),
  dedupe_key text NOT NULL UNIQUE CHECK (length(dedupe_key) BETWEEN 1 AND 400),
  CHECK ((cost_micros IS NULL) = (cost_basis IS NULL))
);
CREATE INDEX usage_events_task ON usage_events (task_id, occurred_at) WHERE task_id IS NOT NULL;
CREATE INDEX usage_events_principal ON usage_events (principal_id, occurred_at);
CREATE INDEX usage_events_time ON usage_events (occurred_at);
-- price_id is the one column a later write may touch (ON DELETE SET NULL when a mistaken price row is removed).
CREATE FUNCTION hub_usage_append_only() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF TG_OP = 'UPDATE' AND (to_jsonb(NEW) - 'price_id') = (to_jsonb(OLD) - 'price_id') THEN
    RETURN NEW;
  END IF;
  RAISE EXCEPTION 'usage_events is append-only';
END $$;
CREATE TRIGGER usage_events_ro BEFORE UPDATE OR DELETE ON usage_events FOR EACH ROW EXECUTE FUNCTION hub_usage_append_only();

-- A budget caps cost or tokens for one scope. Over the limit, new claims in that scope are refused
-- (budget_exceeded) until a human raises the limit, disables the budget, or the period rolls over.
CREATE TABLE budgets (
  id text PRIMARY KEY CHECK (id ~ '^[a-z0-9][a-z0-9._-]{0,62}$'),
  label text NOT NULL DEFAULT '',
  scope_kind text NOT NULL CHECK (scope_kind IN ('all','principal','project','task')),
  scope_id text NOT NULL DEFAULT '' CHECK ((scope_kind = 'all') = (scope_id = '')),
  metric text NOT NULL CHECK (metric IN ('cost_micros','tokens')),
  period text NOT NULL CHECK (period IN ('day','week','month','total')),
  limit_value bigint NOT NULL CHECK (limit_value > 0),
  enabled boolean NOT NULL DEFAULT true,
  version bigint NOT NULL DEFAULT 1
);

-- One row per budget, period and crossing, so the event fires once per period.
CREATE TABLE budget_alerts (
  budget_id text NOT NULL REFERENCES budgets(id) ON DELETE CASCADE,
  period_start timestamptz NOT NULL,
  value bigint NOT NULL,
  fired_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  PRIMARY KEY (budget_id, period_start)
);

INSERT INTO policies(name, subject, action, effect, priority)
SELECT 'anyone records own usage', '*', 'usage.record', 'allow', 0 WHERE EXISTS (SELECT 1 FROM policies);
