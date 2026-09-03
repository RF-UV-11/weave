#!/bin/bash
# Initializes and runs the web app: npm install -> compile protos into
# src/gen (via buf, docs/architecture/ARCHITECTURE.md §4 — the browser
# never speaks raw gRPC, so these are grpc-web-compatible TS clients, not
# Go/Python stubs) -> generate Next.js route types -> start the dev server.
#
# Prerequisites this script does NOT start for you: core, orchestrator,
# and an Envoy grpc-web proxy in front of them (infra/envoy/envoy.yaml
# for the container-shaped version matching podman-compose.yml, or
# infra/envoy/envoy.local.yaml — see that file's comment — if you're
# running Envoy as a native binary because container-to-host networking
# doesn't work reliably in your environment, e.g. Windows/WSL2).
#
# Usage:
#   ./initialize.sh                # full setup + start the dev server
#   ./initialize.sh --setup-only   # setup only, don't start the dev server
#
# Requires: node/npm on PATH, buf on PATH (used elsewhere in this repo
# for Go codegen too — see protos/buf.gen.yaml).
set -euo pipefail

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROTOS_DIR="$(cd "$DIR/../protos" && pwd)"

cd "$DIR"

echo "==> Installing dependencies"
npm install --no-audit --no-fund

echo "==> Compiling protos into src/gen"
buf generate "$PROTOS_DIR"

echo "==> Generating Next.js route types"
npx next typegen

if [ "${1:-}" = "--setup-only" ]; then
  echo "==> Setup complete (--setup-only, not starting the dev server)"
  exit 0
fi

echo "==> Starting the web dev server on http://localhost:3000"
exec npm run dev
