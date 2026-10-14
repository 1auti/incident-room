# ui-tailwind

## Objective
Add Tailwind CSS (v4, `@tailwindcss/vite`) and restyle the frontend with shared presentational components, without changing behavior, UI text, or breaking the e2e suite.

## Constraints
- Keep every `data-testid` / `data-*` attribute on the same element and nesting (e2e relies on them).
- No icons or extra text inside elements asserted with exact `toHaveText`.
- No tabs or responsive `hidden`: all sections stay visible on one page.
- No prettier-tailwind plugin; prettier 120 cols.
- Full plan: `~/.claude/plans/armemos-el-plan-de-refactored-mochi.md`

## Resolved config
- TDD: off (no frontend unit tests; safety net is `make e2e`). Source: project state.
- Checks: `make verify`; `make e2e` at steps 4, 6, 7 and close.
- Delivery strategy: ask-on-risk. Branch: `feat/ui-tailwind`.

## Tasks
- [x] T1 Tailwind base: deps, vite plugin, replace `index.css`
- [x] T2 Shared UI components in `frontend/src/ui/`
- [x] T3 App layout (`App.tsx`)
- [x] T4 Auth forms
- [x] T5 Incidents (board, declare, detail, timeline, postmortem)
- [x] T6 Services, runbooks, metrics
- [x] T7 Copilot
- [x] T8 Bitácora entry and close

## Route
Delegated direct: one writer (`builder`) per task group (writer trigger: 2+ non-trivial files).

## Progress / evidence
T1-T3 via builder (delegated). make verify observed green (parent spot check). T4-T5 via builder: make verify green (spot check, exit 0), make e2e 68 passed (builder report). Escalado badge now span instead of strong (e2e green).
T6-T7 via builder: make verify exit 0 (spot check), make e2e 68 passed (builder report). reviewer: no blockers; optional improvements listed in bitacora. Commits: c4cc3ec, 0bdc169, cf4e4a8, 318ee2c, f72f51a, a8b13ac.

## Next step
Done. Pending: user decision on push / PR.
