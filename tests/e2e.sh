#!/usr/bin/env bash
set -eo errexit

SCRIPT_DIR=$(dirname "$0")

# Check if Kind cluster already exists
if ! kind get clusters | grep -q "^kind$"; then
  echo "=== Setting up Kind cluster with local registry ==="
  "$SCRIPT_DIR/kind-with-registry.sh"

  echo "=== Installing Crossplane ==="
  "$SCRIPT_DIR/install-crossplane.sh"
else
  echo "=== Kind cluster already exists, skipping setup ==="
fi

echo "=== Deploying Skyhook server ==="
"$SCRIPT_DIR/deploy-skyhook.sh"

echo "=== Running tests ==="
"$SCRIPT_DIR/test-skyhook.sh" "$@"

echo "=== E2E tests completed successfully! ==="
