# UC-05 — Timeline del incidente

**Actor:** Ingeniero · On-call · Admin
**Prioridad:** núcleo  (núcleo = uno de los 7 que no pueden fallar en la demo)

## Objetivo
Consultar la historia cronológica e inmutable de un incidente, agregar notas que documentan las acciones tomadas y cambiar la severidad mientras el incidente no esté resuelto.

## Reglas de negocio
- BR-09: los eventos solo se insertan; ninguna operación los modifica ni elimina; se listan en orden cronológico por `occurred_at`.
- BR-02: después de declarar, solo el on-call del servicio y el `admin` cambian la severidad, y solo en `declarado`, `reconocido` o `mitigando`; desde `resuelto` queda fija. Cada cambio genera un evento `cambio_severidad`.
- BR-04: el escalado por SLA genera un evento `escalado` sin autor.
- BR-05: `occurred_at` proviene del reloj inyectable.
- BR-10: cada acción exige el rol indicado en la matriz de permisos; todo usuario autenticado ve las timelines y agrega notas.
- BR-11: el on-call solo cambia la severidad de incidentes de sus servicios.

## Criterios de aceptación
- **UC-05.1** Dado un incidente con varios eventos, incluido un `escalado` generado por el sistema, cuando un usuario autenticado abre su timeline, entonces los eventos se muestran en orden cronológico ascendente por `occurred_at` con tipo, autor (el `escalado` identificado como generado por el sistema) y contenido (BR-09, BR-04, BR-10).
- **UC-05.2** Dado un incidente, cuando un usuario autenticado (`ingeniero`, `oncall` o `admin`) agrega una nota, entonces se inserta un evento `nota` con su autor y `occurred_at` del reloj inyectable, y aparece al final de la timeline; si la nota tenía un error y el usuario agrega otra que la corrige, la original permanece y ambas se muestran en orden (BR-09, BR-05, BR-10).
- **UC-05.3** Dado un evento existente, cuando se intenta modificarlo (`PUT`) o eliminarlo (`DELETE`) por la API, entonces la respuesta es 405 o 403 y el evento permanece intacto (BR-09).
- **UC-05.4** Dado un incidente `mitigando` con severidad SEV2, cuando el on-call de su servicio o un `admin` cambia la severidad a SEV1, entonces `severity` es SEV1 y la timeline muestra un evento `cambio_severidad` con `{"from":"SEV2","to":"SEV1"}` y su autor (BR-02, BR-11).
- **UC-05.5** Dado un `ingeniero` (por ejemplo quien declaró el incidente), un `oncall` que no es el del servicio, o un `admin` sobre un incidente SEV1 `resuelto`, cuando intentan cambiar la severidad (por ejemplo bajar el SEV1 a SEV2), entonces la acción es prohibida o rechazada, la severidad no cambia y no se agrega ningún evento (BR-02, BR-11, BR-10).
- **UC-05.6** Dado un visitante sin sesión, cuando solicita la timeline, intenta agregar una nota o cambiar la severidad, entonces el acceso es denegado y no se crea ningún evento (BR-10).

## Fuera de alcance
- Edición o borrado de eventos, incluso por `admin`.
- Adjuntos, imágenes o archivos en las notas.
- Menciones, comentarios anidados y notificaciones.
- Exportación de la timeline.

## Verificación
- Unit: `TestBR09_TimelineSoloInserta`, `TestBR09_OrdenCronologico`, `TestBR05_NotaUsaRelojInyectable`, `TestBR10_TimelineRequiereSesion`, `TestBR02_CambioSeveridadSoloOncallOAdminHastaMitigando`, `TestBR02_SeveridadFijaDesdeResuelto`, `TestBR11_OncallAjenoNoCambiaSeveridad` en `backend/internal/timeline/...`
- E2E: `e2e/uc-05.spec.ts` (un test por criterio de aceptación)
