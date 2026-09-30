# UC-02 — Declarar un incidente con sugerencia automática de severidad

**Actor:** Ingeniero · On-call · Admin
**Prioridad:** núcleo  (núcleo = uno de los 7 que no pueden fallar en la demo)

## Objetivo
Declarar un incidente sobre un servicio indicando su impacto, recibiendo una severidad sugerida (SEV1-3) que el usuario puede aceptar o cambiar.

## Reglas de negocio
- BR-01: la severidad sugerida se calcula por criticidad del servicio × impacto y es la severidad inicial.
- BR-02: quien declara elige la severidad (por defecto la sugerida); declarar con una distinta agrega un evento `cambio_severidad`; `suggested_severity` nunca cambia.
- BR-05: el instante de declaración proviene del reloj inyectable.
- BR-09: la declaración genera un evento en la timeline, que es append-only.
- BR-10: cada acción exige el rol indicado en la matriz de permisos; `ingeniero`, `oncall` y `admin` pueden declarar.
- BR-11: al declarar, `assigned_to` toma el on-call del servicio, si existe.

## Criterios de aceptación
- **UC-02.1** Dado un servicio con criticidad `critica`, cuando el usuario elige el impacto `caida_total`, entonces el formulario o la API de sugerencia devuelve SEV1; con `estandar` y `degradacion` devuelve SEV3 (BR-01).
- **UC-02.2** Dado un servicio con on-call asignado, cuando un usuario autenticado (`ingeniero`, `oncall` o `admin`) declara el incidente aceptando la severidad sugerida, entonces se crea con `state = declarado`, `severity = suggested_severity`, `declared_at` igual al instante del reloj, `assigned_to` igual al on-call del servicio y un evento `declaracion` en la timeline (BR-01, BR-05, BR-09, BR-10, BR-11).
- **UC-02.3** Dado un servicio con severidad sugerida SEV2, cuando un `ingeniero` declara el incidente eligiendo SEV1, entonces `suggested_severity` permanece SEV2, `severity` es SEV1 y la timeline contiene un evento `cambio_severidad` con `{"from":"SEV2","to":"SEV1"}` y su autor (BR-02).
- **UC-02.4** Dado un servicio sin on-call asignado, cuando se declara un incidente, entonces se crea con `assigned_to` vacío (BR-11).
- **UC-02.5** Dado un usuario autenticado, cuando declara un incidente sin título, sin servicio o sin impacto, entonces recibe un error de validación y no se crea el incidente ni ningún evento.
- **UC-02.6** Dado un visitante sin sesión, cuando intenta declarar un incidente, entonces el acceso es denegado y no se crea el incidente (BR-10).

## Fuera de alcance
- Reconocer, asignar manualmente o transicionar el incidente (UC-04, UC-06).
- Cambio posterior de severidad tras declarar (UC-05, BR-02).
- Notificaciones por email o mensajería.
- Declaración de incidentes desde el copiloto (UC-10).

## Verificación
- Unit: `TestBR01_SeveridadSugeridaPorCriticidadEImpacto`, `TestBR02_CambioSeveridadRegistraEvento`, `TestBR05_DeclaredAtUsaRelojInyectable`, `TestBR11_DeclararAsignaOncallDelServicio`, `TestBR10_TodoRolPuedeDeclarar` en `backend/internal/incident/...`
- E2E: `e2e/uc-02.spec.ts` (un test por criterio de aceptación)
