# UC-04 — Asignación y escalado al on-call del servicio

**Actor:** On-call · Admin
**Prioridad:** secundario  (núcleo = uno de los 7 que no pueden fallar en la demo)

## Objetivo
Asignar un on-call a cada servicio para que reciba y reconozca sus incidentes, y escalar automáticamente a los administradores cuando vence el SLA de reconocimiento sin acuse.

## Reglas de negocio
- BR-03: plazo de reconocimiento por severidad (SEV1 = 5 min, SEV2 = 15 min, SEV3 = 60 min), medido desde `declared_at`.
- BR-04: si el incidente sigue en `declarado` al vencer el plazo, se escala una sola vez a los `admin`, se fija `escalated_at` y se agrega un evento `escalado` con `author_id = null`.
- BR-05: el vencimiento se evalúa con el reloj inyectable.
- BR-09: el escalado y la asignación generan eventos append-only en la timeline.
- BR-11: el on-call solo reconoce incidentes de sus servicios.
- BR-12: solo `admin` asigna el on-call de un servicio.
- BR-18: el cumplimiento de SLA usa `acknowledged_at` frente al plazo de BR-03.

## Criterios de aceptación
- **UC-04.1** Dado un servicio y un usuario con rol `oncall`, cuando un `admin` lo asigna como `oncall_user_id` del servicio, entonces los incidentes nuevos de ese servicio se asignan a ese usuario; si lo intenta un `ingeniero` o un `oncall`, entonces la acción es prohibida y el servicio no cambia (BR-12, BR-11, PA-03).
- **UC-04.2** Dado un incidente SEV2 declarado a las 10:00 que sigue en `declarado`, cuando el reloj llega a las 10:15:01, entonces se fija `escalated_at` y se agrega un único evento `escalado` con `author_id` vacío, visible para los `admin` (BR-03, BR-04, BR-05, PA-08).
- **UC-04.3** Dado un incidente SEV2 declarado a las 10:00, cuando el on-call del servicio lo reconoce a las 10:14:59, entonces pasa a `reconocido` y, al superarse el plazo, no se agrega ningún evento `escalado` (BR-04, BR-05).
- **UC-04.4** Dado un incidente que ya fue escalado, cuando se vuelve a evaluar el vencimiento, entonces no se agrega un segundo evento `escalado` (BR-04).
- **UC-04.5** Dado un `oncall` que no es el on-call del servicio, cuando intenta reconocer un incidente de ese servicio, entonces la acción es prohibida y el incidente sigue en `declarado` (BR-11).
- **UC-04.6** Dado un incidente SEV3 declarado a las 10:00 y cuya severidad no cambió, cuando el reloj marca 10:59:59, entonces no hay escalado, y a las 11:00:01 sí lo hay (BR-03, BR-04, PA-04).

## Fuera de alcance
- Rotaciones o turnos de guardia y más de un on-call por servicio (PA-03).
- Notificaciones por email, SMS o mensajería; el escalado se registra como evento y en la UI (PA-08).
- Escalado en niveles sucesivos o reasignación automática a otro on-call.
- Comportamiento definitivo cuando el servicio no tiene on-call (PA-08).

## Verificación
- Unit: `TestBR03_PlazoSLAPorSeveridad`, `TestBR04_EscalaUnaSolaVezAlVencer`, `TestBR04_ReconocidoATiempoNoEscala`, `TestBR11_OncallAjenoNoReconoce`, `TestBR12_SoloAdminAsignaOncall` en `backend/internal/incident/...` y `backend/internal/service/...` (reloj inyectable, BR-05)
- E2E: `e2e/uc-04.spec.ts` (un test por criterio de aceptación)
