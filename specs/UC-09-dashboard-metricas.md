# UC-09 — Dashboard de métricas

**Actor:** Ingeniero · On-call · Admin
**Prioridad:** núcleo  (núcleo = uno de los 7 que no pueden fallar en la demo)

## Objetivo
Consultar MTTR, incidentes por servicio y cumplimiento de SLA para un período, con las definiciones exactas del dominio.

## Reglas de negocio
- BR-16: MTTR = promedio de `resolved_at − declared_at` sobre incidentes con `resolved_at` en el período; sin datos, sin valor.
- BR-17: incidentes con `declared_at` en el período agrupados por servicio, incluidos los servicios con 0.
- BR-18: cumplimiento de SLA = incidentes que cumplen ÷ incidentes elegibles (plazo vencido o reconocidos), en porcentaje; sin elegibles, sin valor.
- BR-03: plazo de reconocimiento por severidad usado por BR-18.
- BR-05: el período y los vencimientos se evalúan con el reloj inyectable.
- BR-10: sin sesión válida no se accede al dashboard; todo usuario autenticado lo ve.
- Sección 5 de `docs/domain.md`: el período es `[desde, hasta)` en UTC; el dashboard ofrece "últimos 7 días", "últimos 30 días" (por defecto) y rango libre.

## Criterios de aceptación
- **UC-09.1** Dado el incidente A declarado 10:00 y resuelto 10:30, y el incidente B declarado 11:00 y resuelto 12:00, ambos resueltos dentro del período, cuando se consulta el dashboard, entonces el MTTR es 45 minutos (BR-16).
- **UC-09.2** Dado un período sin incidentes resueltos, o con incidentes aún sin resolver, cuando se consulta el dashboard, entonces el MTTR se muestra sin valor y no como 0, y los incidentes sin resolver no entran al cálculo; si además no hay incidentes elegibles, el cumplimiento de SLA también se muestra sin valor (BR-16, BR-18).
- **UC-09.3** Dado un período con 3 incidentes en `payments`, 1 en `search` y un servicio sin incidentes, cuando se consulta el dashboard, entonces se muestra `payments = 3`, `search = 1` y el servicio restante con 0 (BR-17).
- **UC-09.4** Dado un período con 4 incidentes elegibles, 3 reconocidos dentro del plazo y 1 escalado, y además un incidente dentro de plazo y sin reconocer, cuando se consulta el dashboard, entonces el cumplimiento de SLA es 75 % y el incidente pendiente no cuenta ni en el numerador ni en el denominador (BR-18, BR-03, BR-05).
- **UC-09.5** Dado incidentes declarados o resueltos dentro y fuera de un rango libre `[desde, hasta)`, cuando se consulta el dashboard con ese rango, entonces solo se consideran los del intervalo, filtrando MTTR por `resolved_at` y el resto por `declared_at` (BR-16, BR-17, BR-18).
- **UC-09.6** Dado un usuario autenticado de cualquier rol, cuando abre el dashboard sin elegir período, entonces se muestra "últimos 30 días" según el reloj inyectable, y al elegir "últimos 7 días" las métricas se recalculan para ese período (BR-10, BR-05).

## Fuera de alcance
- Métricas distintas de MTTR, incidentes por servicio y cumplimiento de SLA.
- Preajustes de período distintos de "últimos 7 días", "últimos 30 días" y rango libre.
- Restricción de visibilidad por rol: no existe, todo usuario autenticado ve el dashboard (BR-10).
- Exportación de datos y alertas basadas en métricas.

## Verificación
- Unit: `TestBR16_MTTRPromedioDelPeriodo`, `TestBR16_SinDatosSinValor`, `TestBR17_IncidentesPorServicioIncluyeCeros`, `TestBR18_CumplimientoSLAElegibles`, `TestBR18_PendienteNoEntra`, `TestBR16_FiltraPorResolvedAt`, `TestBR17_FiltraPorDeclaredAt`, `TestMetricas_PeriodoPorDefecto30Dias` en `backend/internal/metrics/...` (reloj inyectable, BR-05)
- E2E: `e2e/uc-09.spec.ts` (un test por criterio de aceptación)
