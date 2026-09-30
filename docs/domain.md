# Dominio — Incident Room

Fuente de verdad de las reglas de negocio. Los specs (`specs/UC-XX-*.md`) referencian estas reglas por ID (BR-xx).
Las preguntas abiertas de la primera versión (PA-01..PA-14) están resueltas; la sección 6 registra cada decisión y la regla donde quedó incorporada.

## 1. Entidades

Convenciones: los identificadores son UUID; todo instante se guarda en UTC (`timestamptz`); el instante "actual" siempre proviene del reloj inyectable (BR-05).

### User
| Campo | Tipo | Notas |
|---|---|---|
| id | UUID | |
| name | texto | obligatorio |
| email | texto | único, obligatorio |
| password_hash | texto | nunca se expone por API |
| role | enum `ingeniero` \| `oncall` \| `admin` | al registrarse es `ingeniero`; solo el admin lo cambia (BR-19) |
| created_at | instante | |

### Service
| Campo | Tipo | Notas |
|---|---|---|
| id | UUID | |
| name | texto | único, obligatorio |
| criticality | enum `critica` \| `importante` \| `estandar` | obligatorio; lo fija el admin (BR-01, BR-12) |
| oncall_user_id | UUID → User, nullable | único on-call del servicio; debe tener rol `oncall`; lo asigna el admin (BR-11) |

### Incident
| Campo | Tipo | Notas |
|---|---|---|
| id | UUID | |
| title | texto | obligatorio |
| description | texto | |
| service_id | UUID → Service | obligatorio |
| impact | enum `caida_total` \| `degradacion` \| `menor` | obligatorio al declarar |
| suggested_severity | enum `SEV1` \| `SEV2` \| `SEV3` | calculada por BR-01, inmutable |
| severity | enum `SEV1` \| `SEV2` \| `SEV3` | la elige quien declara (por defecto la sugerida); cambios por BR-02 |
| state | enum `declarado` \| `reconocido` \| `mitigando` \| `resuelto` \| `cerrado` | inicia en `declarado` |
| declared_by | UUID → User | |
| assigned_to | UUID → User, nullable | on-call del servicio; se actualiza si el admin cambia el on-call (BR-11) |
| root_cause | texto, nullable | obligatorio para pasar a `resuelto` (BR-06) |
| declared_at | instante | base de MTTR y SLA |
| acknowledged_at | instante, nullable | se fija al pasar a `reconocido` |
| resolved_at | instante, nullable | se fija al pasar a `resuelto` |
| closed_at | instante, nullable | |
| escalated_at | instante, nullable | se fija al escalar por SLA (BR-04); no nulo = incidente "Escalado" |

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

La baja es física: elimina el runbook y su embedding, por lo que deja de usarse en RAG (BR-12).

### Postmortem
| Campo | Tipo | Notas |
|---|---|---|
| id | UUID | |
| incident_id | UUID → Incident | único: un postmortem por incidente |
| content | texto | borrador generado por LLM a partir de la timeline, editable |
| status | enum `borrador` \| `aprobado` | un postmortem `aprobado` es inmutable (BR-13) |
| generated_by_llm | booleano | indica si el texto inicial fue generado |
| approved_by | UUID → User, nullable | siempre un `admin` |
| approved_at | instante, nullable | |
| updated_at | instante | |

## 2. Máquina de estados del incidente

Flujo lineal: `Declarado → Reconocido → Mitigando → Resuelto → Cerrado`.
Toda transición no listada es inválida: no hay reapertura ni saltos de estado (BR-08).
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
**Enunciado:** al declarar un incidente el sistema calcula `suggested_severity` con la tabla siguiente y la propone como `severity`. Es una sugerencia: quien declara puede elegir otra (BR-02). Los niveles de criticidad son `critica`, `importante` y `estandar`; el admin los fija por servicio.

| Criticidad del servicio \ Impacto | Caída total | Degradación | Menor |
|---|---|---|---|
| `critica` | SEV1 | SEV2 | SEV3 |
| `importante` | SEV2 | SEV2 | SEV3 |
| `estandar` | SEV2 | SEV3 | SEV3 |

**Casos de uso:** UC-02, UC-10.
**Ejemplo:** servicio `payments` (criticidad `critica`), impacto `caida_total` → sugerida SEV1. Servicio `reports` (`estandar`), impacto `degradacion` → SEV3.

