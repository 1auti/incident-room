-- UC-04: acknowledged_at is set when an incident moves from declarado to reconocido (T1).
-- UC-04 needs it to tell acknowledged incidents apart; UC-06 extends the transitions.
-- Nullable and without DEFAULT: the instant is always supplied by the application (BR-05).
ALTER TABLE incidents ADD COLUMN acknowledged_at TIMESTAMPTZ;
