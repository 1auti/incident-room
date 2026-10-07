#!/usr/bin/env bash
# Fast feedback: tests only for what changed against main. It does NOT replace `make verify`
# (which stays the gate for hooks and CI); it is a quick loop for local iteration.
set -euo pipefail
export PATH="$PATH:$HOME/go/bin:$HOME/.local/bin"
cd "$(git rev-parse --show-toplevel)"
# shellcheck source=scripts/lib/go-affected.sh
source scripts/lib/go-affected.sh

base=${BASE_REF:-main}
# Committed, staged and unstaged changes vs the base, plus untracked files.
mapfile -t changed < <({ git diff --name-only --diff-filter=ACMRD "$base" --; git ls-files -o --exclude-standard; } | sort -u)

echo "== verify-changed: Go packages (changed vs $base, plus dependents)"
mapfile -t pkgs < <(go_affected_packages "${changed[@]}")
if ((${#pkgs[@]})); then
  (cd backend && go test "${pkgs[@]}")
else
  echo "no affected Go packages"
fi

echo "== verify-changed: vitest --changed $base"
(cd frontend && npx --no-install vitest run --changed "$base" --passWithNoTests)
