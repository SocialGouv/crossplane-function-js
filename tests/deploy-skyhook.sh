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

# Generate a timestamp tag
TIMESTAMP=$(date +"%Y%m%d%H%M%S")
TAG_TEST="test"
TAG_TIMESTAMP="${TIMESTAMP}"

# Build the Docker image with both tags
echo "Building Docker image..."
docker build --tag localhost:5001/crossplane-skyhook:${TAG_TEST} --tag localhost:5001/crossplane-skyhook:${TAG_TIMESTAMP} .
docker push localhost:5001/crossplane-skyhook:${TAG_TEST}
docker push localhost:5001/crossplane-skyhook:${TAG_TIMESTAMP}

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
  --set function.package.tag=${TAG_TIMESTAMP}

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

# Wait for the Function Revision to be ready with the latest tag
echo "Waiting for Function Revision with tag ${TAG_TIMESTAMP} to be ready..."
for i in {1..30}; do
  if kubectl get functionrevisions.pkg.crossplane.io -o jsonpath='{.items[*].spec.image}' | grep -q "${TAG_TIMESTAMP}"; then
    echo "Function Revision with tag ${TAG_TIMESTAMP} is ready!"
    break
  fi
  echo "Waiting for Function Revision to be ready... ($i/30)"
  sleep 2
  
  # If we've waited too long, show the current state
  if [ $i -eq 30 ]; then
    echo "Current Function Revisions:"
    kubectl get functionrevisions.pkg.crossplane.io -o wide
  fi
done

# Check for the existence of the pod with the latest tag
echo "Checking for pod with the latest tag..."
for i in {1..30}; do
  POD_NAME=$(kubectl get pods -n crossplane-system -l pkg.crossplane.io/function=function-skyhook -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
  
  if [ -n "$POD_NAME" ]; then
    # Check if the pod is running and using the correct image
    POD_IMAGE=$(kubectl get pod $POD_NAME -n crossplane-system -o jsonpath='{.spec.containers[0].image}' 2>/dev/null)
    POD_STATUS=$(kubectl get pod $POD_NAME -n crossplane-system -o jsonpath='{.status.phase}' 2>/dev/null)
    
    if [[ "$POD_IMAGE" == *"${TAG_TIMESTAMP}"* && "$POD_STATUS" == "Running" ]]; then
      echo "Pod $POD_NAME is running with the correct image: $POD_IMAGE"
      break
    else
      echo "Pod $POD_NAME found but either wrong image or not running yet:"
      echo "  - Image: $POD_IMAGE"
      echo "  - Status: $POD_STATUS"
    fi
  else
    echo "No pod found for function-skyhook yet"
  fi
  
  echo "Waiting for pod with tag ${TAG_TIMESTAMP} to be ready... ($i/30)"
  sleep 2
  
  # If we've waited too long, show the current state
  if [ $i -eq 30 ]; then
    echo "Current pods in crossplane-system namespace:"
    kubectl get pods -n crossplane-system
  fi
done

echo "Skyhook server deployed successfully with tag: ${TAG_TIMESTAMP}!"
