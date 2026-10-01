# UC-11 — Gestión de servicios

**Actor:** Admin (consulta: Ingeniero · On-call · Admin)
**Prioridad:** secundario  (núcleo = uno de los 7 que no pueden fallar en la demo; ya hay 7 núcleo. Es dependencia de UC-02, UC-04, UC-07 y UC-09.)

## Objetivo
Permitir que el administrador dé de alta, edite y dé de baja los servicios sobre los que se declaran incidentes, fijando su criticidad, y que todo usuario autenticado pueda consultarlos.

## Reglas de negocio
- BR-10: sin sesión válida no se accede a nada; el acceso denegado no produce efectos.
- BR-12: solo `admin` da de alta, edita y da de baja servicios.
- BR-01: la criticidad (`critica` | `importante` | `estandar`) la fija el admin y determina la severidad sugerida al declarar.
- BR-20: solo se da de baja un servicio sin incidentes ni runbooks; si los tiene, la baja se rechaza.

## Criterios de aceptación
- **UC-11.1** Dado un `admin` autenticado, cuando crea un servicio con un nombre no utilizado y una criticidad válida, entonces el servicio existe con esa criticidad y sin on-call (BR-12, BR-01).
- **UC-11.2** Dado un `admin` autenticado, cuando crea un servicio con nombre vacío, con un nombre ya utilizado o con una criticidad fuera de `critica`, `importante` y `estandar`, entonces recibe un error de validación y no se crea ningún servicio.
- **UC-11.3** Dado un servicio existente, cuando un `admin` cambia su nombre a uno no utilizado o su criticidad a un valor válido, entonces el cambio queda guardado; si el nombre ya lo usa otro servicio o la criticidad no es válida, entonces recibe un error de validación y el servicio no cambia (BR-12, BR-01).
- **UC-11.4** Dado un `ingeniero` o un `oncall`, cuando intenta crear, editar o dar de baja un servicio, entonces la acción es prohibida y no se produce ningún cambio; sin sesión, la respuesta es de no autenticado (BR-12, BR-10).
- **UC-11.5** Dado un servicio sin incidentes ni runbooks, cuando un `admin` lo da de baja, entonces el servicio deja de existir (BR-12, BR-20).
- **UC-11.6** Dado un servicio con al menos un incidente (en cualquier estado) o un runbook, cuando un `admin` intenta darlo de baja, entonces la baja es rechazada con un error y el servicio sigue existiendo sin cambios (BR-20).
- **UC-11.7** Dado cualquier usuario autenticado (`ingeniero`, `oncall` o `admin`), cuando consulta la lista de servicios, entonces ve todos con su nombre, criticidad y on-call; sin sesión, la respuesta es de no autenticado (BR-10).

## Fuera de alcance
- Asignar o cambiar el on-call de un servicio: es UC-04 (BR-11).
- Efectos de cambiar la criticidad sobre incidentes ya declarados: la severidad sugerida se calcula al declarar (BR-01) y no se recalcula.
- Baja lógica, servicios inactivos y baja en cascada (BR-20).
- Importación masiva o catálogo externo de servicios.

## Verificación
- Unit: `TestBR12_SoloAdminGestionaServicios`, `TestBR01_CriticidadValidaAlGestionarServicio`, `TestBR20_BajaRechazadaConIncidentes`, `TestBR20_BajaRechazadaConRunbooks`, `TestBR20_BajaSinDependencias` en `backend/internal/service/...`
- E2E: `e2e/uc-11.spec.ts` (un test por criterio de aceptación)