### BR-02 — Quién y cuándo cambia la severidad; registro en la timeline
**Enunciado:** quien declara elige la severidad al declarar (por defecto, la sugerida). Después de declarado, solo pueden cambiarla el on-call del servicio y el admin, y solo mientras el estado sea `declarado`, `reconocido` o `mitigando`; desde `resuelto` la severidad queda fija. Todo valor distinto del sugerido al declarar, y todo cambio posterior, agrega un evento `cambio_severidad` con severidad anterior, nueva y autor. `suggested_severity` nunca se modifica.
**Motivo del bloqueo desde `resuelto`:** impedir, por ejemplo, bajar un SEV1 a SEV2 para cerrarlo sin postmortem (BR-07).
**Casos de uso:** UC-02, UC-05.
**Ejemplo:** sugerida SEV2, el ingeniero declara SEV1 → evento `cambio_severidad` `{"from":"SEV2","to":"SEV1"}`. Un ingeniero intenta cambiar la severidad de un incidente ya declarado → prohibido. El admin intenta bajar a SEV2 un SEV1 `resuelto` → rechazado.

### BR-03 — SLA de reconocimiento por severidad
**Enunciado:** el plazo para reconocer un incidente es SEV1 = 5 min, SEV2 = 15 min, SEV3 = 60 min. Los valores son configurables por entorno; estos son los valores por defecto. El plazo se toma de la severidad vigente al momento de evaluar y siempre se mide desde `declared_at`.
**Casos de uso:** UC-04, UC-09.
**Ejemplo:** SEV1 declarado a las 10:00:00 → plazo vence a las 10:05:00. Un SEV3 declarado a las 10:00 y subido a SEV1 a las 10:07 sin reconocer tiene el plazo vencido: escala en la siguiente evaluación.

### BR-04 — Escalado automático al vencer el SLA
**Enunciado:** si un incidente sigue en `declarado` cuando `ahora > declared_at + plazo(severity)`, el sistema (a) fija `escalated_at`, (b) agrega un evento `escalado` con `author_id = null` y (c) lo escala a todos los usuarios `admin`: el incidente se muestra con el indicador "Escalado" en el tablero y en su detalle. No hay otros canales de notificación (email, chat). Ocurre una sola vez por incidente. Un incidente reconocido a tiempo nunca escala. Si el servicio no tiene on-call, el incidente queda sin asignar y escala por esta misma regla.
**Casos de uso:** UC-03, UC-04, UC-05.
**Ejemplo:** SEV2 declarado 10:00, sin acuse; a las 10:15:01 se registra `escalado` y aparece el indicador. Si se reconoce a las 10:14:59, no hay evento.

### BR-05 — Reloj inyectable
**Enunciado:** ninguna regla del dominio lee la hora del sistema directamente; recibe el instante actual mediante un reloj inyectable. Los tests fijan y avanzan ese reloj para verificar vencimientos de SLA (BR-03/04), MTTR y timestamps.
**Casos de uso:** UC-02, UC-04, UC-05, UC-06, UC-09.
**Ejemplo:** test con reloj fijo en 10:00 declara un SEV1, avanza a 10:05:01 y verifica un único evento `escalado`.

### BR-06 — Resolver requiere causa raíz
**Enunciado:** la transición T3 (`mitigando → resuelto`) se rechaza si `root_cause` está vacío o solo tiene espacios. El estado no cambia y no se agrega evento.
**Casos de uso:** UC-06.
**Ejemplo:** resolver sin causa raíz → error de validación, estado sigue `mitigando`. Con "Pool de conexiones agotado por fuga en v2.3" → pasa a `resuelto`.

### BR-07 — Cerrar un SEV1 requiere postmortem aprobado
**Enunciado:** la transición T4 (`resuelto → cerrado`) se rechaza si `severity = SEV1` y el incidente no tiene un postmortem con `status = aprobado`. Para SEV2 y SEV3 no hay precondición adicional: el postmortem es opcional.
**Casos de uso:** UC-06, UC-08.
**Ejemplo:** SEV1 resuelto con postmortem en `borrador` → cerrar falla. Tras aprobarlo → cierra. Un SEV2 resuelto cierra sin postmortem.

