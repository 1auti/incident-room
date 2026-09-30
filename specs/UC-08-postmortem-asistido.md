# UC-08 — Postmortem asistido

**Actor:** On-call · Admin
**Prioridad:** secundario  (núcleo = uno de los 7 que no pueden fallar en la demo)

## Objetivo
Generar con un LLM un borrador de postmortem a partir de la timeline de un incidente resuelto, editarlo o regenerarlo y aprobarlo explícitamente (solo admin).

## Reglas de negocio
- BR-13: hay a lo sumo un postmortem por incidente, de cualquier severidad; se genera como `borrador` por un LLM desde la timeline solo con el incidente en `resuelto`; generar y editar: on-call del servicio o `admin`, mientras sea `borrador`; generar de nuevo reemplaza el borrador; aprobar: solo `admin`; un postmortem `aprobado` es inmutable.
- BR-07: un postmortem `aprobado` habilita el cierre de un incidente SEV1.
- BR-09: la generación y la aprobación agregan eventos `postmortem` a la timeline.
- BR-10: cada acción exige el rol indicado en la matriz de permisos.
- BR-11: el on-call solo trabaja el postmortem de incidentes de sus servicios.
- BR-05: `approved_at` y los instantes de los eventos provienen del reloj inyectable.

## Criterios de aceptación
- **UC-08.1** Dado un incidente `resuelto` con eventos en su timeline, cuando el on-call de su servicio o un `admin` genera el postmortem, entonces se crea con `status = borrador`, `generated_by_llm = true`, contenido que cita los eventos de la timeline y un evento `postmortem` de generación (BR-13, BR-09, BR-11).
- **UC-08.2** Dado un incidente que no está `resuelto` (por ejemplo `mitigando`), o un usuario `ingeniero`, cuando se intenta generar el postmortem, entonces la acción es rechazada o prohibida y no se crea postmortem ni evento (BR-13, BR-10).
- **UC-08.3** Dado un postmortem en `borrador`, cuando el on-call del servicio o un `admin` edita su contenido, entonces se guarda el nuevo texto, cambia `updated_at` y el estado sigue `borrador`; si lo genera de nuevo, el contenido del borrador se reemplaza y el incidente conserva un solo postmortem (BR-13).
- **UC-08.4** Dado un postmortem en `borrador`, cuando un `admin` lo aprueba, entonces pasa a `aprobado`, se fijan `approved_by` y `approved_at`, se agrega un evento `postmortem` de aprobación y, si el incidente es SEV1 `resuelto`, ya puede cerrarse (BR-13, BR-09, BR-05, BR-07).
- **UC-08.5** Dado un postmortem en `borrador`, cuando el on-call del servicio o un `ingeniero` intenta aprobarlo, entonces la acción es prohibida y el postmortem sigue en `borrador` (BR-13, BR-10).
- **UC-08.6** Dado un postmortem `aprobado`, cuando se intenta editarlo o generarlo de nuevo, entonces la acción es rechazada y el contenido no cambia (BR-13).

## Fuera de alcance
- Generar el postmortem de un incidente que no esté `resuelto`.
- Edición o regeneración de un postmortem `aprobado`: es inmutable (BR-13).
- Aprobación por roles distintos de `admin`.
- Exportación a PDF u otros formatos y plantillas configurables.

## Verificación
- Unit: `TestBR13_GeneraBorradorDesdeTimeline`, `TestBR13_SoloGeneraEnResuelto`, `TestBR13_UnPostmortemPorIncidente`, `TestBR13_RegenerarReemplazaBorrador`, `TestBR13_EditableSoloEnBorrador`, `TestBR13_AprobarSoloAdmin`, `TestBR13_AprobadoEsInmutable`, `TestBR13_AprobarFijaAutorYFecha`, `TestBR07_PostmortemAprobadoHabilitaCierreSEV1` en `backend/internal/postmortem/...`. El LLM se reemplaza por un doble (stub/fake) en los tests unitarios.
- E2E: `e2e/uc-08.spec.ts` (un test por criterio de aceptación)
