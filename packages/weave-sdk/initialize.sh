#!/bin/bash
# Initializes the weave SDK for local development/testing: venv -> deps
# -> compile the core-client protos it needs into packages/shared-clients.
# No "start" step — this is a library, not a service; import weave in
# your own code, or run its tests with `.venv/.../pytest tests/`.
#
# Usage: ./initialize.sh
set -euo pipefail

SDK_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SDK_DIR/.." && cd .. && pwd)"
PROTOS_DIR="$REPO_ROOT/protos"
SHARED_CLIENTS_DIR="$REPO_ROOT/packages/shared-clients"
SHARED_CLIENTS_GEN="$SHARED_CLIENTS_DIR/weave_shared_clients/gen"
VENV_DIR="$SDK_DIR/.venv"

cd "$SDK_DIR"

if [ ! -d "$VENV_DIR" ]; then
  echo "==> Creating virtualenv at $VENV_DIR"
  python -m venv "$VENV_DIR"
fi

if [ -f "$VENV_DIR/Scripts/python.exe" ]; then
  PYTHON="$VENV_DIR/Scripts/python.exe"
else
  PYTHON="$VENV_DIR/bin/python"
fi

echo "==> Installing shared-clients + weave SDK (editable)"
"$PYTHON" -m pip install --upgrade pip --quiet
if command -v cygpath >/dev/null 2>&1; then
  SHARED_CLIENTS_WIN="$(cygpath -w "$SHARED_CLIENTS_DIR")"
  SDK_WIN="$(cygpath -w "$SDK_DIR")"
else
  SHARED_CLIENTS_WIN="$SHARED_CLIENTS_DIR"
  SDK_WIN="$SDK_DIR"
fi
"$PYTHON" -m pip install -e "${SHARED_CLIENTS_WIN}[dev]" --quiet
"$PYTHON" -m pip install -e "${SDK_WIN}[dev]" --quiet

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

echo "==> Setup complete"