### BR-08 — Transiciones válidas
**Enunciado:** solo son válidas las transiciones T1–T4 de la sección 2, ejecutadas por los roles indicados. No hay reapertura (un incidente `resuelto` o `cerrado` no retrocede) ni saltos de estado. Cualquier otra combinación se rechaza sin cambios ni eventos.
**Casos de uso:** UC-06.
**Ejemplo:** pasar de `declarado` a `resuelto` → rechazado. Pasar de `resuelto` a `mitigando` → rechazado. Un `ingeniero` intentando T1 → prohibido.

### BR-09 — Timeline append-only
**Enunciado:** los eventos de timeline solo se insertan; ninguna operación (API, UI, base de datos de la aplicación) los modifica ni elimina. Se listan en orden cronológico por `occurred_at`. Generan evento: declaración, nota, cambio de estado, cambio de severidad, asignación, escalado y acciones de postmortem (generación, regeneración, aprobación). La edición del texto de un borrador no genera evento.
**Casos de uso:** UC-02, UC-04, UC-05, UC-06, UC-08, UC-10.
**Ejemplo:** `PUT`/`DELETE` sobre un evento → 405/403; el evento original permanece intacto. Una corrección se hace agregando una nueva nota.

### BR-10 — Permisos por rol y visibilidad
**Enunciado:** cada acción exige el rol indicado en la matriz de la sección 4; sin sesión válida solo son accesibles registro e inicio de sesión. El acceso denegado no produce efectos ni eventos. Todo usuario autenticado, sin importar su rol, ve todos los incidentes, sus timelines, el tablero, el dashboard de métricas y los runbooks.
**Casos de uso:** todos.
**Ejemplo:** un `ingeniero` intenta crear un runbook → prohibido; un `admin` lo crea. Un `ingeniero` abre la timeline de un incidente que no declaró → la ve.

### BR-11 — On-call por servicio y asignación
**Enunciado:** cada servicio tiene como máximo un on-call, asignado por el admin; debe ser un usuario con rol `oncall` (asignar otro rol se rechaza). No hay rotaciones ni turnos. Un usuario `oncall` solo puede reconocer, transicionar, cambiar severidad y trabajar el postmortem de incidentes de servicios donde es `oncall_user_id`; `admin` no tiene esta restricción. Al declarar, `assigned_to` toma el on-call del servicio, o queda vacío si no tiene. Cuando el admin cambia el on-call de un servicio, todos sus incidentes activos (BR-15) pasan a `assigned_to` = nuevo on-call y cada uno recibe un evento `asignacion`.
**Casos de uso:** UC-02, UC-04, UC-05, UC-06, UC-08.
**Ejemplo:** el on-call de `payments` intenta reconocer un incidente de `search` → prohibido. El admin cambia el on-call de `payments` de Ana a Bruno con 2 incidentes activos → ambos quedan asignados a Bruno con un evento `asignacion` cada uno.

### BR-12 — Gestión de servicios, runbooks y usuarios
**Enunciado:** solo `admin` da de alta, edita y da de baja servicios, runbooks y usuarios, incluido asignar rol (BR-19) y on-call de servicio (BR-11). La baja de un runbook es física: elimina el runbook y su embedding, y el copiloto deja de recuperarlo (BR-14).
**Casos de uso:** UC-01, UC-04, UC-07, UC-10.
**Ejemplo:** un `oncall` edita un runbook → prohibido; el admin lo edita y `updated_at` cambia. Tras borrar el runbook "Reinicio de payments", una consulta al copiloto sobre ese tema no lo cita.

### BR-13 — Postmortem asistido
**Enunciado:** hay a lo sumo un postmortem por incidente, de cualquier severidad. El borrador lo genera un LLM a partir de la timeline, y solo se puede generar con el incidente en `resuelto` (la timeline ya incluye la causa raíz). Generar y editar: on-call del servicio o admin, mientras el postmortem esté en `borrador`; generar de nuevo reemplaza el contenido del borrador. Aprobar: solo admin, que fija `approved_by` y `approved_at`. Un postmortem `aprobado` es inmutable: no se edita ni se regenera.
**Casos de uso:** UC-08, UC-06.
**Ejemplo:** incidente `resuelto` con 6 eventos → borrador cuyo contenido cita esos eventos. Generar sobre un incidente `mitigando` → rechazado. El on-call intenta aprobar → prohibido. Editar un postmortem aprobado → rechazado.

