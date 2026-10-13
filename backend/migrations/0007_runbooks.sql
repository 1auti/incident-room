-- UC-07: runbooks per service with the embedding the copilot retrieves (BR-12, BR-14).
-- The extension lives in public so every schema (and each test schema) shares one installation;
-- migrations run under a global advisory lock, so concurrent starters cannot race on it.
-- pgvector is not a trusted extension: creating it needs a superuser. Without one, a DBA creates it
-- beforehand (CREATE EXTENSION vector WITH SCHEMA public) and this statement becomes a no-op.
CREATE EXTENSION IF NOT EXISTS vector WITH SCHEMA public;

-- No ON DELETE CASCADE: the foreign key is what stops removing a service with runbooks (BR-20).
-- The embedding is NOT NULL: a runbook is only stored together with its embedding, so the copilot
-- never misses one. 768 is the dimension of the configured model (nomic-embed-text); another model
-- needs a new migration. Removal is physical and takes the embedding with the row (BR-12).
-- updated_at is always supplied by the application (injectable clock, BR-05): no DEFAULT now().
CREATE TABLE runbooks (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    service_id UUID NOT NULL REFERENCES services (id),
    title      TEXT NOT NULL CHECK (btrim(title) <> ''),
    content    TEXT NOT NULL CHECK (btrim(content) <> ''),
    embedding  public.vector(768) NOT NULL,
    created_by UUID NOT NULL REFERENCES users (id),
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX runbooks_service_id_idx ON runbooks (service_id);
