# Dominio — Incident Room

Fuente de verdad de las reglas de negocio. Los specs (`specs/UC-XX-*.md`) referencian estas reglas por ID (BR-xx).
Lo marcado como **[BORRADOR]** o **[SUPUESTO]** no figura en las decisiones tomadas ni en la propuesta: está pendiente de confirmación en "Preguntas abiertas" (PA-xx).

## 1. Entidades

Convenciones: los identificadores son UUID; todo instante se guarda en UTC (`timestamptz`); el instante "actual" siempre proviene del reloj inyectable (BR-05).

### User
| Campo | Tipo | Notas |
|---|---|---|
| id | UUID | |
| name | texto | obligatorio |
| email | texto | único, obligatorio |
| password_hash | texto | nunca se expone por API |
| role | enum `ingeniero` \| `oncall` \| `admin` | obligatorio |
| created_at | instante | |

### Service
| Campo | Tipo | Notas |
|---|---|---|
| id | UUID | |
| name | texto | único, obligatorio |
| criticality | enum `critica` \| `importante` \| `estandar` | **[BORRADOR]** niveles a confirmar (PA-01) |
| oncall_user_id | UUID → User, nullable | usuario con rol `oncall` responsable del servicio (PA-03) |

### Incident
| Campo | Tipo | Notas |
|---|---|---|
| id | UUID | |
| title | texto | obligatorio |
| description | texto | |
| service_id | UUID → Service | obligatorio |
| impact | enum `caida_total` \| `degradacion` \| `menor` | obligatorio al declarar |
| suggested_severity | enum `SEV1` \| `SEV2` \| `SEV3` | calculada por BR-01, inmutable |
| severity | enum `SEV1` \| `SEV2` \| `SEV3` | inicia igual a `suggested_severity`; cambios por BR-02 |
| state | enum `declarado` \| `reconocido` \| `mitigando` \| `resuelto` \| `cerrado` | inicia en `declarado` |
| declared_by | UUID → User | |
| assigned_to | UUID → User, nullable | on-call del servicio al declarar (BR-11) |
| root_cause | texto, nullable | obligatorio para pasar a `resuelto` (BR-06) |
| declared_at | instante | base de MTTR y SLA |
| acknowledged_at | instante, nullable | se fija al pasar a `reconocido` |
| resolved_at | instante, nullable | se fija al pasar a `resuelto` |
| closed_at | instante, nullable | |
| escalated_at | instante, nullable | se fija al escalar por SLA (BR-04) |

### TimelineEvent (append-only, BR-09)
| Campo | Tipo | Notas |
|---|---|---|
| id | UUID | |
| incident_id | UUID → Incident | |
| type | enum `declaracion` \| `nota` \| `cambio_estado` \| `cambio_severidad` \| `asignacion` \| `escalado` \| `postmortem` | |
| author_id | UUID → User, nullable | `null` = evento generado por el sistema (ej. escalado) |
| body | texto | contenido legible (nota, causa raíz, motivo) |
| data | JSON | datos estructurados (ej. `{"from":"SEV2","to":"SEV1"}`) |
| occurred_at | instante | reloj inyectable |

### Runbook
| Campo | Tipo | Notas |
|---|---|---|
| id | UUID | |
| service_id | UUID → Service | un servicio puede tener varios runbooks |
| title | texto | obligatorio |
| content | texto | obligatorio; es lo que se indexa para RAG |
| embedding | vector (pgvector) | derivado de `content`; se recalcula al editar |
| created_by | UUID → User | |
| updated_at | instante | |

Semántica de la baja (física o lógica): ver PA-10.

### Postmortem
| Campo | Tipo | Notas |
|---|---|---|
| id | UUID | |
| incident_id | UUID → Incident | único: un postmortem por incidente |
| content | texto | borrador generado por LLM a partir de la timeline, editable |
| status | enum `borrador` \| `aprobado` | |
| generated_by_llm | booleano | indica si el texto inicial fue generado |
| approved_by | UUID → User, nullable | |
| approved_at | instante, nullable | |
| updated_at | instante | |

