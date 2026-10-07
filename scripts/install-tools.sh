#!/usr/bin/env bash
# Local developer/agent tooling. Go tools are installed with `go install`; the rest are only
# checked, with a hint on how to install them (no sudo, no global package manager side effects).
set -uo pipefail
export PATH="$PATH:$HOME/go/bin:$HOME/.local/bin"

go install mvdan.cc/gofumpt@latest
go install golang.org/x/tools/cmd/goimports@latest
go install golang.org/x/tools/gopls@latest

missing=0
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
check gofumpt "go install mvdan.cc/gofumpt@latest"
check goimports "go install golang.org/x/tools/cmd/goimports@latest"
check gopls "go install golang.org/x/tools/gopls@latest"
echo "Make sure \$HOME/go/bin and the npm global bin dir (e.g. \$HOME/.local/bin) are in PATH."
exit "$missing"
