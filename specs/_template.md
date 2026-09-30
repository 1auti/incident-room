# UC-XX — <nombre>

**Actor:** <Ingeniero | On-call | Admin>
**Prioridad:** <núcleo | secundario>  (núcleo = uno de los 7 que no pueden fallar en la demo)

## Objetivo
<Una o dos oraciones: qué logra el actor.>

## Reglas de negocio
- BR-xx: <referencia a docs/domain.md>

## Criterios de aceptación
- **UC-XX.1** Dado <contexto>, cuando <acción>, entonces <resultado observable>.
- **UC-XX.2** ...

## Fuera de alcance
- <Lo que explícitamente NO hace este caso.>

## Verificación
- Unit: `TestBRxx_...` en `backend/...`
- E2E: `e2e/uc-xx.spec.ts` (un test por criterio de aceptación)
