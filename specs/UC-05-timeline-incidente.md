# UC-05 — Timeline del incidente

**Actor:** Ingeniero · On-call · Admin
**Prioridad:** núcleo  (núcleo = uno de los 7 que no pueden fallar en la demo)

## Objetivo
Consultar la historia cronológica e inmutable de un incidente y agregar notas que documentan las acciones tomadas.

## Reglas de negocio
- BR-09: los eventos solo se insertan; ninguna operación los modifica ni elimina; se listan en orden cronológico por `occurred_at`.
- BR-02: los cambios de severidad generan un evento `cambio_severidad`.
- BR-04: el escalado por SLA genera un evento `escalado` sin autor.
- BR-05: `occurred_at` proviene del reloj inyectable.
- BR-10: cada acción exige el rol indicado en la matriz de permisos.

## Criterios de aceptación
- **UC-05.1** Dado un incidente con varios eventos, cuando un usuario autenticado abre su timeline, entonces los eventos se muestran en orden cronológico ascendente por `occurred_at` con tipo, autor y contenido (BR-09).
- **UC-05.2** Dado un incidente, cuando un usuario autenticado agrega una nota, entonces se inserta un evento `nota` con su autor y `occurred_at` del reloj inyectable, y aparece al final de la timeline (BR-09, BR-05).
- **UC-05.3** Dado un evento existente, cuando se intenta modificarlo (`PUT`) o eliminarlo (`DELETE`) por la API, entonces la respuesta es 405 o 403 y el evento permanece intacto (BR-09).
- **UC-05.4** Dado una nota con un error, cuando el usuario agrega una nueva nota que la corrige, entonces la nota original permanece y ambas se muestran en orden (BR-09).
- **UC-05.5** Dado un incidente con cambio de severidad y escalado por SLA, cuando se abre la timeline, entonces se muestran el evento `cambio_severidad` con anterior, nueva y autor, y el evento `escalado` identificado como generado por el sistema (BR-02, BR-04).
- **UC-05.6** Dado un visitante sin sesión, cuando solicita la timeline o intenta agregar una nota, entonces el acceso es denegado y no se crea ningún evento (BR-10).

## Fuera de alcance
- Edición o borrado de eventos, incluso por `admin`.
- Adjuntos, imágenes o archivos en las notas.
- Menciones, comentarios anidados y notificaciones.
- Exportación de la timeline.

## Verificación
- Unit: `TestBR09_TimelineSoloInserta`, `TestBR09_OrdenCronologico`, `TestBR05_NotaUsaRelojInyectable`, `TestBR10_TimelineRequiereSesion` en `backend/internal/timeline/...`
- E2E: `e2e/uc-05.spec.ts` (un test por criterio de aceptación)
