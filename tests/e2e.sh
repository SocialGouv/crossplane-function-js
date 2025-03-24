#!/usr/bin/env bash
set -e

SCRIPT_DIR=$(dirname "$0")

# Function to handle errors
handle_error() {
  echo "Error occurred at line $1"
  echo "=== E2E tests failed! ==="
  exit 1
}

# Set up error trap
trap 'handle_error $LINENO' ERR

# Check if Kind cluster already exists
if ! kind get clusters | grep -q "^kind$"; then
  echo "=== Setting up Kind cluster with local registry ==="
  "$SCRIPT_DIR/kind-with-registry.sh"

  echo "=== Installing Crossplane ==="
  "$SCRIPT_DIR/install-crossplane.sh"
else
  echo "=== Kind cluster already exists, skipping setup ==="
  
  # Check if Crossplane is installed
  if ! kubectl get namespace crossplane-system &>/dev/null; then
    echo "=== Crossplane not installed, installing now ==="
    "$SCRIPT_DIR/install-crossplane.sh"
  fi
fi

echo "=== Deploying Skyhook server ==="
"$SCRIPT_DIR/deploy-skyhook.sh"

echo "=== Running tests ==="
"$SCRIPT_DIR/test-skyhook.sh" "$@"

echo "=== E2E tests completed successfully! ==="
