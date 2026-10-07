# Decisiones de herramientas para agentes

Qué se adoptó, qué se pospuso y qué se descartó para leer, escribir y verificar código con agentes.
Rama: `chore/agent-code-tools`. Alcance: solo tooling; no toca código de negocio ni specs.

## Estado

| Estado | Herramienta | Motivo |
|---|---|---|
| Implementado | `rg -n` + `Read` con `offset`/`limit` (guía en `AGENTS.md`) | Es lo más barato en tokens: localizar y leer solo el fragmento necesario. |
| Implementado | gopls y typescript-language-server (LSP) | Definición y referencias exactas; evitan falsos positivos de `rg`. |
| Implementado | ast-grep | Búsqueda estructural (por AST) que `rg` no puede expresar. |
| Implementado | gofumpt + goimports (hook post-edit) | Formato e imports sin decisión del agente; gofumpt incluye gofmt. |
| Implementado | prettier (frontend) | Formato TS/TSX sin solaparse con oxlint. Ver "Prettier sobre Biome". |
| Implementado | vitest (frontend) | Runner de tests con `--changed`; no hay equivalente en la librería estándar de JS. |
| Implementado | golangci-lint v2 (`make lint-backend`) | Conjunto por defecto (errcheck, govet, ineffassign, staticcheck, unused); integrado en `verify-backend`. |
| Implementado | Hooks acotados y `make verify-changed` | Feedback rápido por paquete/archivo; la verificación completa sigue en el hook Stop (hoy no hay CI). |
| Pospuesto | ctags / mapa de código | Con ~3.8k LOC `rg` + LSP alcanzan. Revisar si el repo crece. |
| Pospuesto | semgrep | Sin reglas propias que justifiquen otra herramienta; ast-grep cubre la búsqueda. Revisar si el repo crece. |
| Pospuesto | serena MCP | Duplica lo que ya dan los LSP directos. Revisar si el repo crece. |
| Descartado | RAG / pgvector sobre el código | Con ~3.8k LOC el costo de infraestructura (indexado, embeddings, mantenimiento) supera el beneficio. |

## Prettier sobre Biome

Biome formatea y también lintea: duplicaría a oxlint, que ya es el linter del proyecto. Prettier solo
formatea, así que cada herramienta tiene una sola responsabilidad. La config (`frontend/.prettierrc.json`:
sin punto y coma, comillas simples, `printWidth` 120) replica el estilo existente; el código previo con
diferencias menores no se reformateó (fuera de alcance), por eso el pre-commit solo chequea los archivos
staged. Es una decisión reversible.

## Tradeoff del pre-commit

El pre-commit pasó de `make verify` completo a chequeos solo de lo staged: `gofumpt -l`, `go vet` y
`go test` de los paquetes afectados (incluidos los que dependen de ellos), `oxlint` y `prettier --check`
sobre los archivos staged y `tsc -b`. Los commits son más rápidos, a costa de no correr el lint completo de
golangci-lint ni el build de Vite en cada commit. La red de seguridad actual es el hook Stop de Claude
Code, que corre `make verify` completo; `--no-verify` sigue prohibido.

Hoy no existe CI en el repo. Cuando se agregue, debe correr `make verify`, tener golangci-lint instalado y,
si usa `make verify-changed`, hacer checkout con `fetch-depth: 0` (necesita el historial para comparar con
`main`).

Límites conocidos del pre-commit y de `verify-changed`:

- Los chequeos corren sobre el working tree, no sobre el índice: con staging parcial (`git add -p`) se
  valida contenido distinto del que se commitea.
- La selección de paquetes afectados mira los archivos Go cambiados y sus dependientes de producción. No
  detecta dependientes solo de tests ni entradas que no son Go (por ejemplo SQL embebido); un archivo Go
  borrado o renombrado hace que se corran todos los paquetes del backend.

## Instalación local

`make setup` activa los hooks y ejecuta `make tools`: instala con `go install` gofumpt, goimports y gopls, y
verifica que estén ast-grep, typescript-language-server, golangci-lint (v2), jq y rg, con la pista de
instalación de cada uno.

## Adopción

Aplica solo, sin pasos manuales (una vez abierto Claude Code en el repo):

- Hooks de `.claude/settings.json`: PostToolUse (formato y vet tras cada edición), Stop (`make verify`
  completo) y SessionStart (`git config core.hooksPath .githooks` y un aviso de una línea si faltan
  herramientas; no imprime nada si está todo).
- `AGENTS.md` (guía de lectura y búsqueda) y los prompts de los agentes (`rg -n` + `Read` acotado, LSP
  antes de leer archivos enteros).

Requiere un paso único por desarrollador:

- `make setup` (o `make tools`): instala/verifica las herramientas y, si existe el CLI `claude`, intenta
  `claude plugin install gopls-lsp@claude-plugins-official --scope project` (y `typescript-lsp`). Si falla,
  imprime la pista y no corta el script.
- `enabledPlugins` versionado habilita los plugins LSP, pero no se verificó que instale el plugin en un
  clon nuevo; por eso el paso de instalación queda como medida explícita.

Límites honestos:

- Los hooks del proyecto solo corren después de aceptar el diálogo de confianza del workspace.
- Un commit desde una terminal común antes de abrir Claude Code por primera vez (o sin `make setup`) no
  ejecuta el pre-commit, porque `core.hooksPath` todavía no está configurado.
- El LSP empuja diagnósticos al contexto tras cada edición, así que puede AGREGAR tokens. El ahorro NO
  está probado: medilo durante unas sesiones antes de afirmarlo.
