#!/bin/bash
# Initializes and runs mcp-gateway: venv -> Python deps -> compile the
# core-client protos it needs into packages/shared-clients -> start the
# ASGI server. One command from a fresh checkout to a running gateway.
#
# Usage:
#   ./initialize.sh                # full setup + start the server
#   ./initialize.sh --setup-only   # setup only, don't start the server
set -euo pipefail

GATEWAY_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$GATEWAY_DIR/.." && pwd)"
PROTOS_DIR="$REPO_ROOT/protos"
SHARED_CLIENTS_DIR="$REPO_ROOT/packages/shared-clients"
SHARED_CLIENTS_GEN="$SHARED_CLIENTS_DIR/weave_shared_clients/gen"
VENV_DIR="$GATEWAY_DIR/.venv"

cd "$GATEWAY_DIR"

if [ ! -d "$VENV_DIR" ]; then
  echo "==> Creating virtualenv at $VENV_DIR"
  python -m venv "$VENV_DIR"
fi

if [ -f "$VENV_DIR/Scripts/python.exe" ]; then
  PYTHON="$VENV_DIR/Scripts/python.exe"
else
  PYTHON="$VENV_DIR/bin/python"
fi

echo "==> Installing shared-clients + mcp-gateway (editable)"
"$PYTHON" -m pip install --upgrade pip --quiet
if command -v cygpath >/dev/null 2>&1; then
  SHARED_CLIENTS_WIN="$(cygpath -w "$SHARED_CLIENTS_DIR")"
  GATEWAY_WIN="$(cygpath -w "$GATEWAY_DIR")"
else
  SHARED_CLIENTS_WIN="$SHARED_CLIENTS_DIR"
  GATEWAY_WIN="$GATEWAY_DIR"
fi
"$PYTHON" -m pip install -e "${SHARED_CLIENTS_WIN}[dev]" --quiet
"$PYTHON" -m pip install -e "${GATEWAY_WIN}[dev]" --quiet

echo "==> Compiling core client protos into shared-clients"
mkdir -p "$SHARED_CLIENTS_GEN"
"$PYTHON" -m grpc_tools.protoc \
  "-I$PROTOS_DIR" \
  "--python_out=$SHARED_CLIENTS_GEN" \
  "--grpc_python_out=$SHARED_CLIENTS_GEN" \
  "--pyi_out=$SHARED_CLIENTS_GEN" \
  "$PROTOS_DIR/database/v1/common.proto" \
  "$PROTOS_DIR/database/v1/tenant.proto" \
  "$PROTOS_DIR/database/v1/connector.proto" \
  "$PROTOS_DIR/database/v1/credential.proto" \
  "$PROTOS_DIR/database/v1/auth.proto" \
  "$PROTOS_DIR/database/v1/bot_profile.proto" \
  "$PROTOS_DIR/database/v1/http_tool.proto" \
  "$PROTOS_DIR/core/data_access/v1/tenant.proto" \
  "$PROTOS_DIR/core/data_access/v1/connector.proto" \
  "$PROTOS_DIR/core/data_access/v1/auth.proto" \
  "$PROTOS_DIR/core/data_access/v1/bot_profile.proto" \
  "$PROTOS_DIR/core/data_access/v1/http_tool.proto"
find "$SHARED_CLIENTS_GEN" -type d -exec touch {}/__init__.py \;

if [ "${1:-}" = "--setup-only" ]; then
  echo "==> Setup complete (--setup-only, not starting the server)"
  exit 0
fi

echo "==> Starting mcp-gateway"
exec "$PYTHON" server.py
