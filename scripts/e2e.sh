#!/usr/bin/env bash
# Corre la suite E2E de Playwright contra backend y frontend reales levantados por la config,
# con un Postgres descartable. No usa .env; las credenciales del admin son aleatorias.
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
# shellcheck source=lib/disposable-pg.sh
source "$root/scripts/lib/disposable-pg.sh"
disposable_pg_start e2e

suffix="$(openssl rand -hex 4)"
export ADMIN_EMAIL="admin-${suffix}@e2e.test"
export ADMIN_PASSWORD="$(openssl rand -hex 16)"
export E2E_API_PORT="${E2E_API_PORT:-18080}"
export E2E_WEB_PORT="${E2E_WEB_PORT:-15173}"

# El frontend se instala aquí porque su dev server lo levanta Playwright.
(cd "$root/frontend" && npm ci)
cd "$root/e2e"
npm ci
npx playwright test
