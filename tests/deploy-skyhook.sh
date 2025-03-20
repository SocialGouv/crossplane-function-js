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

# Get the registry IP address
REGISTRY_IP=$(docker inspect -f '{{.NetworkSettings.Networks.kind.IPAddress}}' kind-registry)
echo "Registry IP address: ${REGISTRY_IP}"

# Configure containerd to use HTTP for the registry IP
REGISTRY_DIR="/etc/containerd/certs.d/${REGISTRY_IP}:5000"
for node in $(kind get nodes); do
  docker exec "${node}" mkdir -p "${REGISTRY_DIR}"
  cat <<EOF | docker exec -i "${node}" cp /dev/stdin "${REGISTRY_DIR}/hosts.toml"
[host."http://kind-registry:5000"]
  capabilities = ["pull", "resolve"]
  skip_verify = true
EOF
done

# Deploy the Skyhook Helm chart
echo "Deploying Skyhook Helm chart..."
helm upgrade --install skyhook ./charts/skyhook \
  --set function.package.repository=${REGISTRY_IP}:5000 \
  --set function.package.name=crossplane-skyhook \
  --set function.package.tag=test

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
