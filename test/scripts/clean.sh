#!/bin/sh
# Clean mock-test runtime artifacts (keep the source tree intact).
cd "$(dirname "$0")/../.."

echo "==> removing test/integration/artifacts"
rm -rf test/integration/artifacts
mkdir -p test/integration/artifacts
echo "==> done"
