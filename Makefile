# Única fuente de verificación: la usan los hooks de Claude Code, el pre-commit y la CI.
.PHONY: verify verify-backend verify-frontend verify-db e2e setup

verify: verify-backend verify-frontend

verify-backend:
	cd backend && go build ./... && go vet ./... && go test ./...

# El build de la plantilla de Vite incluye el typecheck (tsc -b).
verify-frontend:
	cd frontend && npm run lint && npm run build

# Complemento de verify, no lo reemplaza: corre los tests de backend (incluidos los SQL)
# contra un Postgres descartable. Requiere Docker; no forma parte de los hooks.
verify-db:
	bash scripts/verify-db.sh

# E2E de Playwright (Chromium) contra backend y frontend reales y un Postgres descartable.
# Requiere Docker y `npx playwright install chromium` una vez; no forma parte de verify.
e2e:
	bash scripts/e2e.sh

# Activa los hooks de git versionados en el repo.
setup:
	git config core.hooksPath .githooks
