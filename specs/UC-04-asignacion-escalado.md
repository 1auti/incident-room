# UC-04 — Asignación y escalado al on-call del servicio

**Actor:** On-call · Admin
**Prioridad:** secundario  (núcleo = uno de los 7 que no pueden fallar en la demo)

## Objetivo
Asignar un on-call a cada servicio para que reciba y reconozca sus incidentes, y escalar automáticamente a los administradores cuando vence el SLA de reconocimiento sin acuse.

## Reglas de negocio
- BR-03: plazo de reconocimiento por severidad vigente (SEV1 = 5 min, SEV2 = 15 min, SEV3 = 60 min), siempre medido desde `declared_at`.
- BR-04: si el incidente sigue en `declarado` al vencer el plazo, se escala una sola vez a todos los `admin`: se fija `escalated_at`, se agrega un evento `escalado` con `author_id = null` y el incidente se muestra con el indicador "Escalado" en el tablero y en su detalle. Sin email ni otros canales. Si el servicio no tiene on-call, el incidente queda sin asignar y escala por la misma regla.
- BR-05: el vencimiento se evalúa con el reloj inyectable.
- BR-09: el escalado y la asignación generan eventos append-only en la timeline.
- BR-11: cada servicio tiene como máximo un on-call, asignado por el admin y con rol `oncall`; el on-call solo reconoce incidentes de sus servicios; al cambiar el on-call, los incidentes activos pasan al nuevo on-call con un evento `asignacion`.
- BR-12: solo `admin` asigna el on-call de un servicio.
- BR-15: incidente activo = `state ≠ cerrado`.
- BR-18: el cumplimiento de SLA usa `acknowledged_at` frente al plazo de BR-03.

## Criterios de aceptación
- **UC-04.1** Dado un servicio y un usuario con rol `oncall`, cuando un `admin` lo asigna como `oncall_user_id` del servicio, entonces los incidentes nuevos de ese servicio se asignan a ese usuario; si el usuario asignado no tiene rol `oncall`, o si lo intenta un `ingeniero` o un `oncall`, entonces la acción es rechazada y el servicio no cambia (BR-12, BR-11).
- **UC-04.2** Dado un servicio con on-call Ana y dos incidentes activos (uno `resuelto` sin cerrar) más uno `cerrado`, cuando un `admin` cambia el on-call a Bruno, entonces los dos activos pasan a `assigned_to` = Bruno con un evento `asignacion` cada uno, y el `cerrado` no cambia (BR-11, BR-15, BR-09).
- **UC-04.3** Dado un incidente SEV2 declarado a las 10:00 que sigue en `declarado`, cuando el reloj llega a las 10:15:01, entonces se fija `escalated_at`, se agrega un único evento `escalado` con `author_id` vacío (una nueva evaluación no agrega otro) y el incidente muestra el indicador "Escalado" en el tablero y en su detalle; lo mismo ocurre si el servicio no tiene on-call y el incidente está sin asignar (BR-03, BR-04, BR-05).
- **UC-04.4** Dado un incidente SEV2 declarado a las 10:00, cuando el on-call del servicio lo reconoce a las 10:14:59, entonces pasa a `reconocido` y, al superarse el plazo, no se agrega ningún evento `escalado` ni aparece el indicador "Escalado" (BR-04, BR-05).
- **UC-04.5** Dado un incidente SEV3 declarado a las 10:00 y elevado a SEV1 a las 10:07 sin reconocer, cuando se evalúa el vencimiento, entonces el plazo es el de SEV1 medido desde `declared_at` (10:05), por lo que ya venció y se escala; con la severidad SEV3 sin cambios no hay escalado a las 10:59:59 y sí a las 11:00:01 (BR-03, BR-04).
- **UC-04.6** Dado un `oncall` que no es el on-call del servicio, cuando intenta reconocer un incidente de ese servicio, entonces la acción es prohibida y el incidente sigue en `declarado` (BR-11).

## Fuera de alcance
- Rotaciones o turnos de guardia y más de un on-call por servicio (BR-11).
- Notificaciones por email, SMS o mensajería; el escalado se registra como evento y se muestra con el indicador "Escalado" (BR-04).
- Escalado en niveles sucesivos o reasignación automática a otro on-call por un escalado; la única reasignación es la que ocurre cuando el admin cambia el on-call (BR-11).

## Verificación
- Unit: `TestBR03_PlazoSLAPorSeveridad`, `TestBR04_EscalaUnaSolaVezAlVencer`, `TestBR04_ReconocidoATiempoNoEscala`, `TestBR11_OncallAjenoNoReconoce`, `TestBR11_CambioOncallReasignaIncidentesActivos`, `TestBR11_SoloUsuarioOncallEsAsignable`, `TestBR03_PlazoUsaSeveridadVigente`, `TestBR04_ServicioSinOncallEscala`, `TestBR12_SoloAdminAsignaOncall` en `backend/internal/incident/...` y `backend/internal/service/...` (reloj inyectable, BR-05)
- E2E: `e2e/uc-04.spec.ts` (un test por criterio de aceptación)
