#!/bin/bash
# Initializes the weave SDK for local development/testing: venv -> deps
# -> compile the small slice of core-client protos it needs directly into
# its own weave/gen/ (self-contained — see weave/_core_client.py's
# docstring for why this package doesn't depend on packages/shared-clients
# the way weave's own services do). No "start" step — this is a library,
# not a service; import weave in your own code, or run its tests with
# `.venv/.../pytest tests/`.
#
# This same codegen step is what any *external* consumer of this SDK
# needs too (e.g. a sibling project depending on this directory as a
# path/git dependency pre-publish, or — post-publish — this becomes part
# of the package build instead of a manual step) — it isn't specific to
# developing weave-sdk itself.
#
# Usage: ./initialize.sh
set -euo pipefail

SDK_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SDK_DIR/.." && cd .. && pwd)"
PROTOS_DIR="$REPO_ROOT/protos"
SDK_GEN="$SDK_DIR/weave/gen"
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

echo "==> Installing weave SDK (editable)"
"$PYTHON" -m pip install --upgrade pip --quiet
if command -v cygpath >/dev/null 2>&1; then
  SDK_WIN="$(cygpath -w "$SDK_DIR")"
else
  SDK_WIN="$SDK_DIR"
fi
"$PYTHON" -m pip install -e "${SDK_WIN}[dev]" --quiet

echo "==> Compiling core client protos into weave/gen (bundled, not shared)"
mkdir -p "$SDK_GEN"
"$PYTHON" -m grpc_tools.protoc \
  "-I$PROTOS_DIR" \
  "--python_out=$SDK_GEN" \
  "--grpc_python_out=$SDK_GEN" \
  "--pyi_out=$SDK_GEN" \
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
find "$SDK_GEN" -type d -exec touch {}/__init__.py \;

echo "==> Setup complete"
