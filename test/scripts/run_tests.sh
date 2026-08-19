#!/bin/sh
# Run mock-based integration tests for ai-gateway-controller.
# Usage: sh test/scripts/run_tests.sh [feature_dir]
set -e

cd "$(dirname "$0")/../.."
ROOT=$(pwd)

echo "==> [1/2] go vet ./internal/... ./test/integration/..."
go vet ./internal/... ./test/integration/... || true

echo "==> [2/2] go test ./test/integration/..."
if [ -n "$1" ] && [ "$1" != "-f" ]; then
  go test -v -count=1 -timeout 300s ./test/integration/tests/$1/...
else
  go test -v -count=1 -timeout 600s ./test/integration/...
fi
