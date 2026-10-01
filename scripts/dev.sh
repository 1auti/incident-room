#!/usr/bin/env bash
# Levanta la aplicación completa para usarla en el navegador: Postgres descartable, backend y frontend.
# Ctrl-C detiene todo y elimina el contenedor. Los datos NO persisten entre ejecuciones.
# No usa .env: si no exportás ADMIN_EMAIL y ADMIN_PASSWORD, el admin de ejemplo lleva una contraseña aleatoria.
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
# shellcheck source=lib/disposable-pg.sh
source "$root/scripts/lib/disposable-pg.sh"

api_port="${DEV_API_PORT:-8080}"
web_port="${DEV_WEB_PORT:-5173}"

for tool in go npm curl; do
  command -v "$tool" >/dev/null || { echo "dev: falta '$tool' en el PATH" >&2; exit 1; }
done

pids=()
workdir="$(mktemp -d)"
dev_cleanup() {
  for pid in "${pids[@]:-}"; do
    [ -n "$pid" ] && kill "$pid" >/dev/null 2>&1 || true
  done
  rm -rf "$workdir"
  disposable_pg_cleanup
}

disposable_pg_start dev
trap dev_cleanup EXIT # reemplaza el trap del helper; dev_cleanup también elimina el contenedor
trap 'exit 130' INT TERM # garantiza que Ctrl-C ejecute el trap EXIT

export PORT="$api_port"
export ADMIN_EMAIL="${ADMIN_EMAIL:-admin@incident-room.local}"
if [ -z "${ADMIN_PASSWORD:-}" ]; then
  ADMIN_PASSWORD="$(openssl rand -hex 8)"
  generated_password=1
fi
export ADMIN_PASSWORD

[ -d "$root/frontend/node_modules" ] || (cd "$root/frontend" && npm ci)

# Se compila el binario y se ejecuta directo: `go run` deja un proceso hijo que sobreviviría al kill.
(cd "$root/backend" && go build -o "$workdir/api" ./cmd/api)
"$workdir/api" &
pids+=("$!")

(cd "$root/frontend" && API_PROXY_TARGET="http://127.0.0.1:${api_port}" \
  exec node_modules/.bin/vite --host 127.0.0.1 --port "$web_port" --strictPort) &
pids+=("$!")

wait_for() { # url descripción
  for _ in $(seq 1 60); do
    # Cualquier respuesta HTTP (incluido 401 de la API) indica que el servidor ya escucha.
    curl -s -o /dev/null "$1" && return 0
    sleep 1
  done
  echo "dev: $2 no respondió a tiempo en $1" >&2
  return 1
}
wait_for "http://127.0.0.1:${api_port}/api/me" "el backend"
wait_for "http://127.0.0.1:${web_port}/" "el frontend"

cat <<EOF

Incident Room está listo:
  Aplicación:  http://127.0.0.1:${web_port}
  API:         http://127.0.0.1:${api_port}
  Admin:       ${ADMIN_EMAIL}
  Contraseña:  ${ADMIN_PASSWORD}${generated_password:+  (aleatoria, solo para esta ejecución)}
  Base:        Postgres descartable; los datos se pierden al detener (Ctrl-C).

EOF

# Si cualquiera de los dos procesos termina, se detiene todo.
wait -n
echo "dev: un proceso terminó; deteniendo todo" >&2
exit 1
