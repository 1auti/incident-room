# Backend (Go)
Aplica además de `../AGENTS.md`.

## Estructura
- handler → service → repository.
  - handler: decodifica, valida formato y traduce errores de dominio a HTTP. Nada más.
  - service: todas las reglas de negocio (BR-xx de `docs/domain.md`).
  - repository: SQL.
- Interfaces definidas en el paquete que las consume; dependencias por constructor; sin estado global.

## Invariantes que el diseño debe hacer imposibles de violar
- Timeline append-only: el repositorio de eventos expone inserción y consultas; no existe Update ni Delete.
- Transiciones de estado: una tabla explícita (map estado → destinos válidos) en service.
  Toda transición pasa por una única función que la consulta. Sin patrón State ni jerarquías.

## Go
- `context.Context` como primer parámetro en todo lo que haga I/O.
- Errores envueltos con `fmt.Errorf("...: %w", err)`; errores de dominio como valores o tipos que el handler traduce.
- Sin `panic` fuera de `main`.

## Tests
- Cada regla BR-xx tiene un test table-driven cuyo nombre incluye el ID (ej: `TestBR03_SEV1RequierePostmortem`).
- Reglas de negocio: se testean en service con repositorios fake. SQL: contra Postgres real.

## Copiloto (LangChainGo)
- Consultá LangChainGo con Context7 antes de usar cualquier API suya.
- Las tools del copiloto llaman a service, nunca a repository ni a SQL directo:
  las reglas de negocio aplican igual por chat que por UI.
- Proveedor y modelo del LLM se configuran por entorno (base URL, modelo) para alternar Ollama/Groq sin tocar código.
