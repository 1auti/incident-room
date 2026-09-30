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
