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
# Stop se dispara en cada respuesta: sin cambios pendientes en fuentes no hay nada que verificar.
# Un cambio solo de documentación (docs/, specs/, odd/, *.md fuera de backend/ y frontend/) no
# puede romper build, vet, tests ni lint, así que no dispara la verificación. Los *.md dentro de
# backend/ y frontend/ son archivos de contexto (AGENTS.md, CLAUDE.md) y tampoco la disparan.
# Cualquier cambio en fuentes sigue corriendo `make verify` completo: no se saltea ni se debilita
# ningún chequeo.
touches_source() {
  local entry path rename=0
  # -z: entradas separadas por NUL, sin comillas ni escapes. Un renombre trae dos entradas
  # ("XY nuevo" y luego "viejo" sin prefijo): se evalúan ambas rutas.
  while IFS= read -r -d '' entry; do
    if ((rename)); then
      path=$entry
      rename=0
    else
      path=${entry:3}
      [[ "${entry:0:2}" == *[RC]* ]] && rename=1
    fi
    case "$path" in
      backend/*.md | frontend/*.md) ;;
      backend/* | frontend/* | Makefile | .claude/hooks/* | .githooks/*) return 0 ;;
    esac
  done < <(git status --porcelain -z)
  return 1
}
touches_source || exit 0

if ! out=$(make verify 2>&1); then
  printf 'make verify falló. Corregí antes de terminar:\n%s\n' "$(tail -n 40 <<<"$out")" >&2
  exit 2
fi
exit 0
