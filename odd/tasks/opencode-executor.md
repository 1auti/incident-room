# Feature: opencode-executor

**Objetivo:** Claude Code orquesta y opencode ejecuta el plan aprobado, con un contrato de handoff versionado en el repo.
**Por qué:** el usuario quiere Claude como orquestador y opencode como ejecutor (2026-09-29). Evidencia: `opencode` v2.0.14 instalado; sin proveedores autenticados, solo 8 modelos gratuitos (`opencode models`); el default global es `gentle-orchestrator` (orquestador, no ejecutor); no existe `.opencode/` ni referencia a opencode en `.claude/`.
**Alcance autorizado (repo):** nuevo `.opencode/agents/executor.md`, nuevo `.claude/commands/delegate.md`, `docs/bitacora.md` y este documento. La rama sale de `fix/mcp-postgres-pin`, así que el rango revisado (base `56be6ad`) también incluye `.mcp.json`, `opencode.json`, `docker-compose.yml` y `db/dev/init-agent-ro.sh`: son de esa rama, no de esta feature.
**Fuera de alcance sin OK explícito:** config global (`~/.config/opencode`, `~/.claude`), `opencode.json`, `.mcp.json`, `Makefile`, `AGENTS.md`, autenticar proveedores.
**TDD:** no aplica (config y prompts, sin código). Verificación: smoke test de `opencode run --agent executor` + `make verify`. **Rama:** `feature/opencode-executor` (parte de `fix/mcp-postgres-pin`, 2 commits sobre `main`).
**Modelo inicial del ejecutor:** `opencode/big-pickle` (gratuito; calidad sin verificar; cambio de una línea).
**Estrategia de entrega:** ask-on-risk; forecast < 400 líneas cambiadas, un solo PR.

## Tareas
- [x] T1 `.opencode/agents/executor.md`: implementa solo el plan aprobado, corre `make verify`; `deny` para `git commit`, `git push` y `.env`.
- [x] T2 `.claude/commands/delegate.md`: contrato del lado de Claude (spec + plan → `opencode run --agent executor --format json` → revisión del diff con `reviewer`).
- [x] T3 Smoke test: confirmar si `--agent` acepta un agente `subagent` y que los `deny` se respetan; si no, pasar a `mode: primary`.
- [x] T4 Entrada en `docs/bitacora.md` y commit `feat(tooling): ...` (commit `aefd4f1`; el pre-commit `make verify` pasó).
- [x] T5 Endurecer `executor.md` tras la revisión nativa (R1-002, R3 `.env`): `read` de `.env*` en `deny` y más `shell` en `deny` (`rm`, `curl`, `wget`, `git restore/clean/stash/-C/config`, `bash`/`sh`); verificar con smoke test. Corregir el alcance declarado (R2-scope-mismatch).

Ruta: T1+T2 delegated (2 archivos no triviales, un solo writer); T3 y T4 del parent.

## Progreso / evidencia
- Formato de agente v2 verificado en la doc (context7 `/websites/opencode_ai_v2`): `.opencode/agents/<nombre>.md`, frontmatter `mode`, `model` (`provider/model#variant`), `permissions` como lista `{action, resource, effect}`.
- T1+T2 escritos por un writer delegado; el parent leyó ambos archivos completos y `git status` mostró solo los 3 archivos esperados.
- T3 (parent, `opencode run --agent executor --format json`): `mode: subagent` es aceptado por `--agent`; `git commit --dry-run` → "Permission denied: shell"; escribir `.env.smoketest` → "Permission denied: edit"; `git status -s` y `make -n verify` → completados (el shell no está bloqueado entero). Sin archivos residuales.
- Corrección (T5): la doc v2 sí define la acción `read` y las reglas se evalúan en orden, gana la última que coincide (context7 `/websites/opencode_ai_v2`); mi nota anterior de que `.env` solo se protegía por prompt era incorrecta.
- Revisión nativa (RDD): riesgo `high` (por `db/dev/init-agent-ro.sh`, rango desde `56be6ad`, 8 archivos / 133 líneas); consentimiento concedido, 4 lentes, aprobada sin correcciones, acuse ejecutado (autoridad quemada). 12 hallazgos no bloqueantes (0 blockers).
- T5 (parent, inline: un solo archivo no trivial): smoke test con `opencode run --agent executor`: `read .env.smoketest` → "Permission denied: read"; `cat .env.smoketest`, `curl --version`, `rm --version`, `git stash list` → "Permission denied: shell"; `git status -s` → completado. Un primer intento cortó por timeout tras 4 probes y repetí solo los 2 restantes. Archivo temporal borrado.
- Límite que queda: la lista es de denegación, no de permiso explícito: cualquier comando no listado sigue permitido (p. ej. `python -c`, `node -e`). Una lista de permitidos sería más estricta pero puede bloquear comandos legítimos del flujo.
- No verificado: calidad de `opencode/big-pickle` implementando código real; el smoke test solo prueba permisos.
