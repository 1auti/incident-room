#!/usr/bin/env bash
# Sourced helper. go_affected_packages <repo-relative files...> prints, one per line, the Go packages
# (relative to ./backend, e.g. ./internal/incident) that contain a changed file plus every package that
# depends on them. A go.mod/go.sum change selects everything. Files outside backend/ are ignored.
# If a changed .go file lives in a dir that is no longer a Go package (deleted/renamed), the affected set
# cannot be computed safely, so it falls back to every backend package.
go_affected_packages() {
  local f dirs=() changed=() godirs=()
  for f in "$@"; do
    case "$f" in
      backend/go.mod | backend/go.sum)
        (cd backend && go list ./...)
        return
        ;;
      backend/*.go)
        dirs+=("$(dirname "${f#backend/}")")
        godirs+=("$(dirname "${f#backend/}")")
        ;;
      backend/*) dirs+=("$(dirname "${f#backend/}")") ;;
    esac
  done
  ((${#dirs[@]})) || return 0
  local d out
  while IFS= read -r d; do
    if out=$(cd backend && go list "./$d" 2>/dev/null) && [[ -n "$out" ]]; then
      mapfile -t -O "${#changed[@]}" changed <<<"$out"
    elif ((${#godirs[@]})) && printf '%s\n' "${godirs[@]}" | grep -qxF -- "$d"; then
      # A .go file changed in a dir that is not a package anymore: be safe, select everything.
      (cd backend && go list ./...)
      return
    fi
    # Non-Go files in a non-package dir (docs, fixtures) are ignored.
  done < <(printf '%s\n' "${dirs[@]}" | sort -u)
  ((${#changed[@]})) || return 0
  (cd backend && go list -f '{{.ImportPath}} {{join .Deps " "}}' ./... |
    awk -v changed="${changed[*]}" 'BEGIN { n = split(changed, c, " "); for (i = 1; i <= n; i++) want[c[i]] = 1 }
      { for (i = 1; i <= NF; i++) if ($i in want) { print $1; break } }')
}
