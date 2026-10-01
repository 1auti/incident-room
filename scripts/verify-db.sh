#!/usr/bin/env bash
# Corre los tests de backend contra un Postgres descartable (ver lib/disposable-pg.sh).
# Los tests SQL hacen TRUNCATE, por eso nunca se apuntan a una base real.
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
# shellcheck source=lib/disposable-pg.sh
source "$root/scripts/lib/disposable-pg.sh"
disposable_pg_start verify-db

cd "$root/backend"
# -p 1: los paquetes comparten una sola base y sus tests hacen TRUNCATE; en paralelo se pisan.
go test -p 1 -count=1 ./...
