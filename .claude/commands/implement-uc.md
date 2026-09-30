---
description: Implementa un caso de uso a partir de su spec (ej. /implement-uc UC-04)
---
Implementá el caso de uso $ARGUMENTS siguiendo el flujo de AGENTS.md (no lo repitas acá; aplican también sus límites y `make verify`).

Específico de este comando, en este orden:
1. Leé el spec en `specs/` cuyo nombre empieza con $ARGUMENTS, las reglas BR-xx que lista en `docs/domain.md` y el código relacionado.
   Si el spec no existe o es ambiguo, frená y preguntá.
2. Plan: además de archivos y riesgos, indicá qué test prueba cada criterio de aceptación.
   Esperá mi aprobación antes de escribir código.
3. Para reglas de negocio, escribí el test primero.
4. Con `make verify` en verde: si el caso tiene UI, validá cada criterio de aceptación con Playwright MCP.
5. Pedile al subagente `reviewer` que revise el diff y resolvé todo lo BLOQUEANTE.
6. Agregá la entrada en `docs/bitacora.md` y proponé el mensaje de commit `feat($ARGUMENTS): ...`.
