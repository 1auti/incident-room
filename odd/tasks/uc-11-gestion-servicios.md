# Feature: uc-11-gestion-servicios

**Objetivo:** implementar UC-11 (alta, edición, baja y consulta de servicios) como base de UC-02, UC-04, UC-07 y UC-09.
**Por qué:** ningún spec cubría crear servicios aunque BR-12 lo exige y UC-02 los necesita; el usuario aprobó escribir el spec UC-11 y la regla BR-20 (la baja se rechaza si hay incidentes o runbooks).
**Fuentes:** `specs/UC-11-gestion-servicios.md`, `docs/domain.md` (Service, BR-01, BR-10, BR-12, BR-20), `backend/AGENTS.md`, `frontend/AGENTS.md`.
**Alcance autorizado:** `backend/internal/service`, `backend/migrations/0002_services.sql`, `backend/internal/httpapi`, `backend/cmd/api/main.go`, `frontend/src/{api,features/services}`, `frontend/src/App.tsx`, `e2e/uc-11.spec.ts`, `docs/`.
**Fuera de alcance:** on-call (UC-04), crear las tablas de incidentes y runbooks (UC-02, UC-07), baja lógica o en cascada.
**TDD:** ON (reglas del proyecto); runner `go test` vía `make verify` y `make verify-db`, Playwright vía `make e2e`.
**Rama:** `feature/uc-11-gestion-servicios`, apilada sobre `feature/uc-01-registro-login` (PR #5 abierto).
**Decisiones del usuario:** pantalla mínima de administración; unicidad del nombre sin distinguir mayúsculas; baja rechazada si hay dependencias.

## Tareas
- [x] T1 Spec UC-11 y BR-20 en `docs/domain.md`.
- [x] T2 Backend: migración 0002, paquete `service`, rutas `/api/services`, logger inyectado en `httpapi` (tanda A, builder).
- [x] T3 Frontend y e2e: pantalla mínima y `e2e/uc-11.spec.ts` (tanda B, builder).
- [x] T4 Verificación, `reviewer`, bitácora.
- [ ] T5 Review nativa sobre el árbol limpio y entrega (push y PR a decisión del usuario).

Ruta: cadena writer (Haiku) → communicator (Haiku) → architect (Opus) → aprobación del usuario → builder (Sonnet) en dos tandas → reviewer (Sonnet). Omisión mía: este documento se creó al final, no antes de la primera escritura de código como pide el protocolo.

## Progreso / evidencia
- Tanda A: `make verify`, `make verify-db` (2 corridas) y `make e2e` verificados por el parent; los 4 tests SQL de servicios corrieron contra Postgres real. El builder no pudo aplicar la migración y los cambios de docs (denegación de permisos); el usuario autorizó que los aplicara el parent. `reviewer`: sin bloqueantes.
- Tanda B: `make verify` sin warnings y `make e2e` 13 passed (3 corridas). `reviewer` marcó un bloqueante real (UC-11.2 comparaba el conteo total en una base compartida con workers en paralelo; sus números de línea no existían pero el hallazgo sí): corregido por el parent. También se agregaron la rama de criticidad inválida de UC-11.3, los labels de edición y los asserts de que un ingeniero no ve acciones de admin.
- Obligación para UC-02 y UC-07: declarar `service_id UUID NOT NULL REFERENCES services (id)` sin `ON DELETE CASCADE` (la clave foránea es el respaldo de BR-20); completar `HasIncidents` y `HasRunbooks`; agregar el e2e de UC-11.6.
