#!/usr/bin/env bash
# Corre los tests de backend contra un Postgres descartable: contenedor propio,
# puerto libre y contraseña aleatoria. No usa .env ni toca la base de desarrollo.
# Los tests SQL hacen TRUNCATE, por eso nunca se apuntan a una base real.
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
name="ir-verify-db-$$"
pw="$(openssl rand -hex 16)"

cleanup() { docker rm -f "$name" >/dev/null 2>&1 || true; }
trap cleanup EXIT

docker run -d --rm --name "$name" \
  -e POSTGRES_PASSWORD="$pw" -e POSTGRES_DB=ir_test \
  -p 127.0.0.1::5432 pgvector/pgvector:pg17 >/dev/null

for _ in $(seq 1 60); do
  # pg_isready responde antes del reinicio de initdb; probar una consulta real evita esa ventana.
  if docker exec "$name" psql -U postgres -d ir_test -tAc 'select 1' >/dev/null 2>&1; then
    ready=1
    break
  fi
  sleep 1
done
[ "${ready:-0}" = 1 ] || { echo "verify-db: Postgres no arrancó a tiempo" >&2; exit 1; }

port="$(docker port "$name" 5432/tcp | head -n1 | sed 's/.*://')"
export DATABASE_URL="postgres://postgres:${pw}@127.0.0.1:${port}/ir_test?sslmode=disable"

cd "$root/backend"
# -p 1: los paquetes comparten una sola base y sus tests hacen TRUNCATE; en paralelo se pisan.
go test -p 1 -count=1 ./...
