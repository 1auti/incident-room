# Optimización de tokens (Claude Code y opencode)

Convención: **[medido]** = dato observado en archivos; **[estimado]** = cálculo aproximado;
**[sin verificar]** = suposición pendiente de comprobar. No se afirman ahorros que no se midieron:
los ahorros son "esperables", no medidos.

## 1. Medición

| Origen | Tamaño | Nota |
|---|---|---|
| `~/.claude/CLAUDE.md` (nivel usuario) | 442 líneas / 66.358 bytes [medido] | ~16k tokens a ~4 bytes/token [estimado]; se carga en cada sesión de Claude Code |
| `~/.config/opencode/AGENTS.md` | 210 líneas / 14.281 bytes [medido] | |
| `~/.config/opencode/opencode.json` | 166.718 bytes, 23 agentes [medido] | un prompt de orquestador de ~90 KB [medido] |
| Reglas del proyecto | ~4 KB [medido] | `AGENTS.md` 1.964 + `backend/AGENTS.md` 1.545 + `frontend/AGENTS.md` 576 bytes |

- Probablemente solo se carga el prompt del agente activo, no los 23 [sin verificar].
  Cuál es el agente por defecto de opencode [sin verificar].
- Conclusión: lo que pesa es la configuración global del usuario (decenas de KB), no las reglas del
  proyecto (~4 KB). Recortar el proyecto solo aporta poco; el mayor ahorro esperable está fuera del repo.
- Antes del cambio, el hook Stop inyectaba ~40 líneas de error en cada turno con cambios solo de
  documentación y sin `backend/go.mod` [medido por el propio hook y por el feedback repetido].

## 2. Qué se cambió en este repo y por qué

- **`.claude/hooks/stop-verify.sh`:** ahora corre `make verify` solo si hay cambios pendientes en
  `backend/`, `frontend/`, `Makefile`, `.claude/hooks/` o `.githooks/`. Cambios solo en `docs/`,
  `specs/`, `odd/` o `*.md` (incluidos `AGENTS.md`/`CLAUDE.md` de `backend/` y `frontend/`, que son
  contexto y no pueden romper build ni lint) ya no disparan la verificación. Un renombre hacia una
  ruta de fuentes sí dispara.
- **`.claude/hooks/post-edit.sh`:** la salida de `go vet` y de `eslint` que vuelve a Claude se acota
  a las últimas 40 líneas. Mismos chequeos, mismos mensajes y mismos códigos de salida.
- **`.claude/commands/implement-uc.md` y `.claude/agents/reviewer.md`:** dejan de repetir el flujo y
  las reglas de `AGENTS.md` y lo referencian. Se conserva lo propio de cada uno (orden de acciones del
  comando, test primero para reglas de negocio, cobertura de criterios y BR-xx en la revisión).
- **`AGENTS.md`:** `docs/ARCHITECTURE.md` se marca como pendiente para que no se busque.

**La verificación no se debilita:** cualquier cambio en fuentes sigue ejecutando `make verify`
completo (build, vet, tests, lint, typecheck); no se tocaron el `Makefile`, `.githooks/` ni CI.
Lo único que se evita es correrlo cuando nada verificable cambió.

## 3. Reglas compartidas por ambas herramientas

- Una sola fuente de reglas: `AGENTS.md`. `CLAUDE.md` solo lo importa (`@AGENTS.md`). opencode lee
  `AGENTS.md` de forma nativa [sin verificar para este repo].
- Reglas cortas y sin duplicar entre archivos: lo que se repite se paga en cada sesión.
- El contexto vive en `specs/` y `docs/domain.md`, no en el chat: cada sesión lo lee de ahí y no
  hace falta arrastrar conversaciones largas.

## 4. Hábitos de sesión

- Un caso de uso por sesión; limpiar el contexto (`/clear` o sesión nueva) entre casos.
- Esfuerzo `medium` por defecto; subirlo solo para decisiones de arquitectura.
- Modelo barato para trabajo mecánico (renombres, boilerplate, documentación).
- Delegar en un subagente solo si aísla una lectura pesada. Un subagente escritor consumió
  87k-112k tokens en la etapa de diseño de este proyecto [medido]; delegar no es gratis.
- Leer archivos por rango en vez de completos cuando se sabe qué se busca; evitar releer lo ya leído.

## 5. Recomendaciones que requieren decisión del usuario (no aplicadas)

1. Recortar el `~/.claude/CLAUDE.md` global (66 KB) y el prompt del orquestador de opencode (~90 KB),
   o moverlos a configuración por proyecto. Es la palanca más grande esperable.
2. Fijar la versión de `@playwright/mcp` en vez de `@latest` (reproducibilidad y arranque más
   predecible).
3. Cargar playwright solo para casos de uso con UI.
4. Arreglar el MCP `postgres`: `uvx` no está en el PATH de la sesión (falla la conexión).
5. Evaluar targets de `Makefile` que omitan una capa solo cuando esa capa aún no existe (por ejemplo,
   sin `go.mod`). Requiere aprobación explícita porque `AGENTS.md` prohíbe debilitar la verificación.
