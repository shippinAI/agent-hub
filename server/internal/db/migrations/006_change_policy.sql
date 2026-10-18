-- Change policy (docs/devagotchi/14): versioned tenant-scoped policy documents, change tickets, evidence and reviews.
-- Docs and reviews are append-only; the current doc is the highest version per tenant.
CREATE TABLE policy_docs (
  tenant_id text NOT NULL REFERENCES tenants(id), version bigint NOT NULL CHECK (version > 0), doc jsonb NOT NULL,
  sha256 text NOT NULL, author text NOT NULL, reason text NOT NULL CHECK (length(reason) BETWEEN 3 AND 500),
  created_at timestamptz NOT NULL DEFAULT now(), PRIMARY KEY (tenant_id, version)
);
CREATE TRIGGER policy_docs_ro BEFORE UPDATE OR DELETE ON policy_docs FOR EACH ROW EXECUTE FUNCTION hub_append_only();

CREATE TABLE change_tickets (
  id text PRIMARY KEY CHECK (id ~ '^[A-Za-z0-9][A-Za-z0-9._-]{2,63}$'), tenant_id text NOT NULL DEFAULT 'local' REFERENCES tenants(id),
  author text NOT NULL REFERENCES principals(id), title text NOT NULL CHECK (length(title) BETWEEN 3 AND 300),
  machine_id text REFERENCES machines(id) ON DELETE SET NULL, command text NOT NULL DEFAULT '',
  fields jsonb NOT NULL DEFAULT '{}', lists jsonb NOT NULL DEFAULT '{}', sha256 text NOT NULL,
  status text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','pending_review','passed','blocked','findings','ran')),
  created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(), version bigint NOT NULL DEFAULT 1
);
CREATE INDEX change_tickets_queue ON change_tickets (tenant_id, status, updated_at);

CREATE TABLE ticket_evidence (
  id bigserial PRIMARY KEY, ticket_id text NOT NULL REFERENCES change_tickets(id) ON DELETE CASCADE, kind text NOT NULL,
  path text NOT NULL CHECK (path ~ '^/'), sha256 text NOT NULL, excerpt text NOT NULL DEFAULT '' CHECK (length(excerpt) <= 20000),
  captured_at timestamptz NOT NULL DEFAULT now(), added_by text NOT NULL
);
CREATE INDEX ticket_evidence_ticket ON ticket_evidence (ticket_id, path, captured_at DESC);
CREATE TRIGGER ticket_evidence_ro BEFORE UPDATE ON ticket_evidence FOR EACH ROW EXECUTE FUNCTION hub_append_only();

CREATE TABLE ticket_reviews (
  id bigserial PRIMARY KEY, ticket_id text NOT NULL REFERENCES change_tickets(id) ON DELETE CASCADE,
  reviewer text NOT NULL REFERENCES principals(id), verdict text NOT NULL CHECK (verdict IN ('PASS','BLOCK','FINDINGS')),
  summary text NOT NULL CHECK (length(summary) BETWEEN 3 AND 4000), findings jsonb NOT NULL DEFAULT '[]',
  ticket_sha256 text NOT NULL, created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX ticket_reviews_ticket ON ticket_reviews (ticket_id, id);
CREATE TRIGGER ticket_reviews_ro BEFORE UPDATE OR DELETE ON ticket_reviews FOR EACH ROW EXECUTE FUNCTION hub_append_only();
