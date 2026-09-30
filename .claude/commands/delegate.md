---
description: Delega un plan aprobado al ejecutor opencode (ej. /delegate UC-04)
---
Delegá la implementación de $ARGUMENTS al ejecutor opencode (`.opencode/agents/executor.md`). El flujo de AGENTS.md sigue vigente; acá solo va el contrato de handoff.

En este orden:
1. Precondición: el plan ya fue aprobado por mí (flujo de AGENTS.md). Si no, frená.
2. Armá el prompt del ejecutor con: el spec a leer (el de `specs/` que empieza con $ARGUMENTS), las reglas BR-xx aplicables, la lista exacta de archivos a tocar y el test que prueba cada criterio de aceptación. Para reglas de negocio, indicá "escribí el test primero".
3. Ejecutá desde la raíz del repo: `opencode run --agent executor --format json "<prompt>"`.
   No uses `--auto` salvo que yo lo pida: aprueba todo permiso que no esté denegado explícitamente.
   Si el resultado es largo, guardalo en el scratchpad y leé solo lo necesario.
4. Al volver NO confíes en su reporte: corré `git status`, `git diff` y `make verify` vos mismo.
5. Pedile al subagente `reviewer` que revise el diff y resolvé todo lo BLOQUEANTE (re-delegando al ejecutor o corrigiendo vos, con criterio).
6. Si el ejecutor devolvió dudas o frenó por ambigüedad, llevame la pregunta a mí; no decidas reglas de negocio.
7. Agregá la entrada en `docs/bitacora.md` (herramienta: Claude Code orquestador + opencode ejecutor, con el modelo usado) y proponé el mensaje de commit `feat($ARGUMENTS): ...`.
