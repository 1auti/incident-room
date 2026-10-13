# Arquitectura

Componentes y decisiones de Incident Room. Las reglas de negocio están en [`domain.md`](domain.md) y los casos de uso en [`../specs/`](../specs/); este documento no las repite.

## Componentes

```
Navegador ──> Frontend (React + Vite) ──/api──> Backend Go (net/http) ──> PostgreSQL + pgvector
                                                    │
                                                    ├──> Ollama: embeddings (siempre)
                                                    └──> Chat compatible con OpenAI: Ollama /v1 o Groq
```

| Componente | Responsabilidad |
|---|---|
| Frontend (`frontend/`) | SPA de una sola pantalla. Una carpeta por feature en `src/features/` y todas las llamadas HTTP en `src/api/`. No reimplementa reglas: muestra lo que el backend permite y los errores que devuelve. |
| Backend (`backend/cmd/api`) | API REST bajo `/api`. Al arrancar aplica las migraciones, valida la configuración del LLM y lanza el ticker de escalado. |
| PostgreSQL + pgvector | Datos del dominio y embeddings de los runbooks (`vector(768)`). Las migraciones de `backend/migrations/` van embebidas en el binario y se aplican en orden, una vez cada una, con un advisory lock. |
| Ollama | Calcula los embeddings con `nomic-embed-text`, al guardar un runbook y al preguntarle al copiloto. |
| Modelo de chat | Escribe el borrador del postmortem (UC-08) y responde en el copiloto (UC-10). `LLM_PROVIDER` elige Ollama o Groq. |

## Backend por capas

Cada paquete de dominio en `internal/` tiene dos partes: el service, que concentra las reglas, y `postgres.go`, que concentra el SQL. La excepción es `timeline`, que no tiene SQL propio: su repositorio lo implementa `incident.PostgresRepository`, para reutilizar el scan y la inserción de eventos. `httpapi` es la capa de handlers. La tabla cubre los paquetes de dominio; `dbtest` es solo infraestructura de pruebas SQL.

| Paquete | Casos de uso |
|---|---|
| `auth`, `user` | UC-01: sesiones por cookie (solo se guarda el hash del token), roles y primer admin |
| `service` | UC-11 y la asignación de on-call de UC-04 |
| `incident` | UC-02, UC-03, UC-04 y UC-06: declaración, tablero, escalado y transiciones |
| `timeline` | UC-05: notas y cambio de severidad |
| `runbook` | UC-07: CRUD de runbooks y búsqueda por similitud |
| `postmortem` | UC-08 |
| `metrics` | UC-09 |
| `copilot` | UC-10 |
| `llm` | Adaptadores de LangChainGo para embeddings y chat |

- **Dependencias:** cada interfaz se declara en el paquete que la consume y se inyecta por constructor desde `main.go`. No hay estado global.
- **Errores de dominio:** son valores que `httpapi` traduce a códigos HTTP. Así, un 403 o un 409 sale de una regla del service, no del handler.
- **`copilot` no importa LangChainGo:** define su propia interfaz `Model`, que implementa `llm`. Cambiar de librería no toca las reglas del copiloto.

## Decisiones que protegen invariantes

| Invariante | Cómo lo garantiza el diseño |
|---|---|
| Timeline append-only (BR-09) | La interfaz `timeline.Repository` solo tiene, para eventos, `AppendEvent` y `ListEvents`, y no existe SQL que modifique o borre de `timeline_events`. Cualquier método sobre `/timeline/{eventID}` responde 405. |
| Transiciones válidas (BR-08) | Un único mapa `transitions` (estado → destinos) en `incident/service.go`, consultado por una sola función. No hay patrón State. |
| SLA y escalado (BR-03, BR-04, BR-05) | Ningún service lee el reloj: reciben `now` inyectado. En `main.go`, un ticker (`ESCALATION_INTERVAL`) escala los incidentes vencidos y se detiene limpio antes de cerrar el pool. |
| Condiciones de carrera con el LLM (BR-13) | El LLM puede tardar minutos. Por eso, al guardar el borrador se vuelve a leer el estado con `SELECT … FOR SHARE` dentro de la transacción, y se rechaza el borrador si el incidente se cerró mientras tanto. |
| El copiloto respeta los permisos (BR-14) | Sus herramientas llaman a los mismos services que la API, con el usuario autenticado. No existe ninguna herramienta para cerrar o transicionar un incidente. |

## Copiloto (RAG y tool calling)

1. La pregunta se convierte en embedding. pgvector devuelve hasta 3 runbooks con similitud coseno de al menos `RAG_MIN_SIMILARITY`.
2. Los runbooks se le pasan al modelo en un mensaje aparte, delimitados como datos y no como instrucciones.
3. El bucle modelo → herramientas → modelo tiene un máximo de 4 rondas. Por pregunta se permite una sola herramienta que escribe, y solo como primera acción.
4. Las fuentes que ve el usuario son los runbooks que recuperó el backend, no lo que cite el modelo. Si no hubo runbook relevante ni herramienta usada, la respuesta es fija y no propone ningún procedimiento.
5. Si el modelo falla después de ejecutar una acción, la API responde 200 con esa acción, para que el usuario no la repita y la duplique.

El chat va siempre por la API compatible con OpenAI (Ollama en `OLLAMA_URL/v1`, o Groq), porque el cliente nativo de Ollama de LangChainGo v0.1.15 no soporta tool calling.

## Pruebas

| Nivel | Dónde | Qué cubre |
|---|---|---|
| Unitarias | `*_test.go` de cada service, con repositorios fake | Una prueba por regla BR-xx, con el ID en el nombre |
| SQL | `postgres_test.go`, con `TEST_DATABASE_URL` | Consultas y transacciones contra Postgres real; cada prueba crea y borra su propio schema |
| E2E | `e2e/uc-XX.spec.ts` (Playwright) | Una prueba por criterio de aceptación, contra la aplicación real. `fake-ollama.mjs` reemplaza a Ollama |

## Decisiones pendientes

- **Despliegue en producción.** En desarrollo, el proxy de Vite sirve `/api` desde el mismo origen. Para producción hay dos caminos y ninguno está decidido:
  - mismo origen detrás de un reverse proxy;
  - CORS con cookies `SameSite=None; Secure`, que además obliga a marcar la cookie de sesión como `Secure`.
- **Purga de sesiones expiradas.** Hoy no se borran de la tabla (registrado en la bitácora de UC-01).
