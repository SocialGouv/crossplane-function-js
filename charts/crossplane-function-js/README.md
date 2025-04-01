# Crossplane Function JS Helm Chart

This Helm chart deploys the Crossplane Function JS and DeploymentRuntimeConfig resources.

## Prerequisites

- Kubernetes cluster with Crossplane installed
- Helm 3.0+

## Installation

```bash
helm install crossplane-function-js oci://ghcr.io/socialgouv/helm/crossplane-function-js --version 0.0.2
```

## Configuration

The following table lists the configurable parameters of the XFuncJS chart and their default values.

| Parameter                         | Description                                  | Default                  |
| --------------------------------- | -------------------------------------------- | ------------------------ |
| `function.name`                   | Name of the Function resource                | `function-xfuncjs`       |
| `function.package.repository`     | Repository for the Function image            | `localhost:5001`         |
| `function.package.name`           | Name of the Function image                   | `xfuncjs-server`         |
| `function.package.tag`            | Tag of the Function image                    | `test`                   |
| `function.pullPolicy`             | Image pull policy                            | `Always`                 |
| `runtimeConfig.name`              | Name of the DeploymentRuntimeConfig resource | `xfuncjs-runtime-config` |
| `runtimeConfig.service.name`      | Name of the service                          | `xfuncjs-server`         |
| `runtimeConfig.service.namespace` | Namespace of the service                     | `crossplane-system`      |
| `runtimeConfig.service.port`      | Port of the service                          | `9443`                   |
| `config.logLevel`                 | Log level (debug, info, warn, error)         | `info`                   |
| `config.logFormat`                | Log format (auto, text, json)                | `auto`                   |
| `config.tempDir`                  | Temporary directory for code files           | `""` (uses default)      |
| `config.gcInterval`               | Garbage collection interval                  | `""` (uses default)      |
| `config.idleTimeout`              | Idle process timeout                         | `""` (uses default)      |
| `config.nodeServerPort`           | Port for the Node.js HTTP server             | `""` (uses default)      |
| `config.healthCheckWait`          | Timeout for health check                     | `""` (uses default)      |
| `config.healthCheckInterval`      | Interval for health check polling            | `""` (uses default)      |
| `config.nodeRequestTimeout`       | Timeout for Node.js requests                 | `""` (uses default)      |
| `config.tls.enabled`              | Enable TLS                                   | `false`                  |
| `config.tls.certFile`             | Path to TLS certificate file                 | `""`                     |
| `config.tls.keyFile`              | Path to TLS key file                         | `""`                     |

## Usage

After installing the chart, the Crossplane Function and DeploymentRuntimeConfig will be available for use in your Crossplane compositions.

Example composition using the XFuncJS function:

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
    - step: transform-with-xfuncjs
      functionRef:
        name: function-xfuncjs
      input:
        apiVersion: xfuncjs.fn.crossplane.io/v1beta1
        kind: Input
        spec:
          source:
            inline: |
              // Your JavaScript/TypeScript code here
```

## Uninstallation

```bash
helm uninstall xfuncjs
```
