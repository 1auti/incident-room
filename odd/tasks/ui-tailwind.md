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
- [ ] T4 Auth forms
- [ ] T5 Incidents (board, declare, detail, timeline, postmortem)
- [ ] T6 Services, runbooks, metrics
- [ ] T7 Copilot
- [ ] T8 Bitácora entry and close

## Route
Delegated direct: one writer (`builder`) per task group (writer trigger: 2+ non-trivial files).

## Progress / evidence
T1-T3 via builder (delegated). make verify observed green (parent spot check). e2e not yet run (planned at T4/T6/T7/close).

## Next step
T4 auth + T5 incidents via builder.