### BR-14 — El copiloto actúa con los permisos del usuario
**Enunciado:** las herramientas del copiloto (abrir incidente, agregar nota, listar abiertos) se ejecutan como el usuario autenticado y aplican las mismas reglas que la UI/API; el evento resultante en la timeline registra a ese usuario como autor. Las consultas sobre runbooks (RAG) usan solo los runbooks existentes.
**Casos de uso:** UC-10.
**Ejemplo:** un `ingeniero` pide al copiloto "abrí un incidente en payments, caída total" → se crea con severidad sugerida por BR-01; pedirle "cerrá el incidente" no está disponible como herramienta.

### BR-15 — Incidente activo
**Enunciado:** un incidente está activo si `state ≠ cerrado` (un incidente `resuelto` sin cerrar sigue activo). El tablero y la herramienta "listar abiertos" del copiloto muestran solo activos, y admiten filtros por severidad, servicio y estado.
**Casos de uso:** UC-03, UC-10, UC-04.
**Ejemplo:** un incidente `resuelto` sin cerrar aparece en el tablero; uno `cerrado` no.

### BR-16 — MTTR
**Enunciado:** `MTTR_incidente = resolved_at − declared_at`. MTTR del período = promedio aritmético de esa diferencia sobre los incidentes con `resolved_at` dentro del período. Incidentes sin resolver no entran al cálculo. Sin datos en el período: sin valor (no 0).
**Casos de uso:** UC-09.
**Ejemplo:** A declarado 10:00 resuelto 10:30; B declarado 11:00 resuelto 12:00 → MTTR = 45 min.

### BR-17 — Incidentes por servicio
**Enunciado:** cantidad de incidentes con `declared_at` dentro del período, agrupados por servicio; incluye servicios con 0.
**Casos de uso:** UC-09.
**Ejemplo:** período con 3 incidentes en `payments` y 1 en `search` → `payments = 3`, `search = 1`.

### BR-18 — Cumplimiento de SLA
**Enunciado:** un incidente cumple el SLA si `acknowledged_at ≤ declared_at + plazo(severity)` (BR-03). Cumplimiento del período = incidentes que cumplen ÷ incidentes con `declared_at` en el período **cuyo plazo ya venció o que fueron reconocidos**, expresado en porcentaje. Un incidente aún dentro de plazo y sin reconocer no entra ni al numerador ni al denominador. Sin incidentes elegibles: sin valor.
**Casos de uso:** UC-09, UC-04.
**Ejemplo:** 4 incidentes elegibles, 3 reconocidos a tiempo y 1 escalado → 75 %.

### BR-19 — Registro y rol inicial
**Enunciado:** el registro público crea usuarios con rol `ingeniero`; el email debe ser único. Solo el admin cambia el rol de un usuario (a `oncall` o `admin`). No se puede quitar el rol `oncall` a un usuario que es on-call de algún servicio: primero hay que asignar otro on-call a esos servicios (BR-11). Al iniciar la aplicación, si no existe ningún `admin`, se crea uno con credenciales tomadas de variables de entorno; no hay otro modo de obtener el primer admin.
**Casos de uso:** UC-01.
**Ejemplo:** Ana se registra → queda `ingeniero`. Registrarse con un email existente → rechazado. El admin promueve a Ana a `oncall` → su rol cambia.

## 4. Matriz de permisos por rol

✔ = permitido · ✖ = prohibido · "(propio)" = solo en sus servicios (BR-11)

