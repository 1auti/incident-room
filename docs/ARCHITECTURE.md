# Arquitectura

Tercera fuente de verdad del proyecto, después de `specs/UC-XX-*.md` y `docs/domain.md`. Describe lo que existe en el código; las reglas de negocio (BR-xx) viven en `docs/domain.md` y no se repiten aquí.

## 1. Componentes

```
Navegador ──► Vite dev server (5173) ──proxy /api──► API Go (8080) ──pgx──► PostgreSQL + pgvector
 (React SPA)   frontend/vite.config.ts               backend/cmd/api         migraciones embebidas
                                                        │
                                                        ├─ HTTP /api/embed ─► Ollama (embeddings, siempre)
                                                        └─ HTTP /v1 (OpenAI) ► Ollama o Groq (chat)
```

- **Frontend:** SPA en `frontend/`. En desarrollo el navegador solo habla con Vite, que reenvía `/api` a `API_PROXY_TARGET` (por defecto `http://localhost:8080`) en `frontend/vite.config.ts`.
- **Backend:** un único binario, `backend/cmd/api/main.go`, que arma las dependencias por constructor, aplica migraciones al arrancar y sirve la API con `net/http`.
- **Tarea periódica:** `runEscalation` en `main.go` escala los incidentes con SLA vencido cada `ESCALATION_INTERVAL` (BR-04). Un fallo se loguea y no detiene el servidor.
- **Base de datos:** PostgreSQL con la extensión `vector`; en desarrollo, `docker-compose.yml` (imagen `pgvector/pgvector:pg17`).

## 2. Capas del backend

Flujo `handler → service → repository` (`backend/AGENTS.md`). Las interfaces se definen en el paquete que las consume y las dependencias se inyectan por constructor, sin estado global. El reloj (`time.Now`) se inyecta en los services (BR-05).

| Paquete (`backend/internal/`) | Rol |
|---|---|
| `httpapi` | Handlers y rutas (`handlers.go`, `http.ServeMux` con patrones `METHOD /api/...`); middlewares `requireAuth` y `requireAdmin`. |
| `user`, `auth` | Registro, login (bcrypt) y sesiones opacas por cookie `session` (se guarda el hash SHA-256 del token); `auth/middleware.go`. |
| `service` | Servicios monitoreados y on-call (`manager.go`). |
| `incident` | Declaración, severidad sugerida (`severity.go`), transiciones, SLA y escalado. |
| `timeline` | Eventos del incidente, solo inserción y consulta (BR-09). |
| `runbook` | Runbooks con embedding; búsqueda por similitud (`postgres.go`). |
| `postmortem` | Borrador asistido por LLM, edición y aprobación. |
| `metrics` | Cálculos del dashboard. |
| `copilot` | RAG y tool calling (`copilot.go`, `model.go`). |
| `llm` | Adaptador de LangChainGo a las interfaces del dominio (`chat.go`, `embedder.go`). |
| `dbtest` | Ayudas para tests SQL contra Postgres real. |

Las migraciones (`backend/migrations/`) están embebidas con `go:embed`, se aplican en orden por nombre, una vez cada una y cada una en su transacción; las ya aplicadas no se editan (`migrations.go`).

## 3. Modelo de datos y pgvector

Tablas por migración: `users`, `sessions` (0001); `services` (0002); `incidents`, `timeline_events` (0003); columnas `escalated_at` (0004), `acknowledged_at` (0005), `root_cause`, `resolved_at`, `closed_at` y tabla `postmortems` (0006); `runbooks` (0007).

- `runbooks.embedding` es `public.vector(768) NOT NULL`: un runbook solo se guarda junto con su embedding (BR-12). La migración `0007` ejecuta `CREATE EXTENSION IF NOT EXISTS vector`, que exige superusuario.
- La búsqueda ordena por distancia coseno (`<=>`) y filtra por similitud mínima (`runbook/postgres.go`). Solo hay un índice btree sobre `runbooks.service_id`; no hay índice vectorial.
- 768 dimensiones corresponden a `nomic-embed-text`; otro modelo exige una migración nueva.

## 4. LLM, RAG y copiloto

- **Embeddings:** siempre Ollama (`llm.NewOllamaEmbedder`, `OLLAMA_URL`, `EMBEDDING_MODEL`).
- **Chat:** `chatConfigFromEnv` (`main.go`) elige con `LLM_PROVIDER`: `ollama` (por defecto; `OLLAMA_URL/v1`, sin clave real) o `groq` (exige `GROQ_API_KEY`; `LLM_BASE_URL`). `LLM_MODEL` tiene un valor por defecto por proveedor. Un proveedor desconocido o una clave ausente impiden el arranque. Ambos usan el mismo cliente compatible con OpenAI (`llm/chat.go`).
- **Consumidores del chat:** postmortem (UC-08) y copiloto (UC-10).
- **Copiloto:** `copilot.Service.Ask` recupera hasta 3 runbooks (`ragTopK`) con similitud mínima `RAG_MIN_SIMILARITY` (0.5 por defecto), conversa con el modelo y ejecuta tools en un bucle acotado (`MaxRounds`). Las tools son abrir incidente, agregar nota y listar abiertos; llaman a los services con el usuario autenticado, nunca a SQL (BR-14). Sin runbook relevante ni tools usadas, la respuesta es fija.

