# Feature: uc-01-registro-login

**Objetivo:** implementar UC-01 (registro, login, rutas protegidas, cambio de rol por admin, primer admin desde entorno).
**Por qué:** base de los demás casos de uso (BR-10, BR-12, BR-19). El usuario pidió implementar los UC uno por vez, uno por rama `feature/uc-XX`.
**Fuentes:** `specs/UC-01-registro-login.md`, `docs/domain.md` (BR-10, BR-12, BR-19), `backend/AGENTS.md`.
**Alcance autorizado:** `backend/internal/user`, `backend/internal/auth`, `backend/migrations`, `backend/cmd/api/main.go`, `backend/go.mod`/`go.sum` (solo `pgx/v5` y `x/crypto`), `frontend/src`, `e2e/uc-01.spec.ts`, `docs/bitacora.md`, este documento.
**Fuera de alcance:** recuperación de contraseña, OAuth/SSO, email/2FA, regla BR-19 de no quitar `oncall` a on-call de servicio (depende de UC-04).
**TDD:** exigido para reglas de negocio (`backend/AGENTS.md`); runner: `go test ./...` desde `backend/`, vía `make verify`.
**Rama:** `feature/uc-01-registro-login` (desde `main`).
**Entrega:** estrategia `ask-on-risk`; si pasa de ~400 líneas, un commit/PR por slice (T1-T2 backend, T3 frontend).

## Tareas
- [x] T1 Service de `user` y `auth` con repos fake: tests BR-19 (registro, email único, primer admin), BR-12 (solo admin cambia rol), login, middleware BR-10. Test primero.
- [x] T2 Migración `0001`, repositorios SQL (pgx) contra Postgres real, handlers HTTP, wiring en `main.go` con config por entorno.
- [ ] T3 Frontend: registro, login, guardia de ruta; `e2e/uc-01.spec.ts` (un test por criterio 01.1–01.6).
- [ ] T4 `make verify` verde, validación Playwright, revisión con `reviewer`, entrada en `docs/bitacora.md`, commits `feat(UC-01): ...`.

Ruta: T1/T2/T3 delegated (un writer a la vez, 2+ archivos no triviales); T4 parent + `reviewer`.

## Progreso / evidencia
- T1: writer (Sonnet) reportó RED (vet "no non-test Go files") y GREEN; el parent re-verificó gofmt/vet/test en verde y los 7 tests nombrados por BR.
- T2: writer (Sonnet) dejó SQL/handlers/wiring sin poder probar Postgres. El parent levantó un Postgres descartable (contenedor `ir-uc01-test`, puerto 55432, contraseña aleatoria) y corrió los tests SQL: la primera corrida falló por una carrera real (`CREATE TABLE schema_migrations` fuera del advisory lock); se movió dentro del lock y pasó. Servidor real + curl: 01.1 201 sin password_hash, 01.2 409, 01.3 401 sin cookie / 200 con cookie, 01.4 401, 01.5 403 y rol intacto / 200 y rol `oncall`, 01.6 admin de entorno puede loguear y un reinicio no crea otro (1 admin).
- Review nativa de T1 aprobada (4 lentes, sin bloqueantes). Hallazgos informativos pendientes de decidir en T4: login timing, 500 sin log/test en RequireAuth, nombre fijo "Admin".
- Deuda de arquitectura (aceptada por el usuario): T3 usa un proxy de Vite (`/api`) en vez de CORS; solo cubre desarrollo. Cómo se sirven frontend y API en producción (mismo origen detrás de un reverse proxy, o CORS + cookies `SameSite=None; Secure`) está sin decidir y requiere un trabajo de arquitectura posterior (`docs/ARCHITECTURE.md` aún no existe).
- Plan aprobado por el usuario (2026-09-30). Sesión opaca en tabla `sessions`, cookie HttpOnly; deps nuevas `pgx/v5` y `x/crypto/bcrypt`.
