#!/bin/sh
# Crea el rol de solo lectura del MCP reutilizando db/dev/agent-readonly.sql.
set -eu

psql -v ON_ERROR_STOP=1 -v ro_password="$AGENT_RO_PASSWORD" \
  --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" \
  -f /db/agent-readonly.sql
