# UC-08 — Postmortem asistido

**Actor:** Admin
**Prioridad:** secundario  (núcleo = uno de los 7 que no pueden fallar en la demo)

## Objetivo
Generar con un LLM un borrador de postmortem a partir de la timeline del incidente, editarlo y aprobarlo explícitamente.

## Reglas de negocio
- BR-13: el postmortem se genera como `borrador` por un LLM desde la timeline, es editable mientras sea `borrador`, se aprueba explícitamente y hay a lo sumo uno por incidente.
- BR-07: un postmortem `aprobado` habilita el cierre de un incidente SEV1.
- BR-09: la generación y la aprobación agregan eventos `postmortem` a la timeline.
- BR-10: cada acción exige el rol indicado en la matriz de permisos.
- BR-05: `approved_at` y los instantes de los eventos provienen del reloj inyectable.

## Criterios de aceptación
- **UC-08.1** Dado un incidente con eventos en su timeline, cuando un `admin` genera el postmortem, entonces se crea con `status = borrador`, `generated_by_llm = true`, contenido que cita los eventos de la timeline y un evento `postmortem` de generación (BR-13, BR-09, PA-07).
- **UC-08.2** Dado un incidente que ya tiene postmortem, cuando se intenta generar otro, entonces no se crea un segundo postmortem y el incidente conserva uno solo (BR-13).
- **UC-08.3** Dado un postmortem en `borrador`, cuando un `admin` edita su contenido, entonces se guarda el nuevo texto, cambia `updated_at` y el estado sigue `borrador` (BR-13, PA-07).
- **UC-08.4** Dado un postmortem en `borrador`, cuando un `admin` lo aprueba, entonces pasa a `aprobado`, se fijan `approved_by` y `approved_at` y se agrega un evento `postmortem` de aprobación (BR-13, BR-09, BR-05).
- **UC-08.5** Dado un `ingeniero`, cuando intenta aprobar un postmortem, entonces la acción es prohibida y el postmortem sigue en `borrador` (BR-10, PA-07).
- **UC-08.6** Dado un incidente SEV1 `resuelto` cuyo postmortem fue aprobado, cuando se ejecuta el cierre (UC-06), entonces el incidente pasa a `cerrado` (BR-07).

## Fuera de alcance
- Regeneración del borrador con el LLM sobre un postmortem existente.
- Edición de un postmortem ya aprobado y su inmutabilidad (pendiente, PA-07).
- Definición de qué roles distintos de `admin` pueden generar, editar o aprobar (PA-07).
- Exportación a PDF u otros formatos y plantillas configurables.

## Verificación
- Unit: `TestBR13_GeneraBorradorDesdeTimeline`, `TestBR13_UnPostmortemPorIncidente`, `TestBR13_EditableSoloEnBorrador`, `TestBR13_AprobarFijaAutorYFecha`, `TestBR07_PostmortemAprobadoHabilitaCierreSEV1` en `backend/internal/postmortem/...`. El LLM se reemplaza por un doble (stub/fake) en los tests unitarios.
- E2E: `e2e/uc-08.spec.ts` (un test por criterio de aceptación)
