# Frontend (React + Vite + TypeScript)
Aplica además de `../AGENTS.md`.

- TypeScript estricto; prohibido `any`. Los tipos de la API se definen una sola vez en `src/api/`.
- Llamadas HTTP solo desde `src/api/`; los componentes no usan `fetch` directo.
- Organización por feature (`src/features/incidents/`, ...), no por tipo de archivo.
- Todo elemento mencionado en un criterio de aceptación lleva un `data-testid` estable
  (lo usan Playwright MCP y los tests E2E).
- La UI no reimplementa reglas de negocio: muestra lo que el backend permite y los errores que devuelve.
