#!/usr/bin/env bash
# PostToolUse (Edit|Write): feedback rápido sobre el archivo recién editado.
# exit 2 => el stderr vuelve a Claude, que corrige en el mismo turno.
set -uo pipefail
# Sin jq no podemos leer el input (ni el guard anti-loop): fallamos visible y sin bloquear.
command -v jq >/dev/null || { echo "hook: falta jq (sudo pacman -S jq)" >&2; exit 1; }

file=$(jq -r '.tool_input.file_path // empty')
[[ -z "$file" ]] && exit 0
cd "$CLAUDE_PROJECT_DIR"

# Hooks may run with a minimal PATH: make user-level Go and npm binaries visible.
export PATH="$PATH:$HOME/go/bin:$HOME/.local/bin"

case "$file" in
  "$CLAUDE_PROJECT_DIR"/backend/*.go)
    # El formato no es una decisión del agente: se aplica siempre (imports + gofumpt, que incluye gofmt).
    if command -v goimports >/dev/null; then goimports -w "$file"; fi
    if command -v gofumpt >/dev/null; then gofumpt -w "$file"; else gofmt -w "$file"; fi
    # Vet acotado al paquete del archivo editado: el resto lo cubre el pre-commit y el hook Stop.
    pkg=$(dirname "${file#"$CLAUDE_PROJECT_DIR"/backend/}")
    if ! out=$(cd backend && go vet "./$pkg/" 2>&1); then
      printf 'go vet falló después de editar %s:\n%s\n' "$file" "$(tail -n 40 <<<"$out")" >&2
      exit 2
    fi
    ;;
  "$CLAUDE_PROJECT_DIR"/frontend/*.ts | "$CLAUDE_PROJECT_DIR"/frontend/*.tsx)
    # prettier formatea; oxlint lint: no se pisan (prettier no reglas, oxlint no formato).
    if ! out=$(cd frontend && npx --no-install prettier --write "$file" 2>&1); then
      printf 'prettier falló en %s:\n%s\n' "$file" "$(tail -n 40 <<<"$out")" >&2
      exit 2
    fi
    if ! out=$(cd frontend && npx --no-install oxlint "$file" 2>&1); then
      printf 'oxlint falló en %s:\n%s\n' "$file" "$(tail -n 40 <<<"$out")" >&2
      exit 2
    fi
    ;;
esac
exit 0