## 2. Máquina de estados del incidente

Flujo lineal: `Declarado → Reconocido → Mitigando → Resuelto → Cerrado`.
Toda transición no listada es inválida (ver PA-02 para reapertura y saltos).
"On-call del servicio" = usuario con rol `oncall` que es `oncall_user_id` del servicio del incidente (BR-11).

| # | Desde | Hacia | Rol que puede ejecutarla | Precondición | Efecto |
|---|---|---|---|---|---|
| T1 | Declarado | Reconocido | On-call del servicio · Admin | — | `acknowledged_at = ahora`; evento `cambio_estado` |
| T2 | Reconocido | Mitigando | On-call del servicio · Admin | — | evento `cambio_estado` |
| T3 | Mitigando | Resuelto | On-call del servicio · Admin | causa raíz no vacía (BR-06) | `resolved_at = ahora`; guarda `root_cause`; evento `cambio_estado` |
| T4 | Resuelto | Cerrado | On-call del servicio · Admin | si `severity = SEV1`: postmortem `aprobado` (BR-07) | `closed_at = ahora`; evento `cambio_estado` |

El rol `ingeniero` no ejecuta ninguna transición.

## 3. Reglas de negocio

### BR-01 — Severidad sugerida por criticidad × impacto
**Enunciado:** al declarar un incidente el sistema calcula `suggested_severity` con la tabla siguiente y la usa como `severity` inicial. Es una sugerencia: el usuario puede cambiarla (BR-02).

| Criticidad del servicio \ Impacto | Caída total | Degradación | Menor |
|---|---|---|---|
| `critica` | SEV1 | SEV2 | SEV3 |
| `importante` | SEV2 | SEV2 | SEV3 |
| `estandar` | SEV2 | SEV3 | SEV3 |

**[BORRADOR]** Los valores de la tabla y los niveles de criticidad no están definidos en las decisiones; ver PA-01.
**Casos de uso:** UC-02.
**Ejemplo:** servicio `payments` (criticidad `critica`), impacto `caida_total` → sugerida SEV1. Servicio `reports` (`estandar`), impacto `degradacion` → SEV3.

### BR-02 — Cambio de severidad queda registrado en la timeline
**Enunciado:** si el usuario declara con una severidad distinta de la sugerida, o la cambia después, se agrega un evento `cambio_severidad` con severidad anterior, nueva y autor. `suggested_severity` nunca se modifica. Quién puede cambiarla: PA-05.
**Casos de uso:** UC-02, UC-05.
**Ejemplo:** sugerida SEV2, el ingeniero declara SEV1 → timeline contiene `cambio_severidad` con `{"from":"SEV2","to":"SEV1"}` y su autor.

### BR-03 — SLA de reconocimiento por severidad
**Enunciado:** el plazo para reconocer un incidente desde `declared_at` es SEV1 = 5 min, SEV2 = 15 min, SEV3 = 60 min. Los valores son configurables por entorno; estos son los valores por defecto. El plazo se toma de la severidad vigente en el momento de evaluar (PA-04).
**Casos de uso:** UC-04, UC-09.
**Ejemplo:** SEV1 declarado a las 10:00:00 → plazo vence a las 10:05:00.

### BR-04 — Escalado automático al vencer el SLA
**Enunciado:** si un incidente sigue en `declarado` cuando `ahora > declared_at + plazo(severity)`, el sistema (a) notifica/escala a los usuarios `admin`, (b) fija `escalated_at` y (c) agrega un evento `escalado` con `author_id = null`. Ocurre una sola vez por incidente. Un incidente reconocido a tiempo nunca escala. Destinatarios y caso sin on-call: PA-08.
**Casos de uso:** UC-04, UC-05.
**Ejemplo:** SEV2 declarado 10:00, sin acuse; a las 10:15:01 se registra `escalado`. Si se reconoce a las 10:14:59, no hay evento.

