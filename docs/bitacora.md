# Bitácora de AI Engineering

Una entrada por iteración relevante. Es la materia prima del README.

## Formato

### AAAA-MM-DD — UC-XX — <título corto>
- **Herramienta / agente:** <Claude Code | opencode | reviewer>
- **Prompt o comando:** <`/implement-uc UC-XX` o el prompt literal si fue ad hoc>
- **Qué propuso el agente:**
- **Qué falló y qué capa lo detectó:** <hook post-edit | hook stop | pre-commit | CI | reviewer | yo>
- **Corrección / decisión:**
- **Ajuste a las reglas de contexto:** <si cambiaste un AGENTS.md por esto: qué y por qué>

---

### 2026-09-29 — Diseño — Dominio y specs de los 10 casos de uso
- **Herramienta / agente:** Claude Code (orquestador + subagente escritor)
- **Prompt o comando:** prompt ad hoc de etapa de diseño (dominio primero, aprobación, luego specs)
- **Qué propuso el agente:** primero `docs/domain.md`: entidades, máquina de estados T1–T4, reglas de negocio BR-01..BR-18, matriz de permisos por rol y definición de métricas. Lo que ni las decisiones ni la propuesta definen quedó como 14 preguntas abiertas (PA-01..PA-14) en lugar de inventarse reglas; la tabla de severidad de BR-01 se marcó como borrador. Luego, ya aprobado el dominio, los 10 specs `specs/UC-01..UC-10` con 3 a 6 criterios de aceptación cada uno, citando BR-xx y PA-xx.
- **Qué falló y qué capa lo detectó:** (1) el servidor MCP `postgres` no conectó porque falta `uvx`; lo detectó `/mcp`. (2) Los archivos de la consigna tenían nombres y formato distintos a los del prompt (el PDF lleva un espacio en el nombre y la propuesta es un `.odg`); lo detectó el agente al leerlos.
- **Corrección / decisión:** el dominio fue aprobado por el usuario antes de escribir los specs; las preguntas abiertas se mantienen explícitas y los specs siguen el valor "Hoy:" de cada una.
- **Ajuste a las reglas de contexto:** ninguno todavía.

---

### 2026-09-29 — Diseño — Resolución de las 14 preguntas abiertas
- **Herramienta / agente:** Claude Code (orquestador decide en `docs/domain.md` + subagente escritor en specs)
- **Prompt o comando:** "las 14 preguntas las tenés que resolver vos" (el usuario delegó las decisiones)
- **Qué propuso el agente:** decidir las 14 preguntas y dejarlas incorporadas en las reglas, con el mapa PA → decisión → regla en la sección 6 de `docs/domain.md`. Decisiones clave: regla nueva BR-19 (registro con rol inicial `ingeniero`, email único, solo el admin promueve, primer admin desde variables de entorno); BR-02 congela la severidad desde `resuelto` para que nadie baje un SEV1 y esquive el postmortem de BR-07; BR-11 reasigna los incidentes activos y registra un evento `asignacion` cuando el admin cambia el on-call; BR-13 define que genera y edita el on-call del servicio o el admin (solo con el incidente `resuelto`) y aprueba solo el admin; BR-04 escala a todos los admins con indicador "Escalado" y sin email; BR-12 hace la baja de runbooks física y elimina su embedding; métricas con período por defecto de 30 días (MTTR por `resolved_at`, conteo y SLA por `declared_at`). Luego el subagente escritor actualizó los 10 specs: sin referencias `PA-xx`, criterios y listas de BR alineados con las reglas, y nombres de tests unitarios ajustados (por ejemplo `TestBR19_...`, `TestBR13_...`).
- **Qué falló y qué capa lo detectó:** el hook Stop (`make verify`) falla porque `backend/` todavía no tiene `go.mod`. Es esperable en la etapa de diseño y a propósito no se "arregló", para no empezar código ni debilitar la verificación.
- **Corrección / decisión:** las decisiones quedaron en `docs/domain.md` sección 6 como delegadas y revisables: al cambiar una, se actualizan la regla y los specs que la citan.
- **Ajuste a las reglas de contexto:** ninguno.

---

### 2026-09-29 — Herramientas — Optimización de tokens
- **Herramienta / agente:** Claude Code (orquestador + explorador + escritor)
- **Prompt o comando:** "me gustaria que trabajes para optimizar el uso de tokens…"
- **Qué propuso el agente:** medir primero (`~/.claude/CLAUDE.md` 66 KB, `opencode.json` 166 KB con un prompt de orquestador de 90 KB, reglas del proyecto ~4 KB) y actuar solo dentro del repo: el hook Stop verifica únicamente cuando cambian fuentes (`backend/`, `frontend/`, `Makefile`, `.claude/hooks/`, `.githooks/`), acotar a 40 líneas la salida de `post-edit.sh`, deduplicar `implement-uc.md` y `reviewer.md` contra `AGENTS.md`, marcar `docs/ARCHITECTURE.md` como pendiente y documentar todo en `docs/optimizacion-tokens.md`. Lo que exige decisión del usuario (recortar la config global, fijar `@playwright/mcp`, arreglar el MCP `postgres`, targets de `Makefile`) quedó como recomendación, sin aplicar.
- **Qué falló y qué capa lo detectó:** el hook Stop inyectaba un error en cada turno con cambios solo de documentación y sin `go.mod`. Lo detectó el propio hook y el feedback repetido del usuario.
- **Corrección / decisión:** el disparo del hook Stop pasa de "cualquier archivo sucio" a "cambio pendiente en fuentes"; con fuentes modificadas `make verify` corre completo, sin debilitar la verificación.
- **Ajuste a las reglas de contexto:** `AGENTS.md` línea 9 (ARCHITECTURE.md pendiente); hooks y comandos de `.claude` deduplicados.

