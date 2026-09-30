# Feature: resolve-open-questions

**Objetivo:** cerrar PA-01..PA-14 de `docs/domain.md` y propagar las decisiones a los specs.
**Por qué:** el usuario delegó explícitamente al agente la resolución de las 14 preguntas abiertas (2026-09-29).
**Alcance autorizado:** `docs/domain.md`, `specs/UC-*.md`, `docs/bitacora.md`. Solo documentación, sin código.
**TDD:** no aplica. **Checks:** sin `PA-xx` pendientes en specs; toda BR citada existe; 3–6 criterios por UC.
**Rama:** `docs/resolve-open-questions`.

## Tareas
- [x] T1 Decisiones en `docs/domain.md` — ruta: inline (1 archivo, decisiones del parent)
- [x] T2 Specs + entrada de bitácora — ruta: delegated (trigger: 2+ archivos no triviales)
- [x] T3 Verificación cruzada y commit

## Progreso / evidencia
- domain.md: PA-01..PA-14 resueltas e incorporadas a BR-01..BR-18; nueva BR-19; sección 6 con la tabla de decisiones. Dos huecos hallados por el writer resueltos (evento al regenerar postmortem; no quitar rol `oncall` a un on-call asignado).
- Spot check del parent: `rg 'PA-[0-9]+' specs` vacío; 6 criterios en cada UC; BR citadas = BR-01..BR-19, todas existen.
- Pendiente abierto (no bloqueante): ningún UC tiene como objetivo principal el cambio de severidad; quedó en UC-05.4/05.5.
- `make verify` falla por falta de `backend/go.mod` (esperable en diseño; no se debilitó).
