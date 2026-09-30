# UC-07 — Gestión de runbooks por servicio

**Actor:** Admin
**Prioridad:** núcleo  (núcleo = uno de los 7 que no pueden fallar en la demo)

## Objetivo
Mantener los runbooks de cada servicio (alta, edición y baja) para que el equipo y el copiloto dispongan de procedimientos actualizados.

## Reglas de negocio
- BR-12: solo `admin` da de alta, edita y da de baja runbooks; la baja es física y elimina el embedding, por lo que el copiloto deja de recuperar el runbook.
- BR-14: las consultas del copiloto (RAG) usan solo los runbooks existentes.
- BR-10: cada acción exige el rol indicado en la matriz de permisos; el acceso denegado no produce efectos; todo usuario autenticado puede leer los runbooks.

## Criterios de aceptación
- **UC-07.1** Dado un servicio existente, cuando un `admin` crea un runbook con título y contenido, entonces queda asociado al servicio y aparece en el listado de runbooks de ese servicio (BR-12).
- **UC-07.2** Dado un runbook existente, cuando un `admin` edita su contenido, entonces se guarda el nuevo contenido, cambia `updated_at` y se recalcula su embedding (BR-12).
- **UC-07.3** Dado un `ingeniero` o un `oncall`, cuando intenta crear, editar o eliminar un runbook, entonces la acción es prohibida y el runbook no cambia (BR-12, BR-10).
- **UC-07.4** Dado runbooks de varios servicios, cuando cualquier usuario autenticado consulta los runbooks de un servicio, entonces ve solo los de ese servicio (BR-10, BR-12).
- **UC-07.5** Dado un runbook existente, cuando un `admin` lo da de baja, entonces deja de aparecer en el listado del servicio y el copiloto ya no lo recupera ni lo cita al consultarle sobre su tema (BR-12, BR-14).
- **UC-07.6** Dado un `admin`, cuando crea un runbook sin título o sin contenido, entonces recibe un error de validación y no se crea el runbook.

## Fuera de alcance
- Versionado o historial de cambios de un runbook.
- Baja lógica o restauración de un runbook eliminado: la baja es física (BR-12).
- Ejecución automatizada de pasos del runbook.
- Consulta conversacional de runbooks (UC-10).

## Verificación
- Unit: `TestBR12_SoloAdminGestionaRunbooks`, `TestBR12_TodoAutenticadoLeeRunbooks`, `TestBR10_AccesoDenegadoSinEfectos`, `TestRunbook_EditarRecalculaEmbedding`, `TestBR12_BajaEliminaRunbookYEmbedding` en `backend/internal/runbook/...`
- E2E: `e2e/uc-07.spec.ts` (un test por criterio de aceptación)