| Acción | Ingeniero | On-call | Admin |
|---|---|---|---|
| Registrarse / iniciar sesión | ✔ | ✔ | ✔ |
| Declarar incidente (y elegir su severidad) | ✔ | ✔ | ✔ |
| Agregar nota | ✔ | ✔ | ✔ |
| Ver tablero, incidentes, timelines, dashboard, runbooks | ✔ | ✔ | ✔ |
| Cambiar severidad tras declarar (hasta `mitigando`, BR-02) | ✖ | ✔ (propio) | ✔ |
| Reconocer incidente (T1) | ✖ | ✔ (propio) | ✔ |
| Transicionar T2–T4 | ✖ | ✔ (propio) | ✔ |
| Generar / editar postmortem en `borrador` | ✖ | ✔ (propio) | ✔ |
| Aprobar postmortem | ✖ | ✖ | ✔ |
| Gestionar runbooks (alta/edición/baja) | ✖ | ✖ | ✔ |
| Gestionar servicios y asignar on-call | ✖ | ✖ | ✔ |
| Gestionar usuarios y roles | ✖ | ✖ | ✔ |
| Usar el copiloto | ✔ (según BR-14) | ✔ (según BR-14) | ✔ (según BR-14) |
| Recibir escalado por SLA vencido | ✖ | ✖ | ✔ |

## 5. Métricas (definición exacta)

Todas se calculan sobre un período `[desde, hasta)` en UTC, con el reloj inyectable. El dashboard ofrece "últimos 7 días", "últimos 30 días" (por defecto) y rango libre.

| Métrica | Fórmula | Filtro de período | Regla |
|---|---|---|---|
| MTTR | promedio de `resolved_at − declared_at` | `resolved_at ∈ período` | BR-16 |
| Incidentes por servicio | conteo por `service_id` | `declared_at ∈ período` | BR-17 |
| Cumplimiento de SLA | `cumplen ÷ elegibles × 100` | `declared_at ∈ período` | BR-18 |

## 6. Decisiones sobre las preguntas abiertas

Las 14 preguntas de la primera versión las resolvió el agente por delegación explícita del usuario (2026-09-29). Cada una puede revisarse: al cambiarla, se actualiza la regla indicada y los specs que la citan.

| PA | Pregunta | Decisión | Regla |
|---|---|---|---|
| PA-01 | Niveles de criticidad y tabla criticidad × impacto | 3 niveles (`critica`, `importante`, `estandar`); se confirma la tabla propuesta | BR-01 |
| PA-02 | ¿Reapertura o saltos de estado? | No: solo T1–T4 | BR-08 |
| PA-03 | ¿Quién asigna el on-call y cuántos por servicio? | Uno por servicio, asignado por el admin, sin rotaciones; al cambiarlo se reasignan los incidentes activos | BR-11 |
| PA-04 | ¿El SLA se recalcula si cambia la severidad? | Sí, con la severidad vigente, siempre desde `declared_at` | BR-03 |
| PA-05 | ¿Quién cambia la severidad? | Quien declara, al declarar; después, on-call del servicio y admin hasta `mitigando` | BR-02 |
| PA-06 | Rol inicial y promoción | Nace `ingeniero`; promueve el admin; primer admin desde variables de entorno | BR-19 |
| PA-07 | ¿Quién genera, edita y aprueba el postmortem? | Genera/edita on-call del servicio o admin, desde `resuelto`; aprueba solo admin; aprobado es inmutable | BR-13 |
| PA-08 | Destinatarios y canal del escalado; servicio sin on-call | Todos los admins; evento en timeline + indicador "Escalado"; sin email; sin on-call escala por la misma regla | BR-04 |
| PA-09 | ¿SEV2/SEV3 requieren algo para cerrar? | No; el postmortem es opcional | BR-07 |
| PA-10 | Baja de runbook: ¿física o lógica? | Física; se elimina su embedding y deja de usarse en RAG | BR-12 |
| PA-11 | ¿"Activo" incluye `resuelto`? | Sí: activo = `state ≠ cerrado` | BR-15 |
| PA-12 | Filtro y granularidad del período de métricas | MTTR por `resolved_at`; conteo y SLA por `declared_at`; 7 días, 30 días (defecto) o rango libre | BR-16, BR-17, BR-18, sección 5 |
| PA-13 | Visibilidad por rol | Todos los autenticados ven todo | BR-10 |
| PA-14 | ¿On-call declara y agrega notas? | Sí, igual que ingeniero y admin | BR-10, sección 4 |
| — | ¿Regenerar un borrador genera evento? (hallado al escribir specs) | Sí; editar el texto no | BR-09 |
| — | ¿Se puede quitar el rol `oncall` a quien es on-call de un servicio? (hallado al escribir specs) | No, primero se reasigna el servicio | BR-19 |
