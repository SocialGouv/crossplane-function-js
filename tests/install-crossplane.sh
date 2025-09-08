#!/usr/bin/env bash
set -e

# Function to handle errors
handle_error() {
  echo "Error occurred at line $1"
  echo "=== Installation failed! ==="
  exit 1
}

# Set up error trap
trap 'handle_error $LINENO' ERR

# Add Crossplane Helm repository
echo "Adding Crossplane Helm repository..."
helm repo add crossplane-stable https://charts.crossplane.io/stable
helm repo update

# Install Crossplane v2.x with a shorter timeout
echo "Installing Crossplane v2.x..."
helm upgrade --install crossplane \
  --namespace crossplane-system \
  --create-namespace \
  --timeout 5m \
  --version "^2.0.0" \
  --set "args={--enable-deployment-runtime-configs}" \
  crossplane-stable/crossplane

# Wait for Crossplane to be ready
echo "Waiting for Crossplane pods to be ready..."
kubectl wait --for=condition=ready pod -l app=crossplane --namespace crossplane-system --timeout=300s || echo "Timed out waiting for Crossplane pods"

echo "Crossplane installed successfully!"
