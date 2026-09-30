# Incident Room
Plataforma de gestión de incidentes estilo SRE con copiloto conversacional.
Backend Go (net/http) · PostgreSQL + pgvector · LangChainGo · Frontend React + Vite + TS.
Reglas específicas por lenguaje: `backend/AGENTS.md`, `frontend/AGENTS.md`.

## Fuentes de verdad (en este orden)
1. `specs/UC-XX-*.md`: qué hace cada caso de uso y sus criterios de aceptación.
2. `docs/domain.md`: reglas de negocio con ID (BR-xx): severidades, SLA, transiciones.
3. `docs/ARCHITECTURE.md`: componentes y decisiones.
Si una tarea no está cubierta por estas fuentes o las contradice, frená y preguntá.
No inventes reglas de negocio.

## Flujo de trabajo
1. Explorar: leer spec, dominio y código relacionado.
2. Planificar: proponer archivos a tocar, tests y riesgos. Esperar aprobación antes de implementar.
3. Implementar: lo mínimo para cumplir los criterios de aceptación.
4. Verificar: `make verify` debe pasar (los hooks lo ejecutan automáticamente).
5. Revisar: subagente `reviewer` sobre el diff; resolver todo lo BLOQUEANTE.
6. Registrar: entrada en `docs/bitacora.md`. Commit `tipo(UC-XX): descripción`.

## Verificación
- `make verify` es el único comando de verificación (build, vet, tests, lint, typecheck).
- Nunca uses `--no-verify` ni desactives, saltees o debilites tests o lint para que algo pase.

## Herramientas MCP
- context7: antes de usar la API de una librería (sobre todo LangChainGo), consultá su documentación. No asumas firmas.
- postgres (solo lectura): verificar esquema y datos después de migraciones o cambios en reglas.
- playwright: validar en la UI cada criterio de aceptación de un caso de uso con interfaz.

## Límites
- Solo lo que pide el spec: sin features, flags ni abstracciones "por si acaso".
- No agregues dependencias sin justificar por qué la librería estándar no alcanza.
- No edites migraciones ya aplicadas; creá una nueva.
- No leas ni escribas `.env`; los secretos vienen del entorno.
