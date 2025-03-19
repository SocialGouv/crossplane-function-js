#!/usr/bin/env bash
set -eo errexit

# Build the Docker image
echo "Building Docker image..."
docker build --tag localhost:5001/crossplane-skyhook:test .
docker push localhost:5001/crossplane-skyhook:test

# Create a namespace for our server
echo "Creating namespace..."
kubectl create namespace skyhook-system --dry-run=client -o yaml | kubectl apply -f -

# Deploy the server
echo "Deploying skyhook-server..."
cat <<EOF | kubectl apply -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: skyhook-server
  namespace: skyhook-system
spec:
  replicas: 1
  selector:
    matchLabels:
      app: skyhook-server
  template:
    metadata:
      labels:
        app: skyhook-server
    spec:
      containers:
      - name: skyhook-server
        image: localhost:5001/crossplane-skyhook:test
        args: ["--grpc-addr=:50051"]
        ports:
        - containerPort: 50051
        resources:
          requests:
            cpu: 100m
            memory: 256Mi
          limits:
            cpu: 500m
            memory: 512Mi
---
apiVersion: v1
kind: Service
metadata:
  name: skyhook-server
  namespace: skyhook-system
spec:
  selector:
    app: skyhook-server
  ports:
  - port: 50051
    targetPort: 50051
EOF

# Wait for the server to be ready
echo "Waiting for skyhook-server to be ready..."
kubectl wait --for=condition=ready pod -l app=skyhook-server --namespace skyhook-system --timeout=300s

# Register the Function with Crossplane
echo "Registering Crossplane Function..."
cat <<EOF | kubectl apply -f -
apiVersion: pkg.crossplane.io/v1beta1
kind: Function
metadata:
  name: function-skyhook
spec:
  package: localhost:5001/crossplane-skyhook:test
---
apiVersion: pkg.crossplane.io/v1alpha1
kind: FunctionRuntime
metadata:
  name: skyhook-runtime
spec:
  runtimeConfigRef:
    name: skyhook-runtime-config
---
apiVersion: pkg.crossplane.io/v1alpha1
kind: RuntimeConfig
metadata:
  name: skyhook-runtime-config
spec:
  serviceConfig:
    service:
      name: skyhook-server
      namespace: skyhook-system
      port: 50051
EOF

echo "Skyhook server deployed successfully!"
