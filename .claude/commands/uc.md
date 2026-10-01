---
description: Implementa un caso de uso con la cadena writer → communicator → architect → builder → reviewer (ej. /uc UC-04)
---
Implementá $ARGUMENTS con la cadena de agentes de `.claude/agents/`. El flujo de AGENTS.md sigue vigente; acá va solo el orden y los handoffs. Cada agente se lanza con la herramienta Agent y su `subagent_type`.

1. Rama: si no estás en `feature/<uc>-<slug>`, creala desde `main` (el spec en `specs/` que empieza con $ARGUMENTS da el slug). Si el spec no existe o es ambiguo, frená y preguntá.
2. `writer`: pasale $ARGUMENTS. Devuelve el informe crudo.
3. `communicator`: pasale el informe del writer. Devuelve el brief.
4. `architect`: pasale el brief Y el informe completo del writer (el brief puede haber perdido un dato; el architect decide con ambos). Devuelve el plan.
5. Parada obligatoria: mostrame el plan del architect y esperá mi aprobación. Si trae preguntas bloqueantes, llevámelas a mí; no decidas reglas de negocio.
6. `builder`: pasale el plan aprobado, el modo TDD y el comando de verificación (`make verify`; si hay SQL, también `make verify-db`). Un solo builder a la vez.
7. No confíes en su reporte: corré `git status`, `git diff`, `make verify` y, si hay SQL, `make verify-db` vos mismo.
8. `reviewer` sobre el diff; resolvé todo lo BLOQUEANTE (re-delegando al builder o corrigiendo vos, con criterio).
9. Si hay UI, validá cada criterio de aceptación con Playwright MCP.
10. Entrada en `docs/bitacora.md` (con los modelos usados) y commit `feat($ARGUMENTS): ...`. Push y PR solo si te lo pido.
