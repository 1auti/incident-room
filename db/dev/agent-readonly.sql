-- Rol para el MCP de Postgres. La garantía de solo lectura vive en la base, no en el servidor MCP:
-- el rol no tiene permisos de escritura, así que ningún SQL que mande el agente puede modificar datos.
-- Uso: psql "$DATABASE_URL" -v ro_password='...' -f db/dev/agent-readonly.sql
-- Reemplazá incident_room (base) e incident_owner (rol dueño del esquema, el que corre las migraciones).

CREATE ROLE agent_ro LOGIN PASSWORD :'ro_password';
GRANT CONNECT ON DATABASE incident_room TO agent_ro;
GRANT USAGE ON SCHEMA public TO agent_ro;
GRANT SELECT ON ALL TABLES IN SCHEMA public TO agent_ro;

-- Las tablas que creen migraciones futuras también quedan legibles (y solo legibles).
ALTER DEFAULT PRIVILEGES FOR ROLE incident_owner IN SCHEMA public GRANT SELECT ON TABLES TO agent_ro;

-- Defensa adicional, no la garantía: evita consultas colgadas.
ALTER ROLE agent_ro SET statement_timeout = '5s';
