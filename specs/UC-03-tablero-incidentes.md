# UC-03 — Tablero de incidentes activos con filtros

**Actor:** Ingeniero · On-call · Admin
**Prioridad:** núcleo  (núcleo = uno de los 7 que no pueden fallar en la demo)

## Objetivo
Ver los incidentes activos y filtrarlos por severidad, servicio y estado para decidir dónde actuar primero.

## Reglas de negocio
- BR-15: un incidente está activo si `state ≠ cerrado`; el tablero muestra solo activos y admite filtros por severidad, servicio y estado.
- BR-10: sin sesión válida no se accede al tablero.

## Criterios de aceptación
- **UC-03.1** Dado un incidente `resuelto` y otro `cerrado`, cuando un usuario autenticado abre el tablero, entonces aparece el `resuelto` y no aparece el `cerrado` (BR-15, PA-11).
- **UC-03.2** Dado incidentes activos de distintas severidades, cuando el usuario filtra por SEV1, entonces solo se listan incidentes con `severity = SEV1` (BR-15).
- **UC-03.3** Dado incidentes activos de varios servicios y estados, cuando el usuario filtra por un servicio y un estado a la vez, entonces solo se listan los incidentes que cumplen ambos filtros (BR-15).
- **UC-03.4** Dado un filtro sin coincidencias, cuando el usuario lo aplica, entonces el tablero muestra un estado vacío y no un error.
- **UC-03.5** Dado un usuario con rol `ingeniero`, cuando abre el tablero, entonces ve los incidentes activos de todos los servicios (BR-10, PA-13).
- **UC-03.6** Dado un visitante sin sesión, cuando solicita el tablero, entonces el acceso es denegado (BR-10).

## Fuera de alcance
- Listar incidentes cerrados o histórico.
- Acciones sobre el incidente desde el tablero (transiciones, notas).
- Restricción de visibilidad por rol (pendiente, PA-13).
- Actualización en tiempo real por WebSocket o polling.

## Verificación
- Unit: `TestBR15_ActivoEsTodoLoNoCerrado`, `TestBR15_FiltrosPorSeveridadServicioYEstado` en `backend/internal/incident/...`
- E2E: `e2e/uc-03.spec.ts` (un test por criterio de aceptación)
