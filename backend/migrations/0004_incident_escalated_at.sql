-- UC-03: escalated_at marks an incident escalated by SLA (BR-04). UC-03 only reads it; the
-- escalation itself is implemented by UC-04.
-- Nullable and without DEFAULT: the instant is always supplied by the application (BR-05).
ALTER TABLE incidents ADD COLUMN escalated_at TIMESTAMPTZ;
