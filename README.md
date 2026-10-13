# Incident Room

Plataforma web para declarar, triagear y resolver incidentes de producción, inspirada en prácticas SRE. Incluye reglas de negocio (severidades, SLA, escalado automático, postmortem obligatorio) y un copiloto conversacional que consulta runbooks mediante RAG y ejecuta acciones sobre el sistema mediante tool calling.

Trabajo Práctico Integrador. El foco de la evaluación es la orquestación del entorno de trabajo (AI Engineering) y la funcionalidad entregada.

> **Estado:** los 10 casos de uso propuestos y UC-11 (gestión de servicios, agregado durante el desarrollo) están implementados, con pruebas unitarias, SQL y E2E por criterio de aceptación.

## Equipo

| Integrante | Mail | GitHub |
|---|---|---|
| Cenizo, Lautaro Julián | lautacenizo@gmail.com | [1auti](https://github.com/1auti) |

## Stack

| Capa | Tecnología |
|---|---|
| Backend | Go (`net/http`) |
| Base de datos | PostgreSQL con pgvector |
| IA | LangChainGo (chat, RAG y tool calling) |
| Frontend | React + Vite + TypeScript |
| Desarrollo asistido | Claude Code y opencode, con reglas de contexto versionadas y servidores MCP |

## Casos de uso

Cada caso tiene su spec con criterios de aceptación Dado/cuando/entonces, en `specs/`. Las reglas de negocio con ID (BR-01 a BR-20) están en [`docs/domain.md`](docs/domain.md).

| UC | Caso de uso | Prioridad | Spec |
|---|---|---|---|
| 01 | Registro e inicio de sesión, con roles | núcleo | [UC-01](specs/UC-01-registro-login.md) |
| 02 | Declarar un incidente con sugerencia de severidad | núcleo | [UC-02](specs/UC-02-declarar-incidente.md) |
| 03 | Tablero de incidentes activos con filtros | núcleo | [UC-03](specs/UC-03-tablero-incidentes.md) |
| 04 | Asignación y escalado automático por SLA | secundario | [UC-04](specs/UC-04-asignacion-escalado.md) |
| 05 | Timeline del incidente, append-only | núcleo | [UC-05](specs/UC-05-timeline-incidente.md) |
| 06 | Transiciones de estado con reglas | núcleo | [UC-06](specs/UC-06-transiciones-estado.md) |
| 07 | Gestión de runbooks por servicio | núcleo | [UC-07](specs/UC-07-gestion-runbooks.md) |
| 08 | Postmortem asistido por LLM | secundario | [UC-08](specs/UC-08-postmortem-asistido.md) |
| 09 | Dashboard de métricas (MTTR, incidentes por servicio, SLA) | núcleo | [UC-09](specs/UC-09-dashboard-metricas.md) |
| 10 | Copiloto conversacional (RAG y tool calling) | obligatorio (criterio chatbot) | [UC-10](specs/UC-10-copiloto-conversacional.md) |
| 11 | Gestión de servicios (alta, edición, baja y criticidad) | secundario | [UC-11](specs/UC-11-gestion-servicios.md) |

UC-11 no estaba entre los 10 propuestos. Al planificar UC-02 apareció que ningún spec cubría crear servicios, aunque UC-02, UC-04, UC-07 y UC-09 los necesitan. El agente frenó y preguntó, como pide `AGENTS.md`, y el spec se escribió con las decisiones del usuario (bitácora del 2026-10-01).

## Arquitectura

```
Navegador ──> Frontend (React + Vite) ──/api──> Backend Go (net/http) ──> PostgreSQL + pgvector
                                                    ├──> Ollama: embeddings
                                                    └──> Chat compatible con OpenAI: Ollama o Groq
```

El backend está organizado por capas: `handler → service → repository`.

- **handler** (`internal/httpapi`): decodifica, valida el formato y traduce los errores de dominio a HTTP.
- **service:** concentra todas las reglas de negocio (BR-xx). Hay un paquete por área del dominio en `internal/`.
- **repository:** el SQL, en el `postgres.go` de cada paquete con persistencia.

Algunas decisiones de diseño hacen que ciertas invariantes no se puedan violar:

- **Timeline append-only (BR-09):** el repositorio de eventos solo expone inserción y consultas; no existe Update ni Delete.
- **Transiciones (BR-08):** una tabla explícita estado → destinos válidos, consultada por una única función.
- **Reloj inyectable (BR-05):** el dominio recibe el instante actual, así se pueden testear los vencimientos de SLA.
- **Copiloto (BR-14):** las herramientas llaman a los services, nunca al SQL, así que las reglas aplican igual por chat que por UI.

Máquina de estados: `Declarado → Reconocido → Mitigando → Resuelto → Cerrado`, sin reapertura ni saltos. Reglas clave:
- resolver exige causa raíz (BR-06);
- cerrar un SEV1 exige un postmortem aprobado (BR-07);
- si vence el SLA sin reconocimiento, el incidente se escala a los admins (BR-04).

El detalle de los componentes, los paquetes, las decisiones y lo que queda pendiente (cómo se sirve en producción) está en [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md). Las reglas de negocio están en [`docs/domain.md`](docs/domain.md).

## AI Engineering

### Reglas de contexto

- [`AGENTS.md`](AGENTS.md) es la única fuente de reglas del proyecto; `CLAUDE.md` solo lo importa con `@AGENTS.md`, así que Claude Code y opencode leen lo mismo.
- Reglas específicas por lenguaje en `backend/AGENTS.md` y `frontend/AGENTS.md`.
- Orden de fuentes de verdad: `specs/` → `docs/domain.md` → `docs/ARCHITECTURE.md`. Si algo no está cubierto o se contradice, el agente frena y pregunta; no inventa reglas de negocio.
- Flujo de trabajo obligatorio: explorar, planificar (con aprobación), implementar lo mínimo, verificar, revisar, registrar.
- Reglas de código que guían patrones de diseño: interfaces en el paquete que las consume, inyección por constructor, sin estado global, una prueba table-driven por regla con el ID BR-xx en el nombre, y nada de patrón State para las transiciones.

### Agentes y comandos

Cada agente tiene una sola responsabilidad y el modelo que le alcanza para cumplirla. Las tareas baratas, como explorar y resumir, van a Haiku, y las decisiones de diseño van a Opus.

| Agente (`.claude/agents/`) | Modelo | Herramientas | Rol |
|---|---|---|---|
| `writer` | Haiku | lectura y Bash | Explora el repo para un caso de uso y escribe el informe crudo: spec, reglas, código y pruebas relacionadas. |
| `communicator` | Haiku | solo lectura | Condensa el informe en un brief corto para el architect. |
| `architect` | Opus | solo lectura | Decide el diseño y entrega el plan: archivos, pruebas y riesgos. |
| `builder` | Sonnet | edición y Bash | Implementa el plan aprobado (prueba primero en las reglas de negocio) y corre `make verify`. |
| `reviewer` | heredado | lectura y Bash | Revisa el diff contra el spec, `domain.md` y los `AGENTS.md`, y separa los hallazgos BLOQUEANTES de las mejoras. |
| `ask` | Haiku | solo lectura | Responde dudas puntuales sobre el repo, fuera de la cadena. |

| Comando (`.claude/commands/`) | Qué hace |
|---|---|
| `/uc UC-XX` | Orquesta la cadena `writer → communicator → architect` y se detiene hasta que el usuario aprueba el plan; después sigue con `builder → reviewer`. |
| `/implement-uc UC-XX` | Implementación directa por Claude Code, siguiendo el mismo flujo. |
| `/delegate UC-XX` | Le pasa el plan aprobado a opencode con el agente `.opencode/agents/executor.md`, que tiene denegados commit, push, `rm`, `curl` y `.env*`. Claude verifica el resultado con `git diff`, `make verify` y el `reviewer`; no se confía en el reporte del ejecutor. |

Ningún agente decide reglas de negocio: las dudas vuelven al usuario.

### Prompts clave

Cada uno está registrado en la bitácora con lo que propuso el agente y lo que falló.

| Etapa | Prompt | Resultado |
|---|---|---|
| Diseño | Dominio primero, aprobación, después los specs | `docs/domain.md` con BR-01 a BR-18 y 14 preguntas abiertas explícitas en lugar de reglas inventadas. Con el dominio aprobado, los 10 specs. |
| Diseño | "las 14 preguntas las tenés que resolver vos" | Las decisiones quedaron incorporadas en las reglas, con trazabilidad en la sección 6 de `domain.md`. Apareció BR-19. |
| Herramientas | "me gustaría que trabajes para optimizar el uso de tokens…" | Medición previa a cualquier cambio. El hook Stop pasó a correr solo con cambios en fuentes, y se deduplicaron los comandos contra `AGENTS.md`. |
| Herramientas | "mi idea es trabajar claude como orquestador y opencode como el que hace" | Agente `executor` de opencode y `/delegate` con contrato de handoff. Se probó en BR-01 (UC-02). |
| Implementación | "vamos a crear una rama feature/caso de uso y implementarlo" | Un caso de uso por rama y por PR (#5 a #10), de UC-01 a UC-04 y UC-11. La cadena de agentes se usó en UC-01 (tramo T3) y en UC-11, según la bitácora. |
| Implementación | "Implementa los casos de uso que faltan" | UC-05 a UC-10, uno por vez. Primero el plan; cuando había alternativas, el usuario eligió una ("Si, con A" en UC-08, "Si, con las dos A" en UC-10). Después, implementación directa más `reviewer`. |

### Metodología

Diseño primero, código después. Se produjeron `docs/domain.md` y los 10 specs antes de escribir una línea de la aplicación, y el dominio se aprobó antes de redactar los specs. Las preguntas que ni la propuesta ni las decisiones cubrían se listaron como preguntas abiertas en lugar de inventar reglas, y se resolvieron una por una (tabla de decisiones en la sección 6 de `domain.md`).

### Loops de autocorrección y verificación

| Capa | Qué hace |
|---|---|
| Hook `post-edit` | Tras cada Edit/Write: `gofmt` y `go vet` en Go, `oxlint` en el frontend. Si falla, el error vuelve al agente en el mismo turno. |
| Hook `Stop` | Al terminar una respuesta, si hubo cambios en código, ejecuta `make verify`. Un solo intento automático de corrección. |
| `pre-commit` | Corre solo sobre lo staged, independiente de la herramienta: `gofumpt`, `go vet` y las pruebas de los paquetes Go afectados; en el frontend, `oxlint`, `prettier --check` y el typecheck. Se activa con `make setup` o con el hook `SessionStart`. El `make verify` completo queda en el hook Stop (justificación en `docs/tooling-decisions.md`). |
| `reviewer` | Después de cada caso de uso. Todo BLOQUEANTE se corrige antes del commit. Detectó, entre otros, una carrera entre el LLM y el cierre del incidente en UC-08, y en UC-10 la pérdida de las acciones ya ejecutadas cuando el modelo fallaba. |
| Permisos | Se deniega leer `.env*`, `git commit --no-verify` y `git push --force`. |
| Revisión nativa | Revisión adversarial con cuatro lentes (riesgo, resiliencia, legibilidad, confiabilidad) sobre cambios de riesgo medio o alto. |

`make verify` es el único comando de verificación: build, vet, tests y lint del backend; lint y build (con typecheck) del frontend.

### Qué se aprendió del loop

- **Cada capa detectó errores distintos.** La e2e encontró un error de diseño del copiloto que la prueba unitaria no veía, porque su modelo fake no se comportaba como uno real. El `reviewer` encontró que un postmortem podía guardarse sobre un incidente cerrado mientras el LLM respondía. El hook Stop encontró un linter compilado con un Go más viejo que el del módulo.
- **El reporte de un agente no es evidencia.** En la primera delegación, el ejecutor pegó una salida de `make verify` reescrita a mano. Desde entonces la verificación la repite siempre el orquestador.

### Bitácora

[`docs/bitacora.md`](docs/bitacora.md) registra cada iteración relevante: qué propuso el agente, qué falló y qué capa lo detectó, la corrección y los ajustes a las reglas de contexto. Es la materia prima de esta sección.

### Optimización de tokens

Medición y cambios aplicados para las dos herramientas en [`docs/optimizacion-tokens.md`](docs/optimizacion-tokens.md).

## Servidores MCP

Configurados en `.mcp.json` (Claude Code) y `opencode.json` (opencode).

| Servidor | Tipo | Rol en el desarrollo |
|---|---|---|
| `context7` | externo (HTTP remoto) | Consultar la documentación vigente de una librería antes de usar su API, sobre todo LangChainGo. |
| `playwright` | local (`npx`) | Validar en la UI cada criterio de aceptación de un caso de uso con interfaz. |
| `postgres` | local (`uvx`), solo lectura | Verificar esquema y datos después de migraciones o cambios de reglas. Usa un usuario de solo lectura (`db/dev/agent-readonly.sql`). |

Configuración:

- **`context7`:** no necesita credenciales; es un servidor HTTP remoto.
- **`playwright`:** usa los `data-testid` estables que `frontend/AGENTS.md` exige en todo elemento mencionado por un criterio de aceptación. Esos mismos selectores los usan las pruebas de `e2e/`.
- **`postgres`:** corre `postgres-mcp` en `--access-mode=restricted` y lee la conexión de `AGENT_RO_DATABASE_URL`. Los agentes nunca reciben la URL de la API, y el rol se crea con `db/dev/init-agent-ro.sh`.

Las reglas de contexto les indican a los agentes cuándo usar cada servidor (sección "Herramientas MCP" de `AGENTS.md`). Un servidor que no conecta no se reemplaza con suposiciones. Por ejemplo, en las sesiones en la nube `context7` recibió un 403 del proxy, y la API de LangChainGo se leyó del código fuente del módulo en lugar de asumir firmas (bitácora de UC-07).

## Copiloto conversacional

Construido con **LangChainGo** (`github.com/tmc/langchaingo`): `llms/openai` para el chat con tool calling y `embeddings` con `llms/ollama` para los vectores. `POST /api/copilot` responde preguntas sobre los runbooks con RAG y ejecuta tres herramientas por tool calling: abrir incidente, agregar nota y listar abiertos (UC-10).

- **RAG:** la pregunta se convierte en embedding y se buscan hasta 3 runbooks por similitud coseno en pgvector, con un mínimo configurable con `RAG_MIN_SIMILARITY` (0.5 por defecto, calibrado para `nomic-embed-text`). Las fuentes que muestra la respuesta son los runbooks recuperados por el backend, no lo que diga el modelo.
- **Sin runbook relevante y sin herramientas usadas**, la respuesta es fija: no encontró información y no propone un procedimiento. Así un modelo no puede inventar pasos ni contradecir la ausencia de herramientas para cerrar o transicionar.
- **Herramientas:** corren como el usuario autenticado y llaman a los mismos services que la API, así que aplican las mismas reglas (BR-14) y registran a ese usuario como autor. No hay herramienta para cerrar ni transicionar incidentes.
- **Modelo:** el mismo chat del postmortem (`LLM_PROVIDER`), a través de la API compatible con OpenAI (Ollama en `OLLAMA_URL/v1`). El modelo tiene que soportar tool calling (`llama3.1` lo soporta).

## Cómo correrlo

```sh
make setup      # activa el hook de git pre-commit
make verify     # build, vet, tests y lint de backend y frontend
make verify-db  # tests de backend con SQL real (Postgres descartable; requiere Docker)
make e2e        # Playwright contra la aplicación real (requiere Docker y `npx playwright install chromium` una vez)
make dev        # levanta la aplicación completa para usarla en el navegador (requiere Docker)
```

`make dev` levanta un Postgres descartable, el backend (puerto 8080) y el frontend (puerto 5173) y muestra la URL y las credenciales del admin de ejemplo (`admin@incident-room.local` con una contraseña aleatoria, o las que exportes en `ADMIN_EMAIL` y `ADMIN_PASSWORD`). Ctrl-C lo detiene y elimina la base: **los datos no persisten entre ejecuciones**. Los puertos se cambian con `DEV_API_PORT` y `DEV_WEB_PORT`. Requiere Go, Node, Docker y `curl`.

Los runbooks (UC-07) guardan un embedding calculado por Ollama: crear o editar uno responde 503 si Ollama no está disponible. Se configura con `OLLAMA_URL` (por defecto `http://localhost:11434`) y `EMBEDDING_MODEL` (por defecto `nomic-embed-text`, que produce los 768 valores que espera la tabla; otro modelo requiere una migración nueva). Antes de usarlo, `ollama pull nomic-embed-text`. La migración `0007` crea la extensión `vector`, lo que requiere que el rol de la base sea superusuario (lo es en `make dev`, `make e2e` y `make verify-db`); si no lo es, un DBA debe crearla antes con `CREATE EXTENSION vector WITH SCHEMA public`. El borrador del postmortem (UC-08) lo escribe un modelo de chat elegido con `LLM_PROVIDER`: `ollama` (por defecto, usa la API compatible con OpenAI en `OLLAMA_URL/v1` y `LLM_MODEL`, por defecto `llama3.1`; antes, `ollama pull llama3.1`) o `groq` (requiere `GROQ_API_KEY`, que el backend exige al arrancar; `LLM_MODEL` por defecto `llama-3.1-8b-instant` y `LLM_BASE_URL` por defecto `https://api.groq.com/openai/v1`). Los embeddings siempre salen de Ollama. Si el modelo no responde, generar el postmortem responde 503 y no guarda nada. La e2e no usa Ollama real: `e2e/fake-ollama.mjs` responde embeddings y chat en su lugar (para el copiloto, con reglas por palabras clave que imitan un modelo con tool calling).

Los secretos vienen del entorno; el proyecto no usa archivos `.env` en el repositorio. El proxy de Vite (`/api`) solo cubre desarrollo: cómo se sirve la aplicación en producción es una decisión de arquitectura pendiente.

## Estructura del repositorio

```
specs/         casos de uso y criterios de aceptación
docs/          arquitectura, dominio, bitácora, consigna, optimización de tokens
backend/       Go: cmd/api (arranque), internal/ (un paquete por área), migrations/ (SQL embebido)
frontend/      React + Vite + TypeScript (src/features/, src/api/)
e2e/           Playwright: un spec por caso de uso y fake-ollama.mjs
scripts/       dev, e2e, verify-db, verify-changed e instalación de herramientas
db/dev/        SQL de desarrollo (usuario de solo lectura)
.claude/       hooks, comandos y subagentes de Claude Code
.opencode/     agente executor para /delegate
.githooks/     pre-commit versionado
odd/tasks/     seguimiento de las tareas de cada feature
```
