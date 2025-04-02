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
kubectl create namespace test-xfuncjs --dry-run=client -o yaml | kubectl apply -f -


# Apply the first part of provider in cluster (Provider, DeploymentRuntimeConfig, ClusterRoleBinding)
echo "Applying Provider Kubernetes..."
kubectl apply -f tests/fixtures/crossplane-basics/provider-in-cluster.yaml
kubectl apply -f tests/fixtures/crossplane-basics/functions.yaml

# Wait for the Provider to be installed and its CRDs to be registered
echo "Waiting for Provider Kubernetes to be installed..."
kubectl wait --for=condition=healthy provider.pkg.crossplane.io/provider-kubernetes --timeout=120s || {
  echo "Provider Kubernetes not installed within timeout"
  echo "Current Provider status:"
  kubectl get provider.pkg.crossplane.io/provider-kubernetes -o yaml
  exit 1
}

# Wait for the ProviderConfig CRD to be established
echo "Waiting for ProviderConfig CRD to be established..."
for i in {1..30}; do
  if kubectl get crd providerconfigs.kubernetes.crossplane.io &> /dev/null; then
    echo "ProviderConfig CRD is established!"
    break
  fi
  echo "Waiting for ProviderConfig CRD to be established... ($i/30)"
  sleep 2
done

# Apply the second part of provider in cluster (ProviderConfig)
echo "Applying ProviderConfig..."
kubectl apply -f tests/fixtures/crossplane-basics/provider-config.yaml

# Create composition outputs
echo "Preparing composition with function code..."
yarn --cwd tests/fixtures/domain-sdk install
yarn --cwd tests/fixtures/domain-sdk compo

# Apply CRDs and Compositions
echo "Applying XRD and Compositions..."
kubectl apply -f tests/fixtures/domain-sdk/functions/xsimpleconfigmaps/xrd.yaml
kubectl apply -f tests/fixtures/domain-sdk/manifests/xsimpleconfigmaps.compo.yaml

# Wait for XRD to be established
echo "Waiting for XRD to be established..."
kubectl wait --for=condition=established xrd/xsimpleconfigmaps.test.crossplane.io --timeout=60s || {
  echo "XRD not established within timeout"
  echo "Current XRD status:"
  kubectl get xrd/xsimpleconfigmaps.test.crossplane.io -o yaml
  exit 1
}

# Wait for the XSimpleConfigmap CRD to be established
echo "Waiting for XSimpleConfigmap CRD to be established..."
for i in {1..30}; do
  if kubectl get crd xsimpleconfigmaps.test.crossplane.io &> /dev/null; then
    echo "XSimpleConfigmap CRD is established!"
    break
  fi
  echo "Waiting for XSimpleConfigmap CRD to be established... ($i/30)"
  sleep 2
done

# Generate models from CRD
kubectl get crd xsimpleconfigmaps.test.crossplane.io -o yaml > tests/fixtures/domain-sdk/manifests/xsimpleconfigmaps.crd.yaml
yarn crd-generate --input tests/fixtures/domain-sdk/manifests/xsimpleconfigmaps.crd.yaml --output tests/fixtures/domain-sdk/models


# Create a test XSimpleConfigMap
echo "(Re)Creating test XSimpleConfigMap..."
kubectl delete -f tests/fixtures/domain-sdk/sample.yaml || true
kubectl apply -f tests/fixtures/domain-sdk/sample.yaml

# Wait for the ConfigMap to be created
echo "Waiting for ConfigMap to be created..."
configmap_created=false
for i in {1..30}; do
  if kubectl get configmap generated-configmap -n test-xfuncjs &> /dev/null; then
    echo "ConfigMap created successfully!"
    configmap_created=true
    break
  fi
  echo "Waiting for ConfigMap to be created... ($i/30)"
  
  # Check the status of the XSimpleConfigMap
  if [ $((i % 5)) -eq 0 ]; then
    echo "XSimpleConfigMap status:"
    kubectl get xsimpleconfigmaps.test.crossplane.io -o yaml || true
    echo "Crossplane Function status:"
    kubectl get functions.pkg.crossplane.io || true
    echo "FunctionRuntime status:"
    kubectl get functionruntimes.pkg.crossplane.io || true
  fi
  
  sleep 2
done

if [ "$configmap_created" = false ]; then
  echo "ConfigMap was not created within timeout"
  echo "Final XSimpleConfigMap status:"
  kubectl get xsimpleconfigmaps.test.crossplane.io -o yaml || true
  echo "Final Crossplane Function status:"
  kubectl get functions.pkg.crossplane.io || true
  echo "Final FunctionRuntime status:"
  kubectl get functionruntimes.pkg.crossplane.io || true
  exit 1
fi

# Verify the ConfigMap data
echo "Verifying ConfigMap data..."
configmap_data=$(kubectl get configmap generated-configmap -n test-xfuncjs -o jsonpath='{.data}')
echo "ConfigMap data: $configmap_data"

# Check if the data was transformed correctly (uppercase)
if [[ $configmap_data == *"NAME"* && $configmap_data == *"JOHN DOE"* ]]; then
  echo "Test PASSED: ConfigMap data was transformed correctly!"
  exit 0
else
  echo "Test FAILED: ConfigMap data was not transformed correctly!"
  exit 1
fi
