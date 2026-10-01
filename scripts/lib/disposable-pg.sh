#!/usr/bin/env bash
# Helper (se usa con `source`): levanta un Postgres descartable con pgvector.
# Contenedor propio, puerto libre y contraseña aleatoria; no usa .env ni toca la base de desarrollo.
# Uso: disposable_pg_start <prefijo>   -> exporta DATABASE_URL y registra un trap de limpieza.

disposable_pg_cleanup() {
  [ -n "${DISPOSABLE_PG_NAME:-}" ] && docker rm -f "$DISPOSABLE_PG_NAME" >/dev/null 2>&1 || true
}

disposable_pg_start() {
  local prefix="$1" pw port ready=0 ready_attempts=60

  for tool in docker openssl; do
    command -v "$tool" >/dev/null || { echo "$prefix: falta '$tool' en el PATH" >&2; exit 1; }
  done

  DISPOSABLE_PG_NAME="ir-${prefix}-$$"
  pw="$(openssl rand -hex 16)"
  trap disposable_pg_cleanup EXIT

  docker run -d --rm --name "$DISPOSABLE_PG_NAME" \
    -e POSTGRES_PASSWORD="$pw" -e POSTGRES_DB=ir_test \
    -p 127.0.0.1::5432 pgvector/pgvector:pg17 >/dev/null

  # Durante initdb la imagen levanta un servidor temporal que solo escucha en el socket unix.
  # Probar por TCP (-h 127.0.0.1) solo responde con el servidor definitivo, el mismo que usan los tests.
  for _ in $(seq 1 "$ready_attempts"); do
    if docker exec -e PGPASSWORD="$pw" "$DISPOSABLE_PG_NAME" \
      psql -h 127.0.0.1 -U postgres -d ir_test -tAc 'select 1' >/dev/null 2>&1; then
      ready=1
      break
    fi
    sleep 1
  done
  if [ "$ready" != 1 ]; then
    echo "$prefix: Postgres no arrancó a tiempo; logs del contenedor:" >&2
    docker logs --tail 40 "$DISPOSABLE_PG_NAME" >&2 || true
    exit 1
  fi

  port="$(docker port "$DISPOSABLE_PG_NAME" 5432/tcp | head -n1 | sed 's/.*://')"
  case "$port" in
    '' | *[!0-9]*) echo "$prefix: no se pudo obtener el puerto publicado ('$port')" >&2; exit 1 ;;
  esac
  export DATABASE_URL="postgres://postgres:${pw}@127.0.0.1:${port}/ir_test?sslmode=disable"
}
