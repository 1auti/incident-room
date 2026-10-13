-- UC-06: resolution and closing columns set by T3 and T4 (BR-06, BR-07).
-- Nullable and without DEFAULT: the instants are always supplied by the application (BR-05).
ALTER TABLE incidents ADD COLUMN root_cause TEXT;
ALTER TABLE incidents ADD COLUMN resolved_at TIMESTAMPTZ;
ALTER TABLE incidents ADD COLUMN closed_at TIMESTAMPTZ;

-- Postmortem as defined in docs/domain.md section 1. UC-06 only reads it (T4 of a SEV1 needs an
-- approved one, BR-07); generating, editing and approving it arrive with UC-08 (BR-13).
-- UNIQUE incident_id: at most one postmortem per incident. No ON DELETE CASCADE, like the timeline.
CREATE TABLE postmortems (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    incident_id      UUID NOT NULL UNIQUE REFERENCES incidents (id),
    content          TEXT NOT NULL,
    status           TEXT NOT NULL CHECK (status IN ('borrador', 'aprobado')),
    generated_by_llm BOOLEAN NOT NULL,
    approved_by      UUID REFERENCES users (id),
    approved_at      TIMESTAMPTZ,
    updated_at       TIMESTAMPTZ NOT NULL,
    -- An approved postmortem always records who approved it and when (BR-13).
    CHECK (status = 'borrador' OR (approved_by IS NOT NULL AND approved_at IS NOT NULL))
);
