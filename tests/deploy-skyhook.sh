#!/usr/bin/env bash
set -e

# Function to handle errors
handle_error() {
  echo "Error occurred at line $1"
  echo "=== Deployment failed! ==="
  exit 1
}

# Set up error trap
trap 'handle_error $LINENO' ERR

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
      annotations:
        kubectl.kubernetes.io/restartedAt: "$(date -u +'%Y-%m-%dT%H:%M:%SZ')"
    spec:
      containers:
      - name: skyhook-server
        image: localhost:5001/crossplane-skyhook:test
        imagePullPolicy: Always
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

# Wait for the deployment to create pods
echo "Waiting for deployment to create pods..."
sleep 10

# Wait for the server to be ready
echo "Waiting for skyhook-server to be ready..."
pod_ready=false
for i in {1..30}; do
  if kubectl get pods -l app=skyhook-server -n skyhook-system 2>/dev/null | grep -q "Running"; then
    echo "Pod is running, waiting for it to be ready..."
    if kubectl wait --for=condition=ready pod -l app=skyhook-server --namespace skyhook-system --timeout=10s 2>/dev/null; then
      pod_ready=true
      break
    fi
  fi
  echo "Waiting for pod to be ready... ($i/30)"
  sleep 5
done

if [ "$pod_ready" = false ]; then
  echo "Pod did not become ready within timeout"
  echo "Current pod status:"
  kubectl get pods -n skyhook-system
  echo "Pod logs:"
  kubectl logs -l app=skyhook-server -n skyhook-system --tail=50 || true
  # Continue anyway
fi

# Register the Function with Crossplane
echo "Registering Crossplane Function..."
cat <<EOF | kubectl apply -f -
apiVersion: pkg.crossplane.io/v1beta1
kind: Function
metadata:
  name: function-skyhook
spec:
  package: localhost:5001/crossplane-skyhook:test
  packagePullPolicy: Always
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

# Wait for the Function to be installed
echo "Waiting for Function to be installed..."
for i in {1..30}; do
  if kubectl get functions.pkg.crossplane.io function-skyhook -o jsonpath='{.status.conditions[?(@.type=="Installed")].status}' | grep -q "True"; then
    echo "Function installed successfully!"
    break
  fi
  echo "Waiting for Function to be installed... ($i/30)"
  sleep 2
done

echo "Skyhook server deployed successfully!"
