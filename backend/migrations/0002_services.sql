CREATE TABLE services (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name           TEXT NOT NULL CHECK (name = btrim(name) AND name <> ''),
    criticality    TEXT NOT NULL CHECK (criticality IN ('critica', 'importante', 'estandar')),
    oncall_user_id UUID REFERENCES users (id)
);

-- BR-20 obligation for later migrations: any table that references a service (incidents in UC-02,
-- runbooks in UC-07) must declare `service_id UUID NOT NULL REFERENCES services (id)` with the
-- default action. Never ON DELETE CASCADE: the database foreign key is what stops a service
-- removal that races with a new incident or runbook (it surfaces as service.ErrInUse).

-- Service names are unique regardless of case: "Payments" and "payments" are the same name.
-- The name is stored as typed (trimmed); only the comparison ignores case.
CREATE UNIQUE INDEX services_name_lower_key ON services (lower(name));
