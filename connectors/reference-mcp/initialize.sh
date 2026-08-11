#!/bin/bash
# Initializes and runs the reference MCP server: venv -> deps -> start.
# No protos to compile here — MCP servers speak MCP/JSON-RPC, not gRPC.
#
# Usage:
#   ./initialize.sh                # full setup + start the server
#   ./initialize.sh --setup-only   # setup only, don't start the server
#
# Config: MCP_PORT (default 8765).
set -euo pipefail

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
VENV_DIR="$DIR/.venv"

cd "$DIR"

if [ ! -d "$VENV_DIR" ]; then
  echo "==> Creating virtualenv at $VENV_DIR"
  python -m venv "$VENV_DIR"
fi

if [ -f "$VENV_DIR/Scripts/python.exe" ]; then
  PYTHON="$VENV_DIR/Scripts/python.exe"
else
  PYTHON="$VENV_DIR/bin/python"
fi

echo "==> Installing dependencies"
"$PYTHON" -m pip install --upgrade pip --quiet
"$PYTHON" -m pip install -e . --quiet

if [ "${1:-}" = "--setup-only" ]; then
  echo "==> Setup complete (--setup-only, not starting the server)"
  exit 0
fi

echo "==> Starting reference-mcp on port ${MCP_PORT:-8765}"
exec "$PYTHON" server.py
