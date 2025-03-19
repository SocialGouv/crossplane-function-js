#!/usr/bin/env bash
set -eo errexit

# Add Crossplane Helm repository
helm repo add crossplane-stable https://charts.crossplane.io/stable
helm repo update

# Install Crossplane
helm upgrade --install crossplane \
  --namespace crossplane-system \
  --create-namespace \
  --wait \
  crossplane-stable/crossplane

# Wait for Crossplane to be ready
echo "Waiting for Crossplane pods to be ready..."
kubectl wait --for=condition=ready pod -l app=crossplane --namespace crossplane-system --timeout=300s

echo "Crossplane installed successfully!"
