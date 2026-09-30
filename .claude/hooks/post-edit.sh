#!/usr/bin/env bash
# PostToolUse (Edit|Write): feedback rápido sobre el archivo recién editado.
# exit 2 => el stderr vuelve a Claude, que corrige en el mismo turno.
set -uo pipefail
# Sin jq no podemos leer el input (ni el guard anti-loop): fallamos visible y sin bloquear.
command -v jq >/dev/null || { echo "hook: falta jq (sudo pacman -S jq)" >&2; exit 1; }

file=$(jq -r '.tool_input.file_path // empty')
[[ -z "$file" ]] && exit 0
cd "$CLAUDE_PROJECT_DIR"

case "$file" in
  *.go)
    # El formato no es una decisión del agente: se aplica siempre.
    gofmt -w "$file"
    if ! out=$(cd backend && go vet ./... 2>&1); then
      printf 'go vet falló después de editar %s:\n%s\n' "$file" "$(tail -n 40 <<<"$out")" >&2
      exit 2
    fi
    ;;
  */frontend/*.ts | */frontend/*.tsx)
    if ! out=$(cd frontend && npx oxlint "$file" 2>&1); then
      printf 'oxlint falló en %s:\n%s\n' "$file" "$(tail -n 40 <<<"$out")" >&2
      exit 2
    fi
    ;;
esac
exit 0
