#!/usr/bin/env bash
# Stop: antes de que Claude dé el turno por terminado, se verifica todo el proyecto.
set -uo pipefail
# Sin jq no podemos leer el input (ni el guard anti-loop): fallamos visible y sin bloquear.
command -v jq >/dev/null || { echo "hook: falta jq (sudo pacman -S jq)" >&2; exit 1; }
input=$(cat)

# Anti-loop: si Claude ya está continuando por este hook, lo dejamos parar.
# Resultado: un intento automático de corrección; si sigue fallando, decide el humano.
[[ "$(jq -r '.stop_hook_active' <<<"$input")" == "true" ]] && exit 0

cd "$CLAUDE_PROJECT_DIR"
# Stop se dispara en cada respuesta: sin cambios pendientes no hay nada que verificar.
[[ -z "$(git status --porcelain)" ]] && exit 0

if ! out=$(make verify 2>&1); then
  printf 'make verify falló. Corregí antes de terminar:\n%s\n' "$(tail -n 40 <<<"$out")" >&2
  exit 2
fi
exit 0
