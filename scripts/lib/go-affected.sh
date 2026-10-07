#!/usr/bin/env bash
# Sourced helper. go_affected_packages <repo-relative files...> prints, one per line, the Go packages
# (relative to ./backend, e.g. ./internal/incident) that contain a changed file plus every package that
# depends on them. A go.mod/go.sum change selects everything. Files outside backend/ are ignored.
go_affected_packages() {
  local f dirs=() changed=()
  for f in "$@"; do
    case "$f" in
      backend/go.mod | backend/go.sum)
        (cd backend && go list ./...)
        return
        ;;
      backend/*) dirs+=("$(dirname "${f#backend/}")") ;;
    esac
  done
  ((${#dirs[@]})) || return 0
  local d
  while IFS= read -r d; do
    # Skip dirs that are not Go packages anymore (deleted, or no Go files).
    mapfile -t -O "${#changed[@]}" changed < <(cd backend && go list "./$d" 2>/dev/null)
  done < <(printf '%s\n' "${dirs[@]}" | sort -u)
  ((${#changed[@]})) || return 0
  (cd backend && go list -f '{{.ImportPath}} {{join .Deps " "}}' ./... |
    awk -v changed="${changed[*]}" 'BEGIN { n = split(changed, c, " "); for (i = 1; i <= n; i++) want[c[i]] = 1 }
      { for (i = 1; i <= NF; i++) if ($i in want) { print $1; break } }')
}
