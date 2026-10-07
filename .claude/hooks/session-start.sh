#!/usr/bin/env bash
# SessionStart (startup): activa los hooks de git versionados y avisa (una línea) si faltan herramientas.
# No imprime nada si todo está en orden y nunca falla la sesión.
cd "${CLAUDE_PROJECT_DIR:-.}" 2>/dev/null || exit 0
git config core.hooksPath .githooks >/dev/null 2>&1
bash scripts/install-tools.sh --check 2>/dev/null
exit 0
