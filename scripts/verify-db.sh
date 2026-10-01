#!/usr/bin/env bash
# Corre los tests de backend contra un Postgres descartable: contenedor propio,
# puerto libre y contraseña aleatoria. No usa .env ni toca la base de desarrollo.
# Los tests SQL hacen TRUNCATE, por eso nunca se apuntan a una base real.
set -euo pipefail

for tool in docker openssl; do
  command -v "$tool" >/dev/null || { echo "verify-db: falta '$tool' en el PATH" >&2; exit 1; }
done

root="$(cd "$(dirname "$0")/.." && pwd)"
name="ir-verify-db-$$"
pw="$(openssl rand -hex 16)"
ready_attempts=60

cleanup() { docker rm -f "$name" >/dev/null 2>&1 || true; }
trap cleanup EXIT

docker run -d --rm --name "$name" \
  -e POSTGRES_PASSWORD="$pw" -e POSTGRES_DB=ir_test \
  -p 127.0.0.1::5432 pgvector/pgvector:pg17 >/dev/null

# Durante initdb la imagen levanta un servidor temporal que solo escucha en el socket unix.
# Probar por TCP (-h 127.0.0.1) solo responde con el servidor definitivo, el mismo que usan los tests.
ready=0
for _ in $(seq 1 "$ready_attempts"); do
  if docker exec -e PGPASSWORD="$pw" "$name" \
    psql -h 127.0.0.1 -U postgres -d ir_test -tAc 'select 1' >/dev/null 2>&1; then
    ready=1
    break
  fi
  sleep 1
done
if [ "$ready" != 1 ]; then
  echo "verify-db: Postgres no arrancó a tiempo; logs del contenedor:" >&2
  docker logs --tail 40 "$name" >&2 || true
  exit 1
fi

port="$(docker port "$name" 5432/tcp | head -n1 | sed 's/.*://')"
case "$port" in
  '' | *[!0-9]*) echo "verify-db: no se pudo obtener el puerto publicado ('$port')" >&2; exit 1 ;;
esac
export DATABASE_URL="postgres://postgres:${pw}@127.0.0.1:${port}/ir_test?sslmode=disable"

cd "$root/backend"
# -p 1: los paquetes comparten una sola base y sus tests hacen TRUNCATE; en paralelo se pisan.
go test -p 1 -count=1 ./...