### BR-05 — Reloj inyectable
**Enunciado:** ninguna regla del dominio lee la hora del sistema directamente; recibe el instante actual mediante un reloj inyectable. Los tests fijan y avanzan ese reloj para verificar vencimientos de SLA (BR-03/04), MTTR y timestamps.
**Casos de uso:** UC-02, UC-04, UC-05, UC-06, UC-09.
**Ejemplo:** test con reloj fijo en 10:00 declara un SEV1, avanza a 10:05:01 y verifica un único evento `escalado`.

### BR-06 — Resolver requiere causa raíz
**Enunciado:** la transición T3 (`mitigando → resuelto`) se rechaza si `root_cause` está vacío o solo tiene espacios. El estado no cambia y no se agrega evento.
**Casos de uso:** UC-06.
**Ejemplo:** resolver sin causa raíz → error de validación, estado sigue `mitigando`. Con "Pool de conexiones agotado por fuga en v2.3" → pasa a `resuelto`.

### BR-07 — Cerrar un SEV1 requiere postmortem aprobado
**Enunciado:** la transición T4 (`resuelto → cerrado`) se rechaza si `severity = SEV1` y el incidente no tiene un postmortem con `status = aprobado`. Para SEV2 y SEV3 no hay precondición adicional (PA-09).
**Casos de uso:** UC-06, UC-08.
**Ejemplo:** SEV1 resuelto con postmortem en `borrador` → cerrar falla. Tras aprobarlo → cierra. Un SEV2 resuelto cierra sin postmortem.

### BR-08 — Transiciones válidas
**Enunciado:** solo son válidas las transiciones T1–T4 de la sección 2, ejecutadas por los roles indicados. Cualquier otra combinación (saltar estados, retroceder) se rechaza sin cambios ni eventos.
**Casos de uso:** UC-06.
**Ejemplo:** pasar de `declarado` a `resuelto` → rechazado. Un `ingeniero` intentando T1 → prohibido.

### BR-09 — Timeline append-only
**Enunciado:** los eventos de timeline solo se insertan; ninguna operación (API, UI, base de datos de la aplicación) los modifica ni elimina. Se listan en orden cronológico por `occurred_at`. Generan evento: declaración, nota, cambio de estado, cambio de severidad, asignación, escalado y acciones de postmortem (generación, aprobación).
**Casos de uso:** UC-02, UC-04, UC-05, UC-06, UC-08, UC-10.
**Ejemplo:** `PUT`/`DELETE` sobre un evento → 405/403; el evento original permanece intacto. Una corrección se hace agregando una nueva nota.

### BR-10 — Permisos por rol
**Enunciado:** cada acción exige el rol indicado en la matriz de la sección 4; sin sesión válida solo son accesibles registro e inicio de sesión. El acceso denegado no produce efectos ni eventos.
**Casos de uso:** todos.
**Ejemplo:** un `ingeniero` intenta crear un runbook → prohibido; un `admin` lo crea.

### BR-11 — El on-call opera solo en sus servicios
**Enunciado:** un usuario `oncall` solo puede reconocer y transicionar incidentes de servicios donde es `oncall_user_id`. Al declarar, `assigned_to` toma el on-call del servicio (si existe). `admin` no tiene esta restricción.
**Casos de uso:** UC-04, UC-06.
**Ejemplo:** el on-call de `payments` intenta reconocer un incidente de `search` → prohibido.

### BR-12 — Gestión de servicios, runbooks y usuarios
**Enunciado:** solo `admin` da de alta, edita y da de baja servicios, runbooks y usuarios (incluido asignar rol y on-call de servicio). Todo usuario autenticado puede leer runbooks (los consulta el copiloto, BR-14). **[SUPUESTO]** PA-03, PA-06.
**Casos de uso:** UC-01, UC-04, UC-07, UC-10.
**Ejemplo:** un `oncall` edita un runbook → prohibido; el admin lo edita y `updated_at` cambia.

