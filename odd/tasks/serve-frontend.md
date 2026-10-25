# serve-frontend

**Objetivo:** en producción el backend Go sirve `frontend/dist` (decisión D7 de `docs/ARCHITECTURE.md`, aprobada el 2026-10-20).
**Diseño:** `httpapi.Static(dir, api)` envuelve el handler de la API; `/api/...` pasa a la API, un archivo existente se sirve tal cual, el resto cae en `index.html`. `STATIC_DIR` opcional en `main.go` (sin él, desarrollo igual que hoy). Desde disco, no `go:embed`: `embed` no puede leer `../frontend/dist` desde el módulo `backend`.
**TDD:** no determinado (sin configuración explícita); se escribe test del handler primero por criterio propio. **Checks:** `make verify`.
**Ruta:** delegada (escritor único: 2+ archivos no triviales). Sin migraciones ni cambios de dominio.

- [x] T1 `backend/internal/httpapi/static.go` + `static_test.go` (archivo existente, ruta desconocida → index.html, `/api/x` inexistente → 404 sin index.html, path traversal)
- [x] T2 cablear `STATIC_DIR` en `backend/cmd/api/main.go`
- [x] T3 docs: D7 → implementado en `docs/ARCHITECTURE.md`, `README.md` (variable y uso), bitácora (bitácora la registra el usuario)

Progreso: T1-T3 hechos. Mejoras del reviewer: tests nuevos primero, RED observado (14 casos fallaban: 404 de assets faltantes, Cache-Control), luego GREEN con `go test ./internal/httpapi -run Static`. Validación de `STATIC_DIR` exige `index.html` regular (sin test: `run()` necesita Postgres). Pendiente: bitácora y commit.
