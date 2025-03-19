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
# helm repo update

# Install Crossplane with a shorter timeout
echo "Installing Crossplane..."
helm upgrade --install crossplane \
  --namespace crossplane-system \
  --create-namespace \
  --timeout 5m \
  --set "args={--enable-deployment-runtime-configs}" \
  crossplane-stable/crossplane

# Wait for Crossplane to be ready
echo "Waiting for Crossplane pods to be ready..."
kubectl wait --for=condition=ready pod -l app=crossplane --namespace crossplane-system --timeout=300s || echo "Timed out waiting for Crossplane pods"

# Install Function Runtime CRDs
echo "Installing Function Runtime CRDs..."
kubectl apply -f - <<EOF
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: functionruntimes.pkg.crossplane.io
spec:
  group: pkg.crossplane.io
  names:
    kind: FunctionRuntime
    listKind: FunctionRuntimeList
    plural: functionruntimes
    singular: functionruntime
  scope: Cluster
  versions:
  - name: v1alpha1
    served: true
    storage: true
    schema:
      openAPIV3Schema:
        type: object
        properties:
          spec:
            type: object
            properties:
              runtimeConfigRef:
                type: object
                properties:
                  name:
                    type: string
                required:
                - name
---
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: runtimeconfigs.pkg.crossplane.io
spec:
  group: pkg.crossplane.io
  names:
    kind: RuntimeConfig
    listKind: RuntimeConfigList
    plural: runtimeconfigs
    singular: runtimeconfig
  scope: Cluster
  versions:
  - name: v1alpha1
    served: true
    storage: true
    schema:
      openAPIV3Schema:
        type: object
        properties:
          spec:
            type: object
            properties:
              serviceConfig:
                type: object
                properties:
                  service:
                    type: object
                    properties:
                      name:
                        type: string
                      namespace:
                        type: string
                      port:
                        type: integer
                    required:
                    - name
                    - namespace
                    - port
EOF

echo "Crossplane installed successfully!"
