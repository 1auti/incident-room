# Feature: design-specs

**Objetivo:** producir `specs/UC-01…UC-10` y la primera entrada de `docs/bitacora.md`, sobre `docs/domain.md` (aprobado por el usuario). Solo documentación.
**Alcance autorizado:** `specs/UC-*.md`, `docs/bitacora.md`. Sin código, sin git remoto.
**TDD:** no aplica (sin código). **Checks:** lectura cruzada BR-xx ↔ domain.md; 3–6 criterios por UC.
**Ruta:** delegated direct (un writer; trigger: 2+ archivos no triviales).

## Tareas
- [x] T1 Specs UC-01…UC-10 (un writer)
- [x] T2 Entrada inicial en `docs/bitacora.md`
- [x] T3 Verificación cruzada de BR-xx y conteo de criterios

## Progreso / evidencia
- Writer reportó `complete`; spot check del parent: `rg -c` da 6 criterios en cada UC; BR citadas = BR-01..BR-18 (todas existen en domain.md); UC-06 y bitácora releídos.
- Sin repositorio git local: sin commits (no aplica).
- Pendiente del usuario: confirmar/responder PA-01..PA-14; decidir cuándo subir al repo remoto.
