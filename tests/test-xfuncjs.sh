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

# Create extra resource fixtures for extraResourceRequirements e2e validation
echo "Creating extra-resource fixtures (ConfigMaps + secondary namespace)..."
kubectl create namespace test-xfuncjs-2 --dry-run=client -o yaml | kubectl apply -f -

# 1) Namespaced-only (should match exactly 1 in test-xfuncjs)
kubectl apply -f - <<'YAML'
apiVersion: v1
kind: ConfigMap
metadata:
  name: extra-ns-only
  namespace: test-xfuncjs
  labels:
    crossplane-js.dev/e2e: extra
    crossplane-js.dev/scope: ns-only
data:
  hello: world
YAML

# 2) All-namespaces (should match exactly 2: one per namespace)
kubectl apply -f - <<'YAML'
apiVersion: v1
kind: ConfigMap
metadata:
  name: extra-all-ns-1
  namespace: test-xfuncjs
  labels:
    crossplane-js.dev/e2e: extra
    crossplane-js.dev/scope: all-ns
data:
  hello: world
YAML

kubectl apply -f - <<'YAML'
apiVersion: v1
kind: ConfigMap
metadata:
  name: extra-all-ns-2
  namespace: test-xfuncjs-2
  labels:
    crossplane-js.dev/e2e: extra
    crossplane-js.dev/scope: all-ns
data:
  hello: world
YAML


# Apply the first part of provider in cluster (Provider, DeploymentRuntimeConfig, ClusterRoleBinding)
echo "Applying Provider Kubernetes..."
kubectl apply -f tests/fixtures/crossplane-basics/provider-in-cluster.yaml
kubectl apply -f tests/fixtures/crossplane-basics/functions.yaml

# Ensure Crossplane can fetch cluster-scoped resources for Composition extraResourceRequirements
echo "Applying Crossplane extraResources E2E RBAC (namespaces get/list/watch)..."
kubectl apply -f tests/fixtures/crossplane-basics/crossplane-extraresources-rbac.yaml

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
yarn --cwd tests/fixtures/domain-sdk gen-models # just here to avoid forgetting to regen models
yarn --cwd tests/fixtures/domain-sdk gen-manifests

# Apply CRDs and Compositions
echo "Applying XRD and Compositions..."
kubectl apply --server-side=true -f tests/fixtures/domain-sdk/manifests/simpleconfigmaps.test.crossplane.io

# Wait for XRD to be established
echo "Waiting for XRD to be established..."
kubectl wait --for=condition=established xrd/simpleconfigmaps.test.crossplane.io --timeout=60s || {
  echo "XRD not established within timeout"
  echo "Current XRD status:"
  kubectl get xrd/simpleconfigmaps.test.crossplane.io -o yaml
  exit 1
}

# Create a test SimpleConfigMap
echo "(Re)Creating test SimpleConfigMap..."
kubectl delete -f tests/fixtures/domain-sdk/sample.yaml || true
kubectl apply -f tests/fixtures/domain-sdk/sample.yaml

# Wait for the ConfigMap to be created
echo "Waiting for ConfigMap to be created..."
configmap_created=false
for i in {1..180}; do
  if kubectl get configmap generated-configmap -n test-xfuncjs &> /dev/null; then
    # Also ensure the extraResources-driven annotations are present and match
    # expected counts. The function may run once before Crossplane injects
    # extraResources.
    extra_ns_cm_count=$(kubectl get configmap generated-configmap -n test-xfuncjs -o jsonpath='{.metadata.annotations.crossplane-js\.dev/e2e-extra-ns-cm-count}' 2>/dev/null || true)
    extra_allns_cm_count=$(kubectl get configmap generated-configmap -n test-xfuncjs -o jsonpath='{.metadata.annotations.crossplane-js\.dev/e2e-extra-allns-cm-count}' 2>/dev/null || true)
    extra_namespace_count=$(kubectl get configmap generated-configmap -n test-xfuncjs -o jsonpath='{.metadata.annotations.crossplane-js\.dev/e2e-extra-namespace-count}' 2>/dev/null || true)
    if [[ "$extra_ns_cm_count" == "1" && "$extra_allns_cm_count" == "2" && "$extra_namespace_count" == "1" ]]; then
      echo "ConfigMap created successfully (and extraResources annotations converged)!"
      configmap_created=true
      break
    fi
    echo "ConfigMap exists but extraResources annotations not ready yet: ns=$extra_ns_cm_count allns=$extra_allns_cm_count namespace=$extra_namespace_count"
  fi
  echo "Waiting for ConfigMap to be created... ($i/180)"
  
  # Check the status of the SimpleConfigMap
  if [ $((i % 5)) -eq 0 ]; then
    echo "SimpleConfigMap status:"
    kubectl get simpleconfigmaps.test.crossplane.io -o yaml || true
    echo "Crossplane Function status:"
    kubectl get functions.pkg.crossplane.io || true
    echo "Function pods status:"
    kubectl get pods -n crossplane-system -l app.kubernetes.io/name=function-xfuncjs || true
  fi
  
  sleep 2
