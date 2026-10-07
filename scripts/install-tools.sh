#!/usr/bin/env bash
# Local developer/agent tooling. Go tools are installed with `go install`; the rest are only
# checked, with a hint on how to install them (no sudo, no global package manager side effects).
# `--check`: solo verifica (sin instalar ni imprimir nada si está todo); si falta algo imprime UNA línea.
# Lo usa el hook SessionStart. Sin argumentos el comportamiento es el de siempre.
set -uo pipefail
export PATH="$PATH:$HOME/go/bin:$HOME/.local/bin"

if [[ "${1:-}" == "--check" ]]; then
  missing_list=""
  for bin in jq rg golangci-lint ast-grep typescript-language-server gofumpt goimports gopls; do
    command -v "$bin" >/dev/null || missing_list="$missing_list $bin"
  done
  [[ -n "$missing_list" ]] && echo "Faltan herramientas:$missing_list (corré: make tools)"
  exit 0
fi

# Pinned to the versions the repo was set up with; bump deliberately.
GOFUMPT_VERSION=v0.12.0
GOIMPORTS_VERSION=v0.51.0
GOPLS_VERSION=v0.23.0

missing=0
install_failed=0
goinstall() { # goinstall <module/path@version>
  go install "$1" || { printf 'FAILED   go install %s\n' "$1" >&2; install_failed=1; }
}
goinstall "mvdan.cc/gofumpt@$GOFUMPT_VERSION"
goinstall "golang.org/x/tools/cmd/goimports@$GOIMPORTS_VERSION"
goinstall "golang.org/x/tools/gopls@$GOPLS_VERSION"

check() { # check <binary> <install hint>
  if command -v "$1" >/dev/null; then
    printf 'ok       %s\n' "$1"
  else
    printf 'MISSING  %s  ->  %s\n' "$1" "$2"
    missing=1
  fi
}
check jq "sudo pacman -S jq"
check rg "sudo pacman -S ripgrep"
check golangci-lint "sudo pacman -S golangci-lint (v2)"
check ast-grep "npm i -g @ast-grep/cli"
check typescript-language-server "npm i -g typescript-language-server typescript"
check gofumpt "make tools"
check goimports "make tools"
check gopls "make tools"
# Opcional: los LSP de Claude Code vienen de plugins; enabledPlugins versionado no los instala en un clon nuevo.
if command -v claude >/dev/null; then
  for plugin in gopls-lsp typescript-lsp; do
    claude plugin install "$plugin@claude-plugins-official" --scope project >/dev/null 2>&1 \
      && printf 'ok       plugin %s\n' "$plugin" \
      || printf 'HINT     no se pudo instalar el plugin; probá: claude plugin install %s@claude-plugins-official --scope project\n' "$plugin"
  done
else
  echo "HINT     'claude' no está en PATH: instalá los plugins LSP más tarde (claude plugin install gopls-lsp@claude-plugins-official --scope project; idem typescript-lsp)."
fi
echo "Make sure \$HOME/go/bin and the npm global bin dir (e.g. \$HOME/.local/bin) are in PATH."
((install_failed)) && echo "Some go install commands failed (see above)." >&2
exit $((missing || install_failed))
