# Feature: token-optimization

**Objetivo:** reducir el gasto de tokens del flujo con Claude Code y opencode, y documentarlo.
**Por qué:** el usuario pidió optimizar el uso de tokens antes de empezar la implementación (2026-09-29). Evidencia medida: `~/.claude/CLAUDE.md` 66 KB, `~/.config/opencode/opencode.json` 166 KB (prompt del orquestador de 90 KB), reglas del proyecto 4 KB; el Stop hook inyectó ~40 líneas de error en cada turno con el árbol sucio y sin `go.mod`; el flujo de 8 pasos está duplicado entre `AGENTS.md` e `implement-uc.md`.
**Alcance autorizado (repo):** `.claude/hooks/*.sh`, `.claude/commands/implement-uc.md`, `.claude/agents/reviewer.md`, `AGENTS.md` (solo la referencia a ARCHITECTURE.md), nuevo `docs/optimizacion-tokens.md`, `docs/bitacora.md`.
**Fuera de alcance sin OK explícito:** config global del usuario (`~/.claude`, `~/.config/opencode`), `Makefile` (lo usan CI y pre-commit), `.mcp.json`/`opencode.json`, debilitar `make verify`.
**TDD:** no aplica (scripts de hook: se prueban simulando su entrada). **Rama:** `chore/token-optimization`.

## Tareas
- [x] T1 `stop-verify.sh`: verificar solo si cambiaron fuentes (backend/, frontend/, Makefile, .claude/hooks, .githooks); docs no disparan. Sin debilitar: con código sucio, `make verify` corre igual.
- [x] T2 `post-edit.sh`: acotar la salida inyectada (`tail -n 40`) en las ramas Go y eslint.
- [x] T3 Deduplicar `implement-uc.md` (referenciar AGENTS.md en vez de repetir el flujo) y recortar `reviewer.md` a lo que no está en AGENTS.md.
- [x] T4 `AGENTS.md`: `docs/ARCHITECTURE.md` no existe; marcar "(pendiente)" para que no se busque.
- [x] T5 `docs/optimizacion-tokens.md` (Claude Code + opencode) y entrada en bitácora.
- [x] T6 Verificación: simular el hook Stop en 3 casos; `rg` de duplicados.

Ruta: delegated (2+ archivos no triviales); verificación del parent.

## Progreso / evidencia
- T1–T5 hechos por el writer; T6 verificado por el parent: repo temporal, solo `docs/` modificado → exit 0 sin salida; `backend/main.go` creado sin go.mod → exit 2 con "make verify falló"; `bash -n` OK en ambos hooks (reporte del writer).
- `make verify` no se debilitó: `Makefile`, `.githooks/`, `.mcp.json`, `opencode.json` y `settings.json` sin tocar.
- Sin medición real de ahorro de tokens; el doc usa "esperable" para estimaciones.
- Pendiente del usuario (no aplicado): recortar config global (66 KB / 166 KB), fijar versión de `@playwright/mcp`, arreglar MCP `postgres` (uvx), guards por capa en el Makefile.