done

if [ "$configmap_created" = false ]; then
  echo "ConfigMap was not created within timeout"
  echo "Final SimpleConfigMap status:"
  kubectl get simpleconfigmaps.test.crossplane.io -o yaml || true
  echo "Final Crossplane Function status:"
  kubectl get functions.pkg.crossplane.io || true
  echo "Final Function pods status:"
  kubectl get pods -n crossplane-system -l app.kubernetes.io/name=function-xfuncjs || true
  exit 1
fi

# Verify the ConfigMap data
echo "Verifying ConfigMap data..."
configmap_data=$(kubectl get configmap generated-configmap -n test-xfuncjs -o jsonpath='{.data}')
echo "ConfigMap data: $configmap_data"

# Verify FieldRef resolution (label should match XR name)
echo "Verifying FieldRef-derived label on ConfigMap..."
xr_name_label=$(kubectl get configmap generated-configmap -n test-xfuncjs -o jsonpath='{.metadata.labels.crossplane-js\.dev/xr-name}')
echo "ConfigMap label crossplane-js.dev/xr-name: $xr_name_label"

# Verify extraResourceRequirements injection (counts published as annotations)
echo "Verifying extraResources injection (via annotations)..."
extra_ns_cm_count=$(kubectl get configmap generated-configmap -n test-xfuncjs -o jsonpath='{.metadata.annotations.crossplane-js\.dev/e2e-extra-ns-cm-count}')
extra_allns_cm_count=$(kubectl get configmap generated-configmap -n test-xfuncjs -o jsonpath='{.metadata.annotations.crossplane-js\.dev/e2e-extra-allns-cm-count}')
extra_namespace_count=$(kubectl get configmap generated-configmap -n test-xfuncjs -o jsonpath='{.metadata.annotations.crossplane-js\.dev/e2e-extra-namespace-count}')

extra_ns_cm_names=$(kubectl get configmap generated-configmap -n test-xfuncjs -o jsonpath='{.metadata.annotations.crossplane-js\.dev/e2e-extra-ns-cm-names}')
extra_allns_cm_names=$(kubectl get configmap generated-configmap -n test-xfuncjs -o jsonpath='{.metadata.annotations.crossplane-js\.dev/e2e-extra-allns-cm-names}')
extra_namespace_names=$(kubectl get configmap generated-configmap -n test-xfuncjs -o jsonpath='{.metadata.annotations.crossplane-js\.dev/e2e-extra-namespace-names}')

echo "extra ns ConfigMap count: $extra_ns_cm_count (names: $extra_ns_cm_names)"
echo "extra all-ns ConfigMap count: $extra_allns_cm_count (names: $extra_allns_cm_names)"
echo "extra Namespace count: $extra_namespace_count (names: $extra_namespace_names)"

# Check if the data was transformed correctly (uppercase), FieldRef resolved,
# and extraResources were retrieved.
if [[ $configmap_data == *"NAME"* && $configmap_data == *"JOHN DOE"* && $xr_name_label == "sample-configmap" && $extra_ns_cm_count == "1" && $extra_allns_cm_count == "2" && $extra_namespace_count == "1" ]]; then
  echo "Test PASSED: ConfigMap data was transformed correctly!"
  exit 0
else
  echo "Test FAILED: ConfigMap data was not transformed correctly!"
  exit 1
fi
