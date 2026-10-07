# agent-code-tools

Branch: `chore/agent-code-tools` (from `main`). Delivery strategy: ask-on-risk. Forecast: ~450 changed lines (config/scripts/docs).
TDD: not applicable (tooling only). Checks: `make verify` + manual hook tests. Runner: make.

## Objective
Configure agent read/write/verify tooling as a durable base: faster, scoped verification and better code navigation.

## Authorized scope (user-approved)
Tooling only. No business code, no migrations, no UC specs touched.

## Tasks
- [x] T1 post-edit hook: scoped `go vet` per edited package; gofumpt + goimports for Go; prettier for TS.
- [x] T2 `.githooks/pre-commit`: staged-only checks (gofmt -l, vet+test on affected packages, oxlint on staged, tsc -b); drop full verify + vite build. Stop hook keeps full `make verify`.
- [x] T3 `.golangci.yml` minimal + `make lint-backend` wired into `verify-backend`.
- [x] T4 `AGENTS.md`: reading guide (rg -n first, Read with offset/limit, gopls/TS LSP, ast-grep).
- [x] T5 `make verify-changed`: Go tests only for packages changed vs `main`; vitest `--changed` for frontend.
- [x] T6 Local tool install (npm -g / go install; no brew on Arch): ast-grep, typescript-language-server, gofumpt, goimports; document in `make setup`/docs.
- [x] T7 frontend: prettier + vitest (justified: no stdlib equivalent), scripts and minimal config.
- [x] T8 `docs/tooling-decisions.md` (implemented / deferred / discarded, with reasons) + `docs/bitacora.md` entry.
- [x] T9 review follow-ups (findings 1-5, 7): deleted-Go-file handling (falls back to all packages; simulated staged deletion rc=0, full package set), base-ref validation (BASE_REF=nonexistent rc=1), docs say no CI exists (Stop hook is the net), `frontend/.vitest/` ignored, go tools pinned + install failures exit non-zero, task doc drift fixed. Commits: 78fc9d2, 69b8ac5, docs commit below.

## Decisions
Implemented: ripgrep guidance, gopls + TS LSP, ast-grep, gofumpt/goimports, prettier, vitest, golangci-lint, scoped hooks, verify-changed.
Deferred (revisit if repo grows): ctags/code map, semgrep, serena MCP.
Discarded: RAG/pgvector over code (3.8k LOC, infra cost > benefit).
Formatter choice: prettier (no overlap with oxlint; biome would duplicate lint). Changeable.

## Acceptance
- `make verify` passes. Each hook exercised with a real edit/commit. No `--no-verify`.
- Decision doc exists and matches the above.

## Route
Delegated direct: one writer (2+ non-trivial files, preparation reading). Per-task commit: `chore(tooling): ...`, no AI attribution.

## Progress / evidence
Commits: T1=e8f17f9, T2=23bab3c, T3=4a8cac2, T4=33feb60, T5=3776da8, T6=3776da8, T7=785ca9f, T8=6ba2c57.
Checks: `make verify` rc=0; `make verify-changed` ok; post-edit hook simulated on Go (format+vet fail rc=2) and TS (prettier+oxlint any rc=2); pre-commit simulated (unformatted Go rc=1, valid Go rc=0 with dependents, unformatted TS rc=1, docs-only rc=0).
Tools: ast-grep 0.45.3, typescript-language-server 6.0.1 (npm, no brew on Arch), gofumpt v0.12.0, goimports (go install).

## Next step
Review and PR (user decision).
