#!/usr/bin/env bash
# Corre los tests de backend contra un Postgres descartable (ver lib/disposable-pg.sh).
# Los tests SQL leen TEST_DATABASE_URL (nunca DATABASE_URL) y cada test usa su propio schema.
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
# shellcheck source=lib/disposable-pg.sh
source "$root/scripts/lib/disposable-pg.sh"
disposable_pg_start verify-db
export TEST_DATABASE_URL="$DATABASE_URL"
unset DATABASE_URL

cd "$root/backend"
go test -count=1 ./...
