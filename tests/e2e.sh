#!/usr/bin/env bash
set -eo errexit

SCRIPT_DIR=$(dirname "$0")

echo "=== Setting up Kind cluster with local registry ==="
"$SCRIPT_DIR/kind-with-registry.sh"

echo "=== Installing Crossplane ==="
"$SCRIPT_DIR/install-crossplane.sh"

echo "=== Deploying Skyhook server ==="
"$SCRIPT_DIR/deploy-skyhook.sh"

echo "=== Running tests ==="
"$SCRIPT_DIR/test-skyhook.sh" "$@"

echo "=== E2E tests completed successfully! ==="