### BR-13 — Postmortem asistido
**Enunciado:** el postmortem se genera como `borrador` por un LLM a partir de la timeline del incidente, es editable mientras esté en `borrador` y se aprueba explícitamente (`approved_by`, `approved_at`). Hay a lo sumo un postmortem por incidente. Quién genera, edita y aprueba, y si un aprobado es inmutable: PA-07.
**Casos de uso:** UC-08.
**Ejemplo:** incidente con 6 eventos → borrador cuyo contenido cita esos eventos; tras editar y aprobar, `status = aprobado`.

### BR-14 — El copiloto actúa con los permisos del usuario
**Enunciado:** las herramientas del copiloto (abrir incidente, agregar nota, listar abiertos) se ejecutan como el usuario autenticado y aplican las mismas reglas BR-01…BR-13 que la UI/API; el evento resultante en la timeline registra a ese usuario como autor.
**Casos de uso:** UC-10.
**Ejemplo:** un `ingeniero` pide al copiloto "abrí un incidente en payments, caída total" → se crea con severidad sugerida por BR-01; pedirle "cerrá el incidente" no está disponible como herramienta.

### BR-15 — Incidente activo
**Enunciado:** un incidente está activo si `state ≠ cerrado`. El tablero y la herramienta "listar abiertos" del copiloto muestran solo activos, y admiten filtros por severidad, servicio y estado. **[SUPUESTO]** PA-11.
**Casos de uso:** UC-03, UC-10.
**Ejemplo:** un incidente `resuelto` sin cerrar aparece en el tablero; uno `cerrado` no.

### BR-16 — MTTR
**Enunciado:** `MTTR_incidente = resolved_at − declared_at`. MTTR del período = promedio aritmético de esa diferencia sobre los incidentes con `resolved_at` dentro del período (PA-12). Incidentes sin resolver no entran al cálculo. Sin datos en el período: sin valor (no 0).
**Casos de uso:** UC-09.
**Ejemplo:** A declarado 10:00 resuelto 10:30; B declarado 11:00 resuelto 12:00 → MTTR = 45 min.

### BR-17 — Incidentes por servicio
**Enunciado:** cantidad de incidentes con `declared_at` dentro del período, agrupados por servicio; incluye servicios con 0.
**Casos de uso:** UC-09.
**Ejemplo:** período con 3 incidentes en `payments` y 1 en `search` → `payments = 3`, `search = 1`.

### BR-18 — Cumplimiento de SLA
**Enunciado:** un incidente cumple el SLA si `acknowledged_at ≤ declared_at + plazo(severity)` (BR-03). Cumplimiento del período = incidentes que cumplen ÷ incidentes con `declared_at` en el período **cuyo plazo ya venció o que fueron reconocidos**, expresado en porcentaje. Un incidente aún dentro de plazo y sin reconocer no entra ni al numerador ni al denominador (PA-12). Sin incidentes elegibles: sin valor.
**Casos de uso:** UC-09, UC-04.
**Ejemplo:** 4 incidentes elegibles, 3 reconocidos a tiempo y 1 escalado → 75 %.

## 4. Matriz de permisos por rol

✔ = permitido · ✖ = prohibido · "(propio)" = solo en sus servicios (BR-11) · PA-xx = pendiente de definir

