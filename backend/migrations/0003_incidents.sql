-- UC-02: incidents and their append-only timeline (BR-09).
-- No ON DELETE CASCADE anywhere: the default action is what makes the foreign key stop a
-- service removal (BR-20) and keeps the timeline from disappearing with its incident.
-- Instants are always supplied by the application (injectable clock, BR-05): no DEFAULT now().
CREATE TABLE incidents (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title              TEXT NOT NULL CHECK (title = btrim(title) AND title <> ''),
    description        TEXT NOT NULL DEFAULT '',
    service_id         UUID NOT NULL REFERENCES services (id),
    impact             TEXT NOT NULL CHECK (impact IN ('caida_total', 'degradacion', 'menor')),
    suggested_severity TEXT NOT NULL CHECK (suggested_severity IN ('SEV1', 'SEV2', 'SEV3')),
    severity           TEXT NOT NULL CHECK (severity IN ('SEV1', 'SEV2', 'SEV3')),
    state              TEXT NOT NULL CHECK (state IN ('declarado', 'reconocido', 'mitigando', 'resuelto', 'cerrado')),
    declared_by        UUID NOT NULL REFERENCES users (id),
    assigned_to        UUID REFERENCES users (id),
    declared_at        TIMESTAMPTZ NOT NULL
);

CREATE INDEX incidents_service_id_idx ON incidents (service_id);

CREATE TABLE timeline_events (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    incident_id UUID NOT NULL REFERENCES incidents (id),
    type        TEXT NOT NULL CHECK (type IN ('declaracion', 'nota', 'cambio_estado', 'cambio_severidad', 'asignacion', 'escalado', 'postmortem')),
    author_id   UUID REFERENCES users (id),
    body        TEXT NOT NULL DEFAULT '',
    data        JSONB NOT NULL DEFAULT '{}',
    occurred_at TIMESTAMPTZ NOT NULL
);
