#!/usr/bin/env bash
set -eo errexit

# Create a test namespace
echo "Creating test namespace..."
kubectl create namespace test-skyhook --dry-run=client -o yaml | kubectl apply -f -

# Apply CRDs and Compositions
echo "Applying CRDs and Compositions..."
kubectl apply -f test/fixtures/crd.yaml
kubectl apply -f test/fixtures/composition.yaml

# Wait for CRDs to be established
echo "Waiting for CRDs to be established..."
kubectl wait --for=condition=established crd/simpleconfigmaps.test.crossplane.io --timeout=60s

# Create a test SimpleConfigMap
echo "Creating test SimpleConfigMap..."
kubectl apply -f test/fixtures/sample.yaml

# Wait for the ConfigMap to be created
echo "Waiting for ConfigMap to be created..."
for i in {1..30}; do
  if kubectl get configmap generated-configmap -n test-skyhook &> /dev/null; then
    echo "ConfigMap created successfully!"
    break
  fi
  echo "Waiting for ConfigMap to be created... ($i/30)"
  sleep 2
done

# Verify the ConfigMap data
echo "Verifying ConfigMap data..."
configmap_data=$(kubectl get configmap generated-configmap -n test-skyhook -o jsonpath='{.data}')
echo "ConfigMap data: $configmap_data"

# Check if the data was transformed correctly (uppercase)
if [[ $configmap_data == *"NAME"* && $configmap_data == *"JOHN DOE"* ]]; then
  echo "Test PASSED: ConfigMap data was transformed correctly!"
  exit 0
else
  echo "Test FAILED: ConfigMap data was not transformed correctly!"
  exit 1
fi
