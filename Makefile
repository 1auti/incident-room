# Única fuente de verificación: la usan los hooks de Claude Code, el pre-commit y la CI.
.PHONY: verify verify-backend lint-backend verify-changed verify-frontend verify-db e2e dev setup tools

verify: verify-backend verify-frontend

verify-backend: lint-backend
	cd backend && go build ./... && go vet ./... && go test ./...

# golangci-lint v2 con el conjunto por defecto (ver backend/.golangci.yml).
lint-backend:
	cd backend && golangci-lint run ./...

# El build de la plantilla de Vite incluye el typecheck (tsc -b).
verify-frontend:
	cd frontend && npm run lint && npm run build

# Bucle rápido local: tests Go solo de los paquetes cambiados contra main (y sus dependientes) y
# `vitest --changed` en el frontend. No reemplaza a verify: los hooks y la CI siguen con `make verify`.
verify-changed:
	bash scripts/verify-changed.sh

# Complemento de verify, no lo reemplaza: corre los tests de backend (incluidos los SQL)
# contra un Postgres descartable. Requiere Docker; no forma parte de los hooks.
verify-db:
	bash scripts/verify-db.sh

# E2E de Playwright (Chromium) contra backend y frontend reales y un Postgres descartable.
# Requiere Docker y `npx playwright install chromium` una vez; no forma parte de verify.
e2e:
	bash scripts/e2e.sh

# Levanta la aplicación completa (Postgres descartable, backend y frontend) para usarla en el navegador.
# Requiere Docker. Ctrl-C la detiene; los datos no persisten. No forma parte de verify.
dev:
	bash scripts/dev.sh

# Activa los hooks de git versionados en el repo e instala/verifica las herramientas locales.
setup:
	git config core.hooksPath .githooks
	$(MAKE) tools

# Instala (go install) gofumpt, goimports y gopls; verifica ast-grep, typescript-language-server, etc.
tools:
	bash scripts/install-tools.sh
