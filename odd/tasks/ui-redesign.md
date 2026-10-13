# ui-redesign

## Objective
Redesign the frontend: dark blue/white theme, app shell with sidebar and topbar, KPI dashboard home, separate Services/Incidents/Metrics views, quick "declare incident" modal, Runbooks and Copilot as right-side drawers. No backend changes.

## Constraints
- Keep every `data-testid` / `data-*` and exact texts; e2e assertions stay, only navigation steps change (shared `e2e/helpers.ts`).
- View state via URL hash, no router dependency. UC-01.4 (`/cualquier/ruta` shows login) must still pass.
- KPIs only from existing endpoints (`listIncidents`, `getDashboard`, `listServices`).
- Prettier 120 cols, no `any`, HTTP only in `src/api/`.
- Plan: `~/.claude/plans/armemos-el-plan-de-refactored-mochi.md`

## Resolved config
- TDD: off (no frontend unit tests; pure aggregation helper for KPIs gets a vitest test if cheap). Safety net: `make e2e`.
- Checks: `make verify` per task; `make e2e` after R4, R5, R6 and close.
- Branch: `feat/ui-redesign` (from `feat/ui-tailwind`). Delivery: ask-on-risk.

## Tasks
- [x] R1 Dark theme
- [x] R2 Shell, navigation, Modal/Drawer
- [x] R3 KPI dashboard
- [x] R4 Views (Incidents, Services, Metrics)
- [x] R5 Declare modal + Runbooks/Copilot drawers
- [x] R6 e2e helpers + KPI spec
- [ ] R7 Polish, reviewer, bitacora

## Route
Delegated direct: one `builder` per task group (writer trigger).

## Progress / evidence
R1-R2 via builder: make verify exit 0 (builder report).
R3: vitest 3/3 and make verify green (builder report); wired in App.tsx by orchestrator (tsc ok).
R4-R6: make verify exit 0 and make e2e 70 passed (both re-run by orchestrator). Assertions unchanged; uc-04.1 needed goTo services. e2e intentionally not run (navigation adapted in R6).

## Next step
R7: visual check with screenshots, reviewer, bitacora.
