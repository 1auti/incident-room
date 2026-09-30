# UC-10 — Copiloto conversacional

**Actor:** Ingeniero · On-call · Admin
**Prioridad:** obligatorio (criterio chatbot)

## Objetivo
Conversar con un copiloto que responde consultas sobre los runbooks (RAG) y ejecuta acciones mediante tool calling (abrir incidente, agregar nota, listar abiertos) con los permisos del usuario.

## Reglas de negocio
- BR-14: las herramientas del copiloto se ejecutan como el usuario autenticado, aplican las reglas BR-01…BR-13 y registran a ese usuario como autor del evento.
- BR-12: todo usuario autenticado puede leer los runbooks que consulta el copiloto.
- BR-01: un incidente abierto por el copiloto recibe la severidad sugerida por criticidad × impacto.
- BR-09: las acciones del copiloto generan eventos append-only en la timeline.
- BR-15: "listar abiertos" muestra solo incidentes activos y admite los mismos filtros que el tablero.
- BR-10: sin sesión válida no se usa el copiloto.

## Criterios de aceptación
- **UC-10.1** Dado un runbook del servicio `payments` indexado, cuando el usuario pregunta cómo proceder ante un problema descrito en ese runbook, entonces la respuesta se basa en su contenido y cita el runbook de origen (BR-12).
- **UC-10.2** Dado que ningún runbook es relevante para la consulta, cuando el usuario pregunta, entonces el copiloto indica que no encontró información en los runbooks y no inventa un procedimiento.
- **UC-10.3** Dado un `ingeniero` autenticado, cuando pide "abrí un incidente en payments, caída total", entonces se crea un incidente con severidad sugerida por BR-01, el evento `declaracion` tiene a ese usuario como autor y el copiloto confirma con el identificador creado (BR-14, BR-01, BR-09).
- **UC-10.4** Dado un incidente activo, cuando el usuario pide agregar una nota mediante el copiloto, entonces se inserta un evento `nota` en la timeline con ese usuario como autor (BR-14, BR-09).
- **UC-10.5** Dado incidentes activos y uno `cerrado`, cuando el usuario pide listar los abiertos, opcionalmente filtrando por severidad o servicio, entonces el copiloto responde solo con incidentes activos que cumplen el filtro (BR-15, BR-14, PA-11).
- **UC-10.6** Dado un usuario, cuando pide al copiloto una acción sin herramienta disponible (por ejemplo "cerrá el incidente"), entonces el copiloto informa que no puede hacerlo y el estado del incidente no cambia (BR-14, BR-08).

## Fuera de alcance
- Herramientas distintas de abrir incidente, agregar nota y listar abiertos: cerrar o transicionar incidentes, gestionar runbooks, aprobar postmortem.
- Acciones con permisos superiores a los del usuario autenticado (BR-14).
- Generación del postmortem desde el chat (UC-08).
- Voz, adjuntos o entrada multimodal.

## Verificación
- Unit: `TestBR14_HerramientasActuanComoUsuario`, `TestBR14_SinHerramientaDeCierre`, `TestBR15_ListarAbiertosSoloActivos`, `TestCopiloto_RAGCitaRunbook`, `TestCopiloto_SinContextoNoInventa` en `backend/internal/copilot/...`. El LLM y el modelo de embeddings se reemplazan por stubs/fakes deterministas en los tests unitarios.
- E2E: `e2e/uc-10.spec.ts` (un test por criterio de aceptación)