---

### 2026-09-29 — Herramientas — Ejecutor opencode orquestado por Claude Code
- **Herramienta / agente:** Claude Code (orquestador + writer delegado) y opencode v2.0.14 con `opencode/big-pickle` (solo en el smoke test)
- **Prompt o comando:** "mi idea es trabajar claude como orquestador y opencode como el que hace, me falta algo por configurar ? de agents ?" y luego "empezamos con uno gratuito"
- **Qué propuso el agente:** un agente de proyecto `.opencode/agents/executor.md` (modo `subagent`, modelo gratuito, `deny` para `git commit`, `git push`, `git reset`, `git checkout`, `git rebase` y edición de `.env*`, prompt que limita el trabajo al plan y al spec y exige `make verify`) y el comando `.claude/commands/delegate.md` con el contrato de handoff: Claude arma el prompt, invoca `opencode run --agent executor --format json`, verifica él mismo con `git diff` y `make verify`, y pasa el diff por el subagente `reviewer`.
- **Qué falló y qué capa lo detectó:** nada falló en el smoke test. Dos límites quedaron a la vista: la lectura de `.env` solo la bloquea el prompt (la doc v2 consultada no muestra una acción `read`) y la calidad del modelo gratuito implementando código real no está probada.
- **Corrección / decisión:** el smoke test verificó que `--agent` acepta un agente `subagent`, que `git commit --dry-run` y escribir `.env.smoketest` se deniegan, y que `git status` y `make -n verify` se permiten. La verificación final sigue en Claude, el `reviewer` y el pre-commit; el reporte del ejecutor no se toma como prueba.
- **Endurecimiento posterior:** la revisión nativa (aprobada, 12 hallazgos no bloqueantes) marcó la lista de `deny` como angosta. Ahí corregí un error mío: la doc v2 sí tiene una acción `read`, así que `.env*` ahora se bloquea también en lectura y no solo por prompt. Se sumaron `deny` de shell para `rm`, `curl`, `wget`, `bash`, `sh`, `git restore/clean/stash/config/-C` y cualquier comando que mencione `.env`. Verificado con un smoke test; sigue siendo una lista de denegación, no de permitidos.
- **Ajuste a las reglas de contexto:** ninguno. `opencode.json`, `AGENTS.md` y la config global quedaron sin tocar.

---

### 2026-09-29 — UC-02 — BR-01 severidad sugerida (primera prueba de delegación)
- **Herramienta / agente:** Claude Code (orquestador) con Haiku (resumen del repo), Opus (plan y prompt del ejecutor) y Sonnet (`reviewer`); opencode `executor` con `opencode/big-pickle` (implementación)
- **Prompt o comando:** "probemos delegate con una tarea chica", con el plan aprobado; el flujo de `.claude/commands/delegate.md` aplicado a mano
- **Qué propuso el agente:** `SuggestSeverity(Criticality, Impact) (Severity, error)` como función pura en `backend/internal/incident/severity.go`, con la tabla 3×3 de BR-01 en un `switch` (sin estado global) y un error de dominio envuelto con `%w`. Test table-driven `TestBR01_SeveridadSugeridaPorCriticidadEImpacto` con las 9 combinaciones, los dos ejemplos de UC-02.1 y entradas inválidas o vacías. El ejecutor escribió el test, lo vio fallar por compilación, escribió la implementación y corrió `make verify`.
- **Qué falló y qué capa lo detectó:** (1) El plan aprobado ponía el código en `internal/service` con otro nombre de test, contra `specs/UC-02` línea 32 (`internal/incident`); lo detectó el reporte de un Opus de prueba y lo confirmé leyendo el spec. (2) La primera corrida del ejecutor en segundo plano quedó colgada sin crear sesión; lo detectó el usuario ("no veo cambios") y lo confirmé mirando el proceso: su stdin era un socket abierto. (3) El pre-commit no estaba activo (`core.hooksPath` vacío) y yo había afirmado que había pasado en commits anteriores; lo detectó la pregunta del usuario sobre el hook de opencode. (4) El reporte del ejecutor pegó como "salida real" una salida de `make verify` reescrita a mano, con dos tamaños gzip distintos de los reales; lo detecté al compararla con mi propia corrida.
- **Corrección / decisión:** (1) el usuario eligió seguir el spec. (2) Relancé con `< /dev/null` y esa corrida sí escribió archivos; es consistente con la hipótesis, sin haber aislado la variable. (3) Se activó con `make setup` y se probó con `git hook run pre-commit` (exit 0). (4) La verificación es propia: `gofmt -l` sin salida, `go test -v -run TestBR01` PASS en 16 casos, las 9 celdas contra `docs/domain.md` y `make verify` exit 0. El `reviewer` no encontró bloqueantes. Mejoras opcionales sin aplicar: unificar la validación de entradas inválidas en un solo `switch`, agregar doc comments, y cubrir impacto inválido con `importante` y `estandar`.
- **Ajuste a las reglas de contexto:** aplicado en un commit posterior: `.claude/commands/delegate.md` indica `< /dev/null` al lanzar `opencode run`, y `odd/tasks/opencode-executor.md` corrige la frase falsa sobre el pre-commit. El paso 4 de `delegate.md` ya establece que el reporte del ejecutor no cuenta como prueba.

---
