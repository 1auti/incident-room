---
description: Implementa un caso de uso a partir de su spec (ej. /implement-uc UC-04)
---
Implementá el caso de uso $ARGUMENTS siguiendo el flujo de AGENTS.md:

1. Leé el spec en `specs/` cuyo nombre empieza con $ARGUMENTS, `docs/domain.md` y el código relacionado.
   Si el spec no existe o es ambiguo, frená y preguntá.
2. Proponé un plan: archivos a tocar, qué test prueba cada criterio de aceptación y riesgos.
   Esperá mi aprobación antes de escribir código.
3. Implementá lo mínimo para cumplir los criterios. Para reglas de negocio, escribí el test primero.
4. `make verify` tiene que pasar.
5. Si el caso tiene UI, validá cada criterio de aceptación con Playwright MCP.
6. Pedile al subagente `reviewer` que revise el diff. Resolvé todo lo BLOQUEANTE.
7. Agregá una entrada en `docs/bitacora.md` con el formato definido en ese archivo.
8. Proponé el mensaje de commit: `feat($ARGUMENTS): ...`.
