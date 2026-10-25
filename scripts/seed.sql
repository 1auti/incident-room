-- Demo seed data for LOCAL DEVELOPMENT ONLY. Run it against the disposable dev DB, never a shared or real one.
--
-- Run (all migrations applied):
--   psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f scripts/seed.sql
--
-- Re-runnable: fixed UUIDs + ON CONFLICT DO NOTHING (re-running adds nothing; timestamps keep the first run's values).
-- Timestamps are relative to now() of the first run and respect BR-01 (suggested severity), BR-03/04 (SLA 5/15/60 min
-- by severity, escalation when still 'declarado' past the deadline), BR-06, BR-07, BR-13 and the state machine T1-T4.
-- Seeded users cannot log in (password_hash is a placeholder, not a valid hash). Promote/reset via the app's admin flow.
-- NOT seeded: runbooks (their embeddings come from Ollama), sessions.

BEGIN;

-- Placeholder password_hash: not a valid hash, so no password can ever match it.
INSERT INTO users (id, name, email, password_hash, role, created_at) VALUES
    ('00000000-0000-0000-0000-000000000001', 'Ines Ingeniera', 'ingeniero@example.com', '!seed-no-login', 'ingeniero', now() - interval '30 days'),
    ('00000000-0000-0000-0000-000000000002', 'Omar Oncall',     'oncall@example.com',    '!seed-no-login', 'oncall',    now() - interval '30 days'),
    ('00000000-0000-0000-0000-000000000003', 'Ada Admin',       'admin@example.com',     '!seed-no-login', 'admin',     now() - interval '30 days')
ON CONFLICT DO NOTHING;

-- Only the 'oncall' user can be on-call (BR-11); notifications and reports have none.
INSERT INTO services (id, name, criticality, oncall_user_id) VALUES
    ('00000000-0000-0000-0001-000000000001', 'payments',      'critica',    '00000000-0000-0000-0000-000000000002'),
    ('00000000-0000-0000-0001-000000000002', 'auth',          'critica',    '00000000-0000-0000-0000-000000000002'),
    ('00000000-0000-0000-0001-000000000003', 'search',        'importante', '00000000-0000-0000-0000-000000000002'),
    ('00000000-0000-0000-0001-000000000004', 'notifications', 'importante', NULL),
    ('00000000-0000-0000-0001-000000000005', 'reports',       'estandar',   NULL)
ON CONFLICT DO NOTHING;

-- Incident ids: 00000000-0000-0000-0002-0000000000NN. Users: 01 ingeniero, 02 oncall, 03 admin.
INSERT INTO incidents (id, title, description, service_id, impact, suggested_severity, severity, state,
                       declared_by, assigned_to, declared_at, acknowledged_at, escalated_at, root_cause, resolved_at, closed_at) VALUES
-- 01 SEV1 cerrado (approved postmortem, BR-07)
('00000000-0000-0000-0002-000000000001', 'Caída total de pagos', 'Todos los cobros devuelven error 500.',
 '00000000-0000-0000-0001-000000000001', 'caida_total', 'SEV1', 'SEV1', 'cerrado',
 '00000000-0000-0000-0000-000000000001', '00000000-0000-0000-0000-000000000002',
 now() - interval '10 days', now() - interval '10 days' + interval '3 minutes', NULL,
 'Pool de conexiones agotado por fuga en v2.3', now() - interval '10 days' + interval '90 minutes', now() - interval '8 days'),
-- 02 SEV1 mitigando; declarer raised severity above the suggested SEV2 (BR-02)
('00000000-0000-0000-0002-000000000002', 'Login lento para todos los usuarios', 'Los inicios de sesión tardan más de 20 s.',
 '00000000-0000-0000-0001-000000000002', 'degradacion', 'SEV2', 'SEV1', 'mitigando',
 '00000000-0000-0000-0000-000000000001', '00000000-0000-0000-0000-000000000002',
 now() - interval '2 hours', now() - interval '2 hours' + interval '4 minutes', NULL, NULL, NULL, NULL),
-- 03 SEV2 resuelto (draft postmortem)
('00000000-0000-0000-0002-000000000003', 'Búsquedas devuelven resultados vacíos', 'El índice quedó desactualizado tras un deploy.',
 '00000000-0000-0000-0001-000000000003', 'degradacion', 'SEV2', 'SEV2', 'resuelto',
 '00000000-0000-0000-0000-000000000001', '00000000-0000-0000-0000-000000000002',
 now() - interval '3 days', now() - interval '3 days' + interval '10 minutes', NULL,
 'Reindexado omitido en el pipeline de deploy', now() - interval '3 days' + interval '2 hours', NULL),
-- 04 SEV3 cerrado, no postmortem needed
('00000000-0000-0000-0002-000000000004', 'Reporte semanal con columnas desalineadas', 'Cosmético en el export a CSV.',
 '00000000-0000-0000-0001-000000000005', 'menor', 'SEV3', 'SEV3', 'cerrado',
 '00000000-0000-0000-0000-000000000001', NULL,
 now() - interval '6 days', now() - interval '6 days' + interval '30 minutes', NULL,
 'Separador de columnas mal escapado', now() - interval '6 days' + interval '3 hours', now() - interval '5 days'),
-- 05 SEV2 declarado, SLA (15 min) expired -> escalated (BR-04); no on-call
('00000000-0000-0000-0002-000000000005', 'Notificaciones no se envían', 'Cola de envío detenida.',
 '00000000-0000-0000-0001-000000000004', 'caida_total', 'SEV2', 'SEV2', 'declarado',
 '00000000-0000-0000-0000-000000000001', NULL,
 now() - interval '45 minutes', NULL, now() - interval '45 minutes' + interval '15 minutes 1 second', NULL, NULL, NULL),
-- 06 SEV3 declarado, still within its 60 min SLA
('00000000-0000-0000-0002-000000000006', 'Reporte mensual tarda en generarse', 'Demora de varios minutos.',
 '00000000-0000-0000-0001-000000000005', 'degradacion', 'SEV3', 'SEV3', 'declarado',
 '00000000-0000-0000-0000-000000000001', NULL,
 now() - interval '10 minutes', NULL, NULL, NULL, NULL, NULL),
-- 07 SEV3 reconocido
('00000000-0000-0000-0002-000000000007', 'Recibos de pago con fecha en UTC', 'Menor: el formato de fecha no respeta zona horaria.',
 '00000000-0000-0000-0001-000000000001', 'menor', 'SEV3', 'SEV3', 'reconocido',
 '00000000-0000-0000-0000-000000000001', '00000000-0000-0000-0000-000000000002',
 now() - interval '50 minutes', now() - interval '50 minutes' + interval '20 minutes', NULL, NULL, NULL, NULL),
-- 08 SEV2 mitigando, escalated first and acknowledged late
('00000000-0000-0000-0002-000000000008', 'Búsqueda caída', 'El servicio de búsqueda no responde.',
 '00000000-0000-0000-0001-000000000003', 'caida_total', 'SEV2', 'SEV2', 'mitigando',
 '00000000-0000-0000-0000-000000000001', '00000000-0000-0000-0000-000000000002',
 now() - interval '5 hours', now() - interval '5 hours' + interval '25 minutes', now() - interval '5 hours' + interval '15 minutes 1 second', NULL, NULL, NULL),
-- 09 SEV1 resuelto (draft postmortem; cannot close until approved, BR-07)
('00000000-0000-0000-0002-000000000009', 'Autenticación caída', 'Nadie puede iniciar sesión.',
 '00000000-0000-0000-0001-000000000002', 'caida_total', 'SEV1', 'SEV1', 'resuelto',
 '00000000-0000-0000-0000-000000000001', '00000000-0000-0000-0000-000000000002',
 now() - interval '1 day', now() - interval '1 day' + interval '2 minutes', NULL,
 'Certificado del proveedor de identidad vencido', now() - interval '1 day' + interval '75 minutes', NULL),
-- 10 SEV2 reconocido, within SLA
('00000000-0000-0000-0002-000000000010', 'Tokens de sesión expiran antes de tiempo', 'Algunos usuarios deben volver a loguearse.',
 '00000000-0000-0000-0001-000000000002', 'degradacion', 'SEV2', 'SEV2', 'reconocido',
 '00000000-0000-0000-0000-000000000001', '00000000-0000-0000-0000-000000000002',
 now() - interval '8 minutes', now() - interval '8 minutes' + interval '5 minutes', NULL, NULL, NULL, NULL)
ON CONFLICT DO NOTHING;

-- Event ids: e0000000-0000-0000-NN00-0000000000SS (NN incident, SS sequence). author NULL = system (escalado).
INSERT INTO timeline_events (id, incident_id, type, author_id, body, data, occurred_at) VALUES
-- 01
('e0000000-0000-0000-0100-000000000001', '00000000-0000-0000-0002-000000000001', 'declaracion',   '00000000-0000-0000-0000-000000000001', 'Incidente declarado.', '{}', now() - interval '10 days'),
('e0000000-0000-0000-0100-000000000002', '00000000-0000-0000-0002-000000000001', 'cambio_estado', '00000000-0000-0000-0000-000000000002', 'Reconocido.', '{"from":"declarado","to":"reconocido"}', now() - interval '10 days' + interval '3 minutes'),
('e0000000-0000-0000-0100-000000000003', '00000000-0000-0000-0002-000000000001', 'cambio_estado', '00000000-0000-0000-0000-000000000002', 'Mitigando.', '{"from":"reconocido","to":"mitigando"}', now() - interval '10 days' + interval '20 minutes'),
('e0000000-0000-0000-0100-000000000004', '00000000-0000-0000-0002-000000000001', 'nota',          '00000000-0000-0000-0000-000000000002', 'Se reinició el servicio y se limitó el pool.', '{}', now() - interval '10 days' + interval '45 minutes'),
('e0000000-0000-0000-0100-000000000005', '00000000-0000-0000-0002-000000000001', 'cambio_estado', '00000000-0000-0000-0000-000000000002', 'Pool de conexiones agotado por fuga en v2.3', '{"from":"mitigando","to":"resuelto"}', now() - interval '10 days' + interval '90 minutes'),
('e0000000-0000-0000-0100-000000000006', '00000000-0000-0000-0002-000000000001', 'postmortem',    '00000000-0000-0000-0000-000000000002', 'Borrador de postmortem generado.', '{}', now() - interval '10 days' + interval '100 minutes'),
('e0000000-0000-0000-0100-000000000007', '00000000-0000-0000-0002-000000000001', 'postmortem',    '00000000-0000-0000-0000-000000000003', 'Postmortem aprobado.', '{}', now() - interval '9 days'),
('e0000000-0000-0000-0100-000000000008', '00000000-0000-0000-0002-000000000001', 'cambio_estado', '00000000-0000-0000-0000-000000000003', 'Cerrado.', '{"from":"resuelto","to":"cerrado"}', now() - interval '8 days'),
-- 02
('e0000000-0000-0000-0200-000000000001', '00000000-0000-0000-0002-000000000002', 'declaracion',      '00000000-0000-0000-0000-000000000001', 'Incidente declarado.', '{}', now() - interval '2 hours'),
('e0000000-0000-0000-0200-000000000002', '00000000-0000-0000-0002-000000000002', 'cambio_severidad', '00000000-0000-0000-0000-000000000001', 'Severidad elegida distinta de la sugerida.', '{"from":"SEV2","to":"SEV1"}', now() - interval '2 hours'),
('e0000000-0000-0000-0200-000000000003', '00000000-0000-0000-0002-000000000002', 'cambio_estado',    '00000000-0000-0000-0000-000000000002', 'Reconocido.', '{"from":"declarado","to":"reconocido"}', now() - interval '2 hours' + interval '4 minutes'),
('e0000000-0000-0000-0200-000000000004', '00000000-0000-0000-0002-000000000002', 'cambio_estado',    '00000000-0000-0000-0000-000000000002', 'Mitigando.', '{"from":"reconocido","to":"mitigando"}', now() - interval '2 hours' + interval '30 minutes'),
('e0000000-0000-0000-0200-000000000005', '00000000-0000-0000-0002-000000000002', 'nota',             '00000000-0000-0000-0000-000000000002', 'Se escaló la base de sesiones.', '{}', now() - interval '2 hours' + interval '40 minutes'),
-- 03
('e0000000-0000-0000-0300-000000000001', '00000000-0000-0000-0002-000000000003', 'declaracion',   '00000000-0000-0000-0000-000000000001', 'Incidente declarado.', '{}', now() - interval '3 days'),
('e0000000-0000-0000-0300-000000000002', '00000000-0000-0000-0002-000000000003', 'cambio_estado', '00000000-0000-0000-0000-000000000002', 'Reconocido.', '{"from":"declarado","to":"reconocido"}', now() - interval '3 days' + interval '10 minutes'),
('e0000000-0000-0000-0300-000000000003', '00000000-0000-0000-0002-000000000003', 'cambio_estado', '00000000-0000-0000-0000-000000000002', 'Mitigando.', '{"from":"reconocido","to":"mitigando"}', now() - interval '3 days' + interval '40 minutes'),
('e0000000-0000-0000-0300-000000000004', '00000000-0000-0000-0002-000000000003', 'cambio_estado', '00000000-0000-0000-0000-000000000002', 'Reindexado omitido en el pipeline de deploy', '{"from":"mitigando","to":"resuelto"}', now() - interval '3 days' + interval '2 hours'),
('e0000000-0000-0000-0300-000000000005', '00000000-0000-0000-0002-000000000003', 'postmortem',    '00000000-0000-0000-0000-000000000002', 'Borrador de postmortem generado.', '{}', now() - interval '3 days' + interval '130 minutes'),
-- 04
('e0000000-0000-0000-0400-000000000001', '00000000-0000-0000-0002-000000000004', 'declaracion',   '00000000-0000-0000-0000-000000000001', 'Incidente declarado.', '{}', now() - interval '6 days'),
('e0000000-0000-0000-0400-000000000002', '00000000-0000-0000-0002-000000000004', 'cambio_estado', '00000000-0000-0000-0000-000000000003', 'Reconocido.', '{"from":"declarado","to":"reconocido"}', now() - interval '6 days' + interval '30 minutes'),
('e0000000-0000-0000-0400-000000000003', '00000000-0000-0000-0002-000000000004', 'cambio_estado', '00000000-0000-0000-0000-000000000003', 'Mitigando.', '{"from":"reconocido","to":"mitigando"}', now() - interval '6 days' + interval '1 hour'),
('e0000000-0000-0000-0400-000000000004', '00000000-0000-0000-0002-000000000004', 'cambio_estado', '00000000-0000-0000-0000-000000000003', 'Separador de columnas mal escapado', '{"from":"mitigando","to":"resuelto"}', now() - interval '6 days' + interval '3 hours'),
('e0000000-0000-0000-0400-000000000005', '00000000-0000-0000-0002-000000000004', 'cambio_estado', '00000000-0000-0000-0000-000000000003', 'Cerrado.', '{"from":"resuelto","to":"cerrado"}', now() - interval '5 days'),
-- 05
('e0000000-0000-0000-0500-000000000001', '00000000-0000-0000-0002-000000000005', 'declaracion', '00000000-0000-0000-0000-000000000001', 'Incidente declarado.', '{}', now() - interval '45 minutes'),
('e0000000-0000-0000-0500-000000000002', '00000000-0000-0000-0002-000000000005', 'escalado',    NULL, 'SLA de reconocimiento vencido: escalado a los admins.', '{}', now() - interval '45 minutes' + interval '15 minutes 1 second'),
-- 06
('e0000000-0000-0000-0600-000000000001', '00000000-0000-0000-0002-000000000006', 'declaracion', '00000000-0000-0000-0000-000000000001', 'Incidente declarado.', '{}', now() - interval '10 minutes'),
-- 07
('e0000000-0000-0000-0700-000000000001', '00000000-0000-0000-0002-000000000007', 'declaracion',   '00000000-0000-0000-0000-000000000001', 'Incidente declarado.', '{}', now() - interval '50 minutes'),
('e0000000-0000-0000-0700-000000000002', '00000000-0000-0000-0002-000000000007', 'cambio_estado', '00000000-0000-0000-0000-000000000002', 'Reconocido.', '{"from":"declarado","to":"reconocido"}', now() - interval '30 minutes'),
-- 08
('e0000000-0000-0000-0800-000000000001', '00000000-0000-0000-0002-000000000008', 'declaracion',   '00000000-0000-0000-0000-000000000001', 'Incidente declarado.', '{}', now() - interval '5 hours'),
('e0000000-0000-0000-0800-000000000002', '00000000-0000-0000-0002-000000000008', 'escalado',      NULL, 'SLA de reconocimiento vencido: escalado a los admins.', '{}', now() - interval '5 hours' + interval '15 minutes 1 second'),
('e0000000-0000-0000-0800-000000000003', '00000000-0000-0000-0002-000000000008', 'cambio_estado', '00000000-0000-0000-0000-000000000002', 'Reconocido.', '{"from":"declarado","to":"reconocido"}', now() - interval '5 hours' + interval '25 minutes'),
('e0000000-0000-0000-0800-000000000004', '00000000-0000-0000-0002-000000000008', 'cambio_estado', '00000000-0000-0000-0000-000000000002', 'Mitigando.', '{"from":"reconocido","to":"mitigando"}', now() - interval '5 hours' + interval '50 minutes'),
-- 09
('e0000000-0000-0000-0900-000000000001', '00000000-0000-0000-0002-000000000009', 'declaracion',   '00000000-0000-0000-0000-000000000001', 'Incidente declarado.', '{}', now() - interval '1 day'),
('e0000000-0000-0000-0900-000000000002', '00000000-0000-0000-0002-000000000009', 'cambio_estado', '00000000-0000-0000-0000-000000000002', 'Reconocido.', '{"from":"declarado","to":"reconocido"}', now() - interval '1 day' + interval '2 minutes'),
('e0000000-0000-0000-0900-000000000003', '00000000-0000-0000-0002-000000000009', 'cambio_estado', '00000000-0000-0000-0000-000000000002', 'Mitigando.', '{"from":"reconocido","to":"mitigando"}', now() - interval '1 day' + interval '15 minutes'),
('e0000000-0000-0000-0900-000000000004', '00000000-0000-0000-0002-000000000009', 'cambio_estado', '00000000-0000-0000-0000-000000000002', 'Certificado del proveedor de identidad vencido', '{"from":"mitigando","to":"resuelto"}', now() - interval '1 day' + interval '75 minutes'),
('e0000000-0000-0000-0900-000000000005', '00000000-0000-0000-0002-000000000009', 'postmortem',    '00000000-0000-0000-0000-000000000002', 'Borrador de postmortem generado.', '{}', now() - interval '1 day' + interval '80 minutes'),
-- 10
('e0000000-0000-0000-1000-000000000001', '00000000-0000-0000-0002-000000000010', 'declaracion',   '00000000-0000-0000-0000-000000000001', 'Incidente declarado.', '{}', now() - interval '8 minutes'),
('e0000000-0000-0000-1000-000000000002', '00000000-0000-0000-0002-000000000010', 'cambio_estado', '00000000-0000-0000-0000-000000000002', 'Reconocido.', '{"from":"declarado","to":"reconocido"}', now() - interval '3 minutes')
ON CONFLICT DO NOTHING;

-- Postmortems only for resolved/closed incidents (BR-13); 01 approved by admin (needed to close SEV1, BR-07).
INSERT INTO postmortems (id, incident_id, content, status, generated_by_llm, approved_by, approved_at, updated_at) VALUES
('00000000-0000-0000-0003-000000000001', '00000000-0000-0000-0002-000000000001',
 'Resumen: caída total de pagos por agotamiento del pool de conexiones (fuga en v2.3). Acción: corregir la fuga y alertar sobre el uso del pool.',
 'aprobado', true, '00000000-0000-0000-0000-000000000003', now() - interval '9 days', now() - interval '9 days'),
('00000000-0000-0000-0003-000000000003', '00000000-0000-0000-0002-000000000003',
 'Borrador: el índice de búsqueda quedó desactualizado porque el deploy omitió el reindexado.',
 'borrador', true, NULL, NULL, now() - interval '3 days' + interval '130 minutes'),
('00000000-0000-0000-0003-000000000009', '00000000-0000-0000-0002-000000000009',
 'Borrador: la autenticación cayó por un certificado vencido del proveedor de identidad.',
 'borrador', true, NULL, NULL, now() - interval '1 day' + interval '80 minutes')
ON CONFLICT DO NOTHING;

COMMIT;
