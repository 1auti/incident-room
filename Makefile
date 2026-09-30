# Única fuente de verificación: la usan los hooks de Claude Code, el pre-commit y la CI.
.PHONY: verify verify-backend verify-frontend setup

verify: verify-backend verify-frontend

verify-backend:
	cd backend && go build ./... && go vet ./... && go test ./...

# El build de la plantilla de Vite incluye el typecheck (tsc -b).
verify-frontend:
	cd frontend && npm run lint && npm run build

# Activa los hooks de git versionados en el repo.
setup:
	git config core.hooksPath .githooks
