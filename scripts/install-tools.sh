#!/usr/bin/env bash
# Local developer/agent tooling. Go tools are installed with `go install`; the rest are only
# checked, with a hint on how to install them (no sudo, no global package manager side effects).
set -uo pipefail
export PATH="$PATH:$HOME/go/bin:$HOME/.local/bin"

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
echo "Make sure \$HOME/go/bin and the npm global bin dir (e.g. \$HOME/.local/bin) are in PATH."
((install_failed)) && echo "Some go install commands failed (see above)." >&2
exit $((missing || install_failed))
