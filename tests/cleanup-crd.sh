#!/usr/bin/env bash
set -e

echo "Deleting existing CustomResourceDefinition..."
kubectl delete crd simpleconfigmaps.test.crossplane.io || {
  echo "No existing CRD found or error deleting it. Continuing..."
}

echo "Cleanup completed."
