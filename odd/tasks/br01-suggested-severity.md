# Feature: br01-suggested-severity

**Objetivo:** implementar BR-01 (severidad sugerida por criticidad × impacto) como función pura en el paquete `incident`, y usarla como primera prueba real de `/delegate` (Claude orquesta, opencode ejecuta).
**Por qué:** el usuario pidió "probemos delegate con una tarea chica" (2026-09-29). Cubre el criterio UC-02.1 y no depende de I/O, reloj ni base de datos.
**Fuentes:** `docs/domain.md` BR-01 (tabla 3×3), `specs/UC-02-declarar-incidente.md` UC-02.1, `backend/AGENTS.md`.
**Alcance autorizado:** nuevos `backend/internal/incident/severity.go` y `backend/internal/incident/severity_test.go` (test `TestBR01_SeveridadSugeridaPorCriticidadEImpacto`, como pide `specs/UC-02` línea 32); `docs/bitacora.md`; este documento.
**Corrección de plan (aprobada por el usuario):** el plan inicial ponía el código en `backend/internal/service` con el test `TestBR01_SeveridadSugerida`, lo que contradecía el spec (fuente de verdad n.º 1). El usuario eligió seguir el spec.
**Fuera de alcance:** handler, repository, BR-02 (elección manual y evento `cambio_severidad`), cualquier otra BR, dependencias nuevas, `Makefile`, config de opencode.
**TDD:** exigido por las reglas del proyecto (`backend/AGENTS.md`, `implement-uc.md`: test primero para reglas de negocio); runner: `go test ./...` desde `backend/`, vía `make verify`. **Rama:** `feature/br01-suggested-severity` (sale de `feature/opencode-executor`).
**Modelos:** exploración: Haiku (resumen textual del repo, solo lectura); plan/prompt del ejecutor: Opus (subagente de solo lectura, planifica desde el resumen de Haiku); ejecución: opencode `executor` (`opencode/big-pickle`); revisión: subagente `reviewer` con Sonnet; verificación de registro: parent.
**Entrega:** un commit de código `feat(UC-02): ...` (`f10c384`) más commits de documentación; PRs apilados sobre `main` (ver la sección "Entrega y PRs").

## Tareas
- [x] T1 Prompt del ejecutor redactado por Opus a partir del resumen de Haiku (sin escribir código).
- [x] T2 Ejecutar `opencode run --agent executor --format json "<prompt>"` con timeout largo.
- [x] T3 Verificación del parent: `git status`, `git diff`, comprobar test primero y alcance, `make verify`.
- [x] T4 Revisión con el subagente `reviewer` (Sonnet); resolver lo BLOQUEANTE (sin bloqueantes; 5 mejoras opcionales sin aplicar por el límite "solo lo que pide el spec").
- [x] T5 Entrada en `docs/bitacora.md` y commit `feat(UC-02): ...` (commit `f10c384`; esta vez el pre-commit `make verify` corrió de verdad y pasó).

Ruta: T1 delegated (Opus, solo lectura); T2 delegated al ejecutor externo; T3 y T5 del parent; T4 delegated (Sonnet).

## Progreso / evidencia
- T1: Haiku (Explore) entregó un resumen textual del repo; Opus (Plan) planificó solo desde ese resumen. Un primer Opus que releyó el repo por su cuenta se descartó porque el usuario pidió que se nutra de Haiku. Ese primer intento detectó que el spec pide `backend/internal/incident/` y `TestBR01_SeveridadSugeridaPorCriticidadEImpacto` (yo había puesto `service`): verificado en `specs/UC-02` línea 32, el usuario eligió seguir el spec.
- Decisiones del parent sobre las preguntas abiertas de Opus: identificadores en inglés (`CriticalityCritical`, `ImpactTotalOutage`, `SeveritySEV1`, `ErrInvalidSeverityInput`), un solo error centinela, sin stub para la fase roja (el rojo será un error de compilación).
- Línea base antes de delegar: `make verify` con exit 0; árbol solo con este documento sin trackear.
- T2: primer intento colgado más de 5 min sin crear sesión de opencode; su stdin era un socket abierto (los smoke tests en primer plano tenían `/dev/null`). Lo maté y relancé con `< /dev/null`; el segundo intento sí escribió archivos, lo que apoya la hipótesis de que `opencode run` en segundo plano espera entrada estándar, sin estar probada aislando la variable.
- T2/T3 evidencia: la secuencia de herramientas del ejecutor muestra test escrito → `go test` en rojo (`undefined: ...`) → `severity.go` → `go test` en verde → `make verify`. El Stop hook del proyecto falló con `undefined: Criticality` mientras solo existía el test, consistente con test primero.
- T3 verificación propia del parent: solo aparecen los 2 archivos esperados en `backend/internal/incident/` (`go.mod`/`go.sum` sin cambios, sin archivos extra); `gofmt -l` sin salida; `go test -v -run TestBR01` PASS en 16 casos; las 9 celdas coinciden con `docs/domain.md`; sin map a nivel de paquete, solo `errors` y `fmt`, error con `%w`; `make verify` exit 0.
- Observación sobre el ejecutor: el reporte pegado como "salida real" de `make verify` no coincide en dos tamaños gzip con la salida real (reescrito a mano). Por eso el reporte no se toma como prueba. También leyó `specs/UC-02.1.md` (no existe) antes de encontrar `specs/UC-02-declarar-incidente.md`.

## Entrega y PRs
- **Estrategia:** PRs apilados sobre `main`, uno por unidad de trabajo. El total acumulado es 344 líneas (37 + 137 + 170), bajo el presupuesto de 400; se separan por foco, no por tamaño.
- **Repositorio:** `origin` (`1auti/incident-room`). Las tres ramas se pushearon y los PRs se abrieron tras confirmación explícita del usuario.

| PR | Rama → base | Commits | Líneas |
|---|---|---|---|
| [#1](https://github.com/1auti/incident-room/pull/1) | `fix/mcp-postgres-pin` → `main` | `9ccaab7`, `239897e` | 37 |
| [#2](https://github.com/1auti/incident-room/pull/2) | `feature/opencode-executor` → `fix/mcp-postgres-pin` | `aefd4f1`, `ba9b302`, `366709b`, `af3ac44` | 137 |
| [#3](https://github.com/1auti/incident-room/pull/3) | `feature/br01-suggested-severity` → `feature/opencode-executor` | `f10c384`, `10bc4d3`, `89455cf` y este commit de registro | 170 más este commit |

- **Mergeo:** en orden 1, 2, 3 con "merge commit" o "rebase and merge"; con "squash" los PRs hijos quedarían con un diff contaminado.
- **Sin CI:** el repo no tiene `.github/`; la única verificación es la local (`make verify` y el pre-commit).
- **Revisión nativa:** el rango acumulado se revisó seis veces (todas aprobadas, sin bloqueantes). Cada commit crea un candidato nuevo y pide consentimiento otra vez.
- **Pendiente para el #2:** su versión de `odd/tasks/opencode-executor.md` conserva la frase falsa sobre el pre-commit; la corrige el #3.