| Acción | Ingeniero | On-call | Admin |
|---|---|---|---|
| Registrarse / iniciar sesión | ✔ | ✔ | ✔ |
| Declarar incidente | ✔ | ✔ [SUPUESTO] | ✔ |
| Agregar nota | ✔ | ✔ [SUPUESTO] | ✔ |
| Ver tablero, timeline, dashboard, runbooks | ✔ [SUPUESTO] | ✔ | ✔ |
| Cambiar severidad | PA-05 | PA-05 | ✔ |
| Reconocer incidente (T1) | ✖ | ✔ (propio) | ✔ |
| Transicionar T2–T4 | ✖ | ✔ (propio) | ✔ |
| Generar / editar postmortem | PA-07 | PA-07 | ✔ |
| Aprobar postmortem | ✖ [SUPUESTO] | PA-07 | ✔ |
| Gestionar runbooks (alta/edición/baja) | ✖ | ✖ | ✔ |
| Gestionar servicios y asignar on-call | ✖ | ✖ | ✔ |
| Gestionar usuarios y roles | ✖ | ✖ | ✔ |
| Usar el copiloto | ✔ (según BR-14) | ✔ (según BR-14) | ✔ (según BR-14) |
| Recibir escalado por SLA vencido | ✖ | ✖ | ✔ |

## 5. Métricas (definición exacta)

Todas se calculan sobre un período `[desde, hasta)` en UTC, con el reloj inyectable.

| Métrica | Fórmula | Regla |
|---|---|---|
| MTTR | `resolved_at − declared_at`, promedio del período | BR-16 |
| Incidentes por servicio | conteo por `service_id` con `declared_at ∈ período` | BR-17 |
| Cumplimiento de SLA | `cumplen ÷ elegibles × 100` | BR-18 |

## 6. Preguntas abiertas

Nada de esto se inventó: son reglas que ni las decisiones ni la propuesta definen. Donde fue imprescindible escribir algo, quedó marcado **[BORRADOR]** / **[SUPUESTO]** arriba.

- **PA-01** — Niveles de criticidad de servicio y valores de la tabla de BR-01. Propuse 3 niveles (`critica`/`importante`/`estandar`) y una tabla como borrador.
- **PA-02** — Transiciones no listadas: ¿hay reapertura (ej. `resuelto → mitigando`)? ¿se permiten saltos? Hoy: solo T1–T4.
- **PA-03** — ¿Quién asigna el on-call de un servicio (UC-04)? ¿Hay uno solo por servicio o una lista/rotación? Hoy: uno solo, asignado por admin.
- **PA-04** — Si cambia la severidad de un incidente `declarado`, ¿el plazo de SLA se recalcula con la nueva severidad y cuenta desde `declared_at` o desde el cambio? Hoy: severidad vigente, desde `declared_at`.
- **PA-05** — ¿Quién puede cambiar la severidad (BR-02): quien declara, on-call, admin? Hoy solo se garantiza a admin.
- **PA-06** — Registro (UC-01): ¿qué rol recibe un usuario nuevo y quién lo promueve a On-call/Admin? Hoy: nace `ingeniero`, promueve el admin. Además ¿el admin inicial se siembra por entorno?
- **PA-07** — Postmortem: ¿quién puede generarlo, editarlo y aprobarlo? ¿Un aprobado es inmutable? ¿Se puede generar en cualquier estado o desde `resuelto`?
- **PA-08** — Escalado: ¿"al admin" es a todos los admins o a uno? ¿Cómo se notifica (solo evento en timeline/UI, o además email)? ¿Qué pasa si el servicio no tiene on-call asignado?
- **PA-09** — ¿SEV2/SEV3 requieren algo para cerrar más allá de estar `resuelto`? Hoy: nada.
- **PA-10** — Baja de runbook: ¿física o lógica? Si es lógica, ¿deja de indexarse para RAG?
- **PA-11** — Definición de "activo/abierto": ¿incluye `resuelto`? Hoy: todo lo que no está `cerrado`.
- **PA-12** — Período de las métricas: ¿MTTR se filtra por fecha de resolución y SLA por fecha de declaración (como está escrito)? ¿Qué granularidad de período ofrece el dashboard (rango libre, 7/30 días)?
- **PA-13** — Visibilidad: ¿todos los roles autenticados ven todos los incidentes y el dashboard, o el ingeniero solo los suyos? Hoy: todos ven todo.
- **PA-14** — ¿On-call puede declarar incidentes y agregar notas? (Las decisiones lo dan al Ingeniero; se supuso que On-call también.)
