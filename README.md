# Incident Room

Plataforma web para declarar, triagear y resolver incidentes de producción, inspirada en prácticas SRE. Incluye reglas de negocio (severidades, SLA, escalado automático, postmortem obligatorio) y un copiloto conversacional que consulta runbooks mediante RAG y ejecuta acciones sobre el sistema mediante tool calling.

Trabajo Práctico Integrador. El foco de la evaluación es la orquestación del entorno de trabajo (AI Engineering) y la funcionalidad entregada.

> **Estado:** etapa de diseño cerrada (dominio y 10 specs); backend y frontend inicializados, casos de uso en implementación. Las secciones marcadas **(pendiente)** se completan a medida que avanza el desarrollo.

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

Cada caso tiene su spec con criterios de aceptación Dado/cuando/entonces, en `specs/`. Las reglas de negocio con ID (BR-01 a BR-19) están en [`docs/domain.md`](docs/domain.md).

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

## Arquitectura

Backend por capas: `handler → service → repository`.

- **handler:** decodifica, valida formato y traduce errores de dominio a HTTP.
- **service:** todas las reglas de negocio (BR-xx).
- **repository:** SQL.

Decisiones de diseño que hacen imposibles de violar dos invariantes:

- **Timeline append-only (BR-09):** el repositorio de eventos solo expone inserción y consultas; no existe Update ni Delete.
- **Transiciones (BR-08):** una tabla explícita estado → destinos válidos, consultada por una única función.
- **Reloj inyectable (BR-05):** el dominio recibe el instante actual, para testear vencimientos de SLA.
- **Copiloto (BR-14):** las tools llaman a `service`, nunca a SQL; las reglas aplican igual por chat que por UI.

Máquina de estados: `Declarado → Reconocido → Mitigando → Resuelto → Cerrado` (sin reapertura ni saltos). Reglas clave: resolver exige causa raíz (BR-06); cerrar un SEV1 exige postmortem aprobado (BR-07); si vence el SLA sin reconocimiento se escala a los admins (BR-04).

Detalle en [`docs/domain.md`](docs/domain.md) y en las reglas por carpeta (`backend/AGENTS.md`, `frontend/AGENTS.md`). El documento de arquitectura `docs/ARCHITECTURE.md` está **(pendiente)**.

## AI Engineering

### Reglas de contexto

- [`AGENTS.md`](AGENTS.md) es la única fuente de reglas del proyecto; `CLAUDE.md` solo lo importa con `@AGENTS.md`, así que Claude Code y opencode leen lo mismo.
- Reglas específicas por lenguaje en `backend/AGENTS.md` y `frontend/AGENTS.md`.
- Orden de fuentes de verdad: `specs/` → `docs/domain.md` → `docs/ARCHITECTURE.md`. Si algo no está cubierto o se contradice, el agente frena y pregunta; no inventa reglas de negocio.
- Flujo de trabajo obligatorio: explorar, planificar (con aprobación), implementar lo mínimo, verificar, revisar, registrar.
- Comando `/implement-uc UC-XX` (`.claude/commands/`) y subagente `reviewer` (`.claude/agents/`), que revisa el diff contra el spec y las reglas de dominio.

### Metodología

Diseño primero, código después. Se produjeron `docs/domain.md` y los 10 specs antes de escribir una línea de la aplicación, y el dominio se aprobó antes de redactar los specs. Las preguntas que ni la propuesta ni las decisiones cubrían se listaron como preguntas abiertas en lugar de inventar reglas, y se resolvieron una por una (tabla de decisiones en la sección 6 de `domain.md`).

### Loops de autocorrección y verificación

| Capa | Qué hace |
|---|---|
| Hook `post-edit` | Tras cada Edit/Write: `gofmt` y `go vet` en Go, `oxlint` en el frontend. Si falla, el error vuelve al agente en el mismo turno. |
| Hook `Stop` | Al terminar una respuesta, si hubo cambios en código, ejecuta `make verify`. Un solo intento automático de corrección. |
| `pre-commit` | `make verify`, independiente de la herramienta. Se activa con `make setup`. |
| Permisos | Se deniega leer `.env*`, `git commit --no-verify` y `git push --force`. |
| Revisión nativa | Revisión adversarial con cuatro lentes (riesgo, resiliencia, legibilidad, confiabilidad) sobre cambios de riesgo medio o alto. |

`make verify` es el único comando de verificación: build, vet, tests y lint del backend; lint y build (con typecheck) del frontend.

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

## Copiloto conversacional (pendiente)

Consultas sobre runbooks con RAG (pgvector) y acciones por tool calling: abrir incidente, agregar nota, listar abiertos. Se documenta al implementar UC-10.

## Cómo correrlo

```sh
make setup      # activa el hook de git pre-commit
make verify     # build, vet, tests y lint de backend y frontend
make verify-db  # tests de backend con SQL real (Postgres descartable; requiere Docker)
make e2e        # Playwright contra la aplicación real (requiere Docker y `npx playwright install chromium` una vez)
make dev        # levanta la aplicación completa para usarla en el navegador (requiere Docker)
```

`make dev` levanta un Postgres descartable, el backend (puerto 8080) y el frontend (puerto 5173) y muestra la URL y las credenciales del admin de ejemplo (`admin@incident-room.local` con una contraseña aleatoria, o las que exportes en `ADMIN_EMAIL` y `ADMIN_PASSWORD`). Ctrl-C lo detiene y elimina la base: **los datos no persisten entre ejecuciones**. Los puertos se cambian con `DEV_API_PORT` y `DEV_WEB_PORT`. Requiere Go, Node, Docker y `curl`.

Los secretos vienen del entorno; el proyecto no usa archivos `.env` en el repositorio. El proxy de Vite (`/api`) solo cubre desarrollo: cómo se sirve la aplicación en producción es una decisión de arquitectura pendiente.

## Estructura del repositorio

```
specs/         casos de uso y criterios de aceptación
docs/          dominio, bitácora, consigna, optimización de tokens
backend/       Go (cmd/api)
frontend/      React + Vite + TypeScript
db/dev/        SQL de desarrollo (usuario de solo lectura)
.claude/       hooks, comandos y subagentes de Claude Code
.githooks/     pre-commit versionado
odd/tasks/     seguimiento de las tareas de cada feature
```
