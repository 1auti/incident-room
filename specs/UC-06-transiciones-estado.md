# UC-06 — Transiciones de estado con reglas

**Actor:** On-call · Admin
**Prioridad:** núcleo  (núcleo = uno de los 7 que no pueden fallar en la demo)

## Objetivo
Llevar el incidente por `declarado → reconocido → mitigando → resuelto → cerrado` aplicando las reglas que impiden resolver sin causa raíz y cerrar un SEV1 sin postmortem aprobado.

## Reglas de negocio
- BR-05: los timestamps de transición provienen del reloj inyectable.
- BR-06: resolver (T3) exige causa raíz no vacía ni solo espacios.
- BR-07: cerrar (T4) un SEV1 exige un postmortem `aprobado`.
- BR-08: solo son válidas las transiciones T1–T4 por los roles indicados; cualquier otra se rechaza sin cambios ni eventos.
- BR-09: cada transición agrega un evento `cambio_estado` a la timeline.
- BR-10: cada acción exige el rol indicado en la matriz de permisos.
- BR-11: el on-call solo transiciona incidentes de sus servicios.

## Criterios de aceptación
- **UC-06.1** Dado un incidente `declarado` y el on-call de su servicio, cuando ejecuta T1 y luego T2, entonces el estado pasa a `reconocido` y luego a `mitigando`, `acknowledged_at` se fija en T1 y cada paso agrega un evento `cambio_estado` (BR-08, BR-09, BR-05).
- **UC-06.2** Dado un incidente `mitigando`, cuando se intenta resolver con la causa raíz vacía o solo con espacios, entonces se rechaza con un error de validación, el estado sigue `mitigando` y no se agrega evento (BR-06).
- **UC-06.3** Dado un incidente `mitigando`, cuando se resuelve con la causa raíz "Pool de conexiones agotado por fuga en v2.3", entonces pasa a `resuelto`, se guarda `root_cause`, se fija `resolved_at` y se agrega un evento `cambio_estado` (BR-06, BR-05, BR-09).
- **UC-06.4** Dado un incidente SEV1 `resuelto` sin postmortem o con postmortem en `borrador`, cuando se intenta cerrarlo, entonces se rechaza, el estado sigue `resuelto` y no se agrega evento (BR-07).
- **UC-06.5** Dado un incidente SEV1 `resuelto` con postmortem `aprobado`, o un incidente SEV2 `resuelto` sin postmortem, cuando se ejecuta T4, entonces pasa a `cerrado` y se fija `closed_at` (BR-07, PA-09).
- **UC-06.6** Dado un incidente, cuando se intenta una transición no listada (por ejemplo `declarado → resuelto` o un retroceso), o la ejecuta un `ingeniero` o un `oncall` que no es el del servicio, entonces se rechaza sin cambios ni eventos (BR-08, BR-10, BR-11, PA-02).

## Fuera de alcance
- Reapertura de incidentes y saltos de estado (PA-02).
- Generación y aprobación del postmortem (UC-08).
- Cambio de severidad (UC-02, UC-05; quién puede hacerlo, PA-05).
- Transiciones ejecutadas desde el copiloto: no existe herramienta de cierre (UC-10, BR-14).

## Verificación
- Unit: `TestBR06_ResolverRequiereCausaRaiz`, `TestBR07_CerrarSEV1RequierePostmortemAprobado`, `TestBR08_SoloTransicionesT1aT4`, `TestBR11_OncallSoloTransicionaSusServicios`, `TestBR05_TransicionesUsanRelojInyectable` en `backend/internal/incident/...`
- E2E: `e2e/uc-06.spec.ts` (un test por criterio de aceptación)
