#!/bin/bash
# Initializes the Acme Electronics demo vendor: venv -> deps (incl. the
# weave SDK + shared-clients, needed by setup_weave.py) -> compile the
# core-client protos into shared-clients -> start the demo API.
#
# This is two things in one directory: a real HTTP API (api.py) standing
# in for "a business's own systems," and a one-time registration script
# (setup_weave.py) that uses the weave SDK to attach a subset of that
# API to a real running Weave core as tools + bot profiles. The API must
# be running (this script's default action) before setup_weave.py is run
# separately against it.
#
# Usage:
#   ./initialize.sh                # full setup + start the demo API
#   ./initialize.sh --setup-only   # setup only, don't start the API
#
# Config: DEMO_PORT (default 9100).
set -euo pipefail

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$DIR/../.." && pwd)"
PROTOS_DIR="$REPO_ROOT/protos"
SHARED_CLIENTS_DIR="$REPO_ROOT/packages/shared-clients"
SHARED_CLIENTS_GEN="$SHARED_CLIENTS_DIR/weave_shared_clients/gen"
SDK_DIR="$REPO_ROOT/packages/weave-sdk"
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

echo "==> Installing dependencies (demo API + weave SDK + shared-clients)"
"$PYTHON" -m pip install --upgrade pip --quiet
if command -v cygpath >/dev/null 2>&1; then
  SHARED_CLIENTS_WIN="$(cygpath -w "$SHARED_CLIENTS_DIR")"
  SDK_WIN="$(cygpath -w "$SDK_DIR")"
else
  SHARED_CLIENTS_WIN="$SHARED_CLIENTS_DIR"
  SDK_WIN="$SDK_DIR"
fi
"$PYTHON" -m pip install -e ".[dev]" --quiet
"$PYTHON" -m pip install -e "${SHARED_CLIENTS_WIN}[dev]" --quiet
"$PYTHON" -m pip install -e "${SDK_WIN}[dev]" --quiet
# Pinned to match every other service's grpcio version exactly (not
# just >=) — grpcio-tools' generated _grpc.py stubs embed a minimum
# grpcio version check, so two services regenerating shared-clients'
# gen/ with different grpcio-tools patch versions can leave the other
# unable to import it. Real bug hit while building this demo: regenerating
# with a newer grpcio-tools here broke orchestrator's already-running
# venv until shared-clients was regenerated back down.
"$PYTHON" -m pip install grpcio==1.83.0 grpcio-tools==1.83.0 --quiet

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
  echo "==> Setup complete (--setup-only, not starting the API)"
  exit 0
fi

echo "==> Starting Acme Electronics demo API on port ${DEMO_PORT:-9100}"
exec "$PYTHON" api.py
