#!/usr/bin/env bash
set -e

# Function to handle errors
handle_error() {
  echo "Error occurred at line $1"
  echo "=== Test failed! ==="
  exit 1
}

# Set up error trap
trap 'handle_error $LINENO' ERR

# Create a test namespace
echo "Creating test namespace..."
kubectl create namespace test-skyhook --dry-run=client -o yaml | kubectl apply -f -

# Clean up existing CRD if it exists
echo "Cleaning up existing CRD..."
./tests/cleanup-crd.sh

# Apply CRDs and Compositions
echo "Applying CRDs and Compositions..."
kubectl apply -f tests/fixtures/crd.yaml
kubectl apply -f tests/fixtures/composition.yaml

# Wait for XRD to be established
echo "Waiting for XRD to be established..."
kubectl wait --for=condition=established xrd/simpleconfigmaps.test.crossplane.io --timeout=60s || {
  echo "XRD not established within timeout"
  echo "Current XRD status:"
  kubectl get xrd/simpleconfigmaps.test.crossplane.io -o yaml
  exit 1
}

# Create a test SimpleConfigMap
echo "Creating test SimpleConfigMap..."
kubectl apply -f tests/fixtures/sample.yaml

# Wait for the ConfigMap to be created
echo "Waiting for ConfigMap to be created..."
configmap_created=false
for i in {1..30}; do
  if kubectl get configmap generated-configmap -n test-skyhook &> /dev/null; then
    echo "ConfigMap created successfully!"
    configmap_created=true
    break
  fi
  echo "Waiting for ConfigMap to be created... ($i/30)"
  
  # Check the status of the SimpleConfigMap
  if [ $((i % 5)) -eq 0 ]; then
    echo "SimpleConfigMap status:"
    kubectl get simpleconfigmaps.test.crossplane.io -o yaml || true
    echo "Crossplane Function status:"
    kubectl get functions.pkg.crossplane.io || true
    echo "FunctionRuntime status:"
    kubectl get functionruntimes.pkg.crossplane.io || true
  fi
  
  sleep 2
done

if [ "$configmap_created" = false ]; then
  echo "ConfigMap was not created within timeout"
  echo "Final SimpleConfigMap status:"
  kubectl get simpleconfigmaps.test.crossplane.io -o yaml || true
  echo "Final Crossplane Function status:"
  kubectl get functions.pkg.crossplane.io || true
  echo "Final FunctionRuntime status:"
  kubectl get functionruntimes.pkg.crossplane.io || true
  exit 1
fi

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