## 5. Frontend

- Vite + React + TypeScript + Tailwind (`@tailwindcss/vite`).
- `src/api/`: único lugar con `fetch` (`client.ts`, `credentials: 'same-origin'`) y tipos de la API, un módulo por recurso.
- `src/features/<feature>/`: `auth`, `incidents`, `services`, `runbooks`, `copilot`, `metrics`, `dashboard`.
- `src/shell/`: `AppShell`, `Sidebar`, `Topbar` y `useView.ts`; la vista activa vive en el hash de la URL (`#/dashboard`, `#/incidentes`, `#/servicios`, `#/metricas`), sin librería de router.
- `src/ui/`: componentes compartidos (Button, Card, Modal, Drawer, etc.).
- Elementos de los criterios de aceptación llevan `data-testid` (`frontend/AGENTS.md`).

## 6. Verificación

- `make verify`: build, vet, golangci-lint y tests del backend; lint y build (con typecheck) del frontend. Es el único comando de verificación y lo ejecutan el hook Stop y el pre-commit.
- `make verify-db`: tests de backend, SQL incluidos, contra un Postgres descartable (`scripts/verify-db.sh`, `scripts/lib/disposable-pg.sh`; requiere Docker). Los tests leen `TEST_DATABASE_URL`.
- `make e2e`: Playwright (`e2e/`) contra backend y frontend reales y Postgres descartable (`scripts/e2e.sh`, `e2e/playwright.config.ts`). `e2e/fake-ollama.mjs` reemplaza a Ollama.
- `make verify-changed`: bucle rápido local; no reemplaza a `verify`.
- Reglas de negocio: tests table-driven en service con repositorios fake, un test por BR-xx.

## 7. Decisiones

### D1. Capas handler → service → repository
Estado: vigente. Las reglas de negocio solo viven en service; el handler traduce errores a HTTP y el repository es SQL. Fuente: `backend/AGENTS.md`.

### D2. Invariantes por diseño
Estado: vigente. El repositorio de eventos no expone Update ni Delete (BR-09); las transiciones salen de una tabla explícita consultada por una única función (BR-08). Fuente: `backend/AGENTS.md`.

### D3. Cliente de chat único compatible con OpenAI
Estado: vigente. LangChainGo v0.1.15 no soporta tool calling con su cliente nativo de Ollama, así que Ollama (`/v1`) y Groq comparten `llm/chat.go`; el proveedor se cambia por entorno sin tocar código. Fuente: `llm/chat.go`, `docs/bitacora.md`.

### D4. Garantías de RAG en el backend
Estado: vigente. Las fuentes mostradas son los runbooks recuperados por el backend, y sin runbook relevante ni tools la respuesta es fija, para que el modelo no invente procedimientos. Fuente: `copilot/copilot.go`, `docs/bitacora.md`.

### D5. Vistas por hash, sin router
Estado: vigente. Evita una dependencia para cuatro vistas. Fuente: comentario en `frontend/src/shell/useView.ts`.

### D6. Proxy de Vite solo para desarrollo
Estado: vigente. En desarrollo el navegador usa un único origen (Vite) y `/api` se reenvía al backend. Fuente: `frontend/vite.config.ts`.

### D7. El backend sirve el frontend compilado en producción
Estado: **Decidido; pendiente de implementar** (aprobado por el usuario el 2026-10-20). Hoy el backend no sirve archivos estáticos.

Decisión: en producción el backend Go sirve `frontend/dist` como archivos estáticos, con fallback a `index.html` para toda ruta que no empiece con `/api`. Un solo binario, un solo origen, sin CORS. El proxy de Vite sigue siendo solo de desarrollo (D6).

Implementarla requeriría:
- Un handler estático en `backend/internal/httpapi` (rutas `/api/...` intactas, fallback a `index.html` para el resto) y su cableado en `New`.
- Decidir cómo llega `frontend/dist` al binario (`go:embed` o directorio en disco) y un paso de build en el `Makefile` (hoy `make verify` ya ejecuta `npm run build`, pero no empaqueta nada). No existe Dockerfile.
- Un test del handler: archivo existente, ruta desconocida que cae en `index.html` y `/api/...` inexistente que no cae en `index.html`.
- Actualizar el README, que hoy describe esto como pendiente.

## 8. Preguntas abiertas

- ¿`frontend/dist` se embebe en el binario o se sirve desde disco? (parte de D7)
- ¿Cómo se despliega en producción (Dockerfile, CI)? El `Makefile` indica que hoy no hay CI y no hay Dockerfile en el repo.
- ¿Hace falta un índice vectorial (HNSW/IVFFlat) en `runbooks.embedding`? Hoy no existe; la búsqueda recorre las filas filtradas.
- La cookie de sesión lleva `HttpOnly` y `SameSite=Lax`, pero no `Secure` (la bitácora lo registra como hallazgo del reviewer); no está decidido cuándo se activa.
