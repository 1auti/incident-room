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
- [ ] T2 Migración `0001`, repositorios SQL (pgx) contra Postgres real, handlers HTTP, wiring en `main.go` con config por entorno.
- [ ] T3 Frontend: registro, login, guardia de ruta; `e2e/uc-01.spec.ts` (un test por criterio 01.1–01.6).
- [ ] T4 `make verify` verde, validación Playwright, revisión con `reviewer`, entrada en `docs/bitacora.md`, commits `feat(UC-01): ...`.

Ruta: T1/T2/T3 delegated (un writer a la vez, 2+ archivos no triviales); T4 parent + `reviewer`.

## Progreso / evidencia
- T1: writer (Sonnet) reportó RED (vet "no non-test Go files") y GREEN; el parent re-verificó gofmt/vet/test en verde y los 7 tests nombrados por BR.
- Plan aprobado por el usuario (2026-09-30). Sesión opaca en tabla `sessions`, cookie HttpOnly; deps nuevas `pgx/v5` y `x/crypto/bcrypt`.
