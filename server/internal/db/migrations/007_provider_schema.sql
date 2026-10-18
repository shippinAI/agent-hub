-- Provider schema v1 (docs/devagotchi/34-provider-schema.md, ADR 0002). Forward-only and data-safe:
-- every old column value is copied to providers_legacy_007 before it is dropped, the old models list moves
-- to provider_models, and each old row is converted to the new shape (kind/cli_name/router -> adapter).
-- terms_status (verified/unverified) is gone for good (D6, G-17); terms_url and terms_note stay as plain info.

CREATE TABLE providers_legacy_007 AS
  SELECT id, tenant_id, kind, label, models, router_endpoint_id, cli_name, terms_url, terms_status, terms_note, version,
         now() AS archived_at
  FROM providers;
ALTER TABLE providers_legacy_007 ADD PRIMARY KEY (id);

CREATE TABLE provider_models (
  provider_id text NOT NULL REFERENCES providers(id) ON DELETE CASCADE, model text NOT NULL,
  label text NOT NULL DEFAULT '', position int NOT NULL DEFAULT 0, fetched_at timestamptz NOT NULL DEFAULT now(),
  raw jsonb NOT NULL DEFAULT '{}', PRIMARY KEY (provider_id, model)
);
INSERT INTO provider_models(provider_id, model, position, raw)
  SELECT p.id, m.model, m.ord::int, jsonb_build_object('source', 'migration_007')
  FROM providers p, jsonb_array_elements_text(p.models) WITH ORDINALITY AS m(model, ord)
  ON CONFLICT DO NOTHING;

ALTER TABLE router_endpoints DROP CONSTRAINT IF EXISTS router_endpoints_kind_check;
ALTER TABLE router_endpoints ADD CONSTRAINT router_endpoints_kind_check CHECK (kind IN ('litellm','openrouter','direct','custom'));

ALTER TABLE providers
  ADD COLUMN schema_version int NOT NULL DEFAULT 1 CHECK (schema_version >= 1),
  ADD COLUMN vendor text NOT NULL DEFAULT '',
  ADD COLUMN product text NOT NULL DEFAULT '',
  ADD COLUMN icon_ref text NOT NULL DEFAULT '',
  ADD COLUMN docs_url text NOT NULL DEFAULT '',
  ADD COLUMN auth_kinds text[] NOT NULL DEFAULT '{}',
  ADD COLUMN runner text[] NOT NULL DEFAULT '{}',
  ADD COLUMN editions text[] NOT NULL DEFAULT '{local,hosted}',
  ADD COLUMN adapter jsonb,
  ADD COLUMN fallback_adapters jsonb NOT NULL DEFAULT '[]',
  ADD COLUMN host_for text NOT NULL DEFAULT '' CHECK (host_for IN ('', 'harness')),
  ADD COLUMN model_source jsonb NOT NULL DEFAULT '{"source":"static","refresh_minutes":360}',
  ADD COLUMN usage jsonb NOT NULL DEFAULT '{"tokens":"none","quota":"none","cost_basis":"client_estimate"}',
  ADD COLUMN state jsonb NOT NULL DEFAULT '{"read":"live_only"}',
  ADD COLUMN detect jsonb NOT NULL DEFAULT '{}',
  ADD COLUMN capabilities jsonb NOT NULL DEFAULT
    '{"streaming":false,"tools":false,"mcp_client":false,"mcp_host":false,"resume":false,"fork":false,"images":false,"reasoning_effort":false,"approvals":false}',
  ADD COLUMN disabled_at timestamptz;

-- Convert the old rows. Vendor names for the five preset ids; anything else keeps its id prefix.
UPDATE providers p SET
  vendor = CASE split_part(p.id, '-', 1) WHEN 'anthropic' THEN 'Anthropic' WHEN 'openai' THEN 'OpenAI' WHEN 'xai' THEN 'xAI'
           WHEN 'claude' THEN 'Anthropic' WHEN 'chatgpt' THEN 'OpenAI' ELSE split_part(p.id, '-', 1) END,
  product = p.label,
  auth_kinds = CASE p.kind WHEN 'api' THEN '{api_key}'::text[] ELSE '{client_login}'::text[] END,
  runner = CASE p.kind WHEN 'api' THEN '{hub}'::text[] ELSE '{user_device}'::text[] END,
  adapter = CASE p.kind
    WHEN 'api' THEN jsonb_build_object(
      'kind', CASE WHEN p.id LIKE 'anthropic%' THEN 'anthropic_compatible' ELSE 'openai_compatible' END,
      'config', jsonb_build_object('router', CASE WHEN r.id IS NULL THEN jsonb_build_object('mode', 'direct')
        ELSE jsonb_build_object('mode', r.kind, 'endpoint_id', r.id) END))
    ELSE jsonb_build_object('kind', 'headless_json', 'config',
      jsonb_build_object('argv', jsonb_build_array(coalesce(nullif(p.cli_name, ''), p.id), '-p')))
    END,
  capabilities = CASE p.kind WHEN 'api' THEN p.capabilities || '{"streaming":true}'
    ELSE p.capabilities || '{"streaming":true,"tools":true,"resume":true}' END,
  usage = CASE p.kind WHEN 'api' THEN '{"tokens":"api_response","quota":"none","cost_basis":"provider_charged"}'::jsonb ELSE p.usage END
FROM providers p2 LEFT JOIN router_endpoints r ON r.id = p2.router_endpoint_id
WHERE p2.id = p.id;
UPDATE providers SET icon_ref = 'icons/' || lower(regexp_replace(vendor, '[^A-Za-z0-9]+', '', 'g')) WHERE icon_ref = '';

ALTER TABLE providers DROP COLUMN terms_status, DROP COLUMN kind, DROP COLUMN cli_name,
  DROP COLUMN router_endpoint_id, DROP COLUMN models;
ALTER TABLE providers RENAME COLUMN model_source TO models;

-- Enum guards mirror internal/providers (the Go validator is the full check; these keep bad SQL out).
ALTER TABLE providers
  ADD CONSTRAINT providers_auth_kinds_check CHECK (auth_kinds <@ '{client_login,signin_oauth,api_key,plan_key}'::text[]),
  ADD CONSTRAINT providers_runner_check CHECK (runner <@ '{user_device,hosted_workspace,hub}'::text[]),
  ADD CONSTRAINT providers_editions_check CHECK (editions <@ '{local,hosted}'::text[]),
  ADD CONSTRAINT providers_adapter_kind_check CHECK (adapter IS NULL OR adapter->>'kind' IN
    ('acp','codex_app_server','headless_json','openai_compatible','anthropic_compatible'));
