# Feature: opencode-executor

**Objetivo:** Claude Code orquesta y opencode ejecuta el plan aprobado, con un contrato de handoff versionado en el repo.
**Por qué:** el usuario quiere Claude como orquestador y opencode como ejecutor (2026-09-29). Evidencia: `opencode` v2.0.14 instalado; sin proveedores autenticados, solo 8 modelos gratuitos (`opencode models`); el default global es `gentle-orchestrator` (orquestador, no ejecutor); no existe `.opencode/` ni referencia a opencode en `.claude/`.
**Alcance autorizado (repo):** nuevo `.opencode/agents/executor.md`, nuevo `.claude/commands/delegate.md`, `docs/bitacora.md`.
**Fuera de alcance sin OK explícito:** config global (`~/.config/opencode`, `~/.claude`), `opencode.json`, `.mcp.json`, `Makefile`, `AGENTS.md`, autenticar proveedores.
**TDD:** no aplica (config y prompts, sin código). Verificación: smoke test de `opencode run --agent executor` + `make verify`. **Rama:** `feature/opencode-executor` (parte de `fix/mcp-postgres-pin`, 2 commits sobre `main`).
**Modelo inicial del ejecutor:** `opencode/big-pickle` (gratuito; calidad sin verificar; cambio de una línea).
**Estrategia de entrega:** ask-on-risk; forecast < 400 líneas cambiadas, un solo PR.

## Tareas
- [x] T1 `.opencode/agents/executor.md`: implementa solo el plan aprobado, corre `make verify`; `deny` para `git commit`, `git push` y `.env`.
- [x] T2 `.claude/commands/delegate.md`: contrato del lado de Claude (spec + plan → `opencode run --agent executor --format json` → revisión del diff con `reviewer`).
- [x] T3 Smoke test: confirmar si `--agent` acepta un agente `subagent` y que los `deny` se respetan; si no, pasar a `mode: primary`.
- [ ] T4 Entrada en `docs/bitacora.md` y commit `feat(tooling): ...`.

Ruta: T1+T2 delegated (2 archivos no triviales, un solo writer); T3 y T4 del parent.

## Progreso / evidencia
- Formato de agente v2 verificado en la doc (context7 `/websites/opencode_ai_v2`): `.opencode/agents/<nombre>.md`, frontmatter `mode`, `model` (`provider/model#variant`), `permissions` como lista `{action, resource, effect}`.
- T1+T2 escritos por un writer delegado; el parent leyó ambos archivos completos y `git status` mostró solo los 3 archivos esperados.
- T3 (parent, `opencode run --agent executor --format json`): `mode: subagent` es aceptado por `--agent`; `git commit --dry-run` → "Permission denied: shell"; escribir `.env.smoketest` → "Permission denied: edit"; `git status -s` y `make -n verify` → completados (el shell no está bloqueado entero). Sin archivos residuales.
- Límite conocido: la lectura de `.env` solo está bloqueada por el prompt (la doc v2 consultada no muestra una acción `read`).
- No verificado: calidad de `opencode/big-pickle` implementando código real; el smoke test solo prueba permisos.
