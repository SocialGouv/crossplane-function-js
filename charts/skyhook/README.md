# Skyhook Helm Chart

This Helm chart deploys the Crossplane Skyhook Function and DeploymentRuntimeConfig resources.

## Prerequisites

- Kubernetes cluster with Crossplane installed
- Helm 3.0+

## Installation

```bash
# Add the repository (if hosted in a Helm repository)
# helm repo add skyhook-repo https://example.com/charts
# helm repo update

# Install the chart
helm install skyhook ./charts/skyhook
```

## Configuration

The following table lists the configurable parameters of the Skyhook chart and their default values.

| Parameter | Description | Default |
|-----------|-------------|---------|
| `function.name` | Name of the Function resource | `function-skyhook` |
| `function.package.repository` | Repository for the Function image | `localhost:5001` |
| `function.package.name` | Name of the Function image | `crossplane-skyhook` |
| `function.package.tag` | Tag of the Function image | `test` |
| `function.pullPolicy` | Image pull policy | `Always` |
| `runtimeConfig.name` | Name of the DeploymentRuntimeConfig resource | `skyhook-runtime-config` |
| `runtimeConfig.service.name` | Name of the service | `skyhook-server` |
| `runtimeConfig.service.namespace` | Namespace of the service | `crossplane-system` |
| `runtimeConfig.service.port` | Port of the service | `9443` |

## Usage

After installing the chart, the Crossplane Function and DeploymentRuntimeConfig will be available for use in your Crossplane compositions.

Example composition using the Skyhook function:

```yaml
apiVersion: apiextensions.crossplane.io/v1
kind: Composition
metadata:
  name: example-composition
spec:
  compositeTypeRef:
    apiVersion: example.org/v1alpha1
    kind: ExampleXR
  mode: Pipeline
  pipeline:
    - step: transform-with-skyhook
      functionRef:
        name: function-skyhook
      input:
        apiVersion: skyhook.fn.crossplane.io/v1beta1
        kind: Input
        spec:
          source:
            inline: |
              // Your JavaScript/TypeScript code here
```

## Uninstallation

```bash
helm uninstall skyhook
