#!/usr/bin/env bash
set -e

# Function to handle errors
handle_error() {
  echo "Error occurred at line $1"
  echo "=== Update failed! ==="
  exit 1
}

# Set up error trap
trap 'handle_error $LINENO' ERR

# Default values
FUNCTION_NAME="function-skyhook"
REPOSITORY="localhost:5001"
IMAGE_NAME="crossplane-skyhook"
IMAGE_TAG="test"
PULL_POLICY="Always"
RUNTIME_CONFIG_NAME="skyhook-runtime-config"
SERVICE_NAME="skyhook-server"
SERVICE_NAMESPACE="crossplane-system"
SERVICE_PORT="50051"

# Parse command line arguments
while [[ $# -gt 0 ]]; do
  case $1 in
    --function-name)
      FUNCTION_NAME="$2"
      shift 2
      ;;
    --repository)
      REPOSITORY="$2"
      shift 2
      ;;
    --image-name)
      IMAGE_NAME="$2"
      shift 2
      ;;
    --image-tag)
      IMAGE_TAG="$2"
      shift 2
      ;;
    --pull-policy)
      PULL_POLICY="$2"
      shift 2
      ;;
    --runtime-config-name)
      RUNTIME_CONFIG_NAME="$2"
      shift 2
      ;;
    --service-name)
      SERVICE_NAME="$2"
      shift 2
      ;;
    --service-namespace)
      SERVICE_NAMESPACE="$2"
      shift 2
      ;;
    --service-port)
      SERVICE_PORT="$2"
      shift 2
      ;;
    *)
      echo "Unknown option: $1"
      exit 1
      ;;
  esac
done

# Update values.yaml
cat > values.yaml << EOF
# Default values for skyhook chart

# Function configuration
function:
  name: ${FUNCTION_NAME}
  package:
    repository: ${REPOSITORY}
    name: ${IMAGE_NAME}
    tag: ${IMAGE_TAG}
  pullPolicy: ${PULL_POLICY}

# RuntimeConfig configuration
runtimeConfig:
  name: ${RUNTIME_CONFIG_NAME}
  service:
    name: ${SERVICE_NAME}
    namespace: ${SERVICE_NAMESPACE}
    port: ${SERVICE_PORT}
EOF

echo "values.yaml updated successfully!"
echo "You can now deploy the chart with: helm install skyhook ."
