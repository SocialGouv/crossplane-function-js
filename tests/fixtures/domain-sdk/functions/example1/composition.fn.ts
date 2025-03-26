import type { CrossplaneDesiredResources, FunctionInput } from "skyhook-sdk"
import { logger } from "skyhook-sdk"

export default function(input: FunctionInput): CrossplaneDesiredResources {
  logger.info("Example1 composition function started")
  
  // Use type assertion to access properties safely
  const inputAny = input as any;
  const data = inputAny.observed?.composite?.resource?.spec?.data || {};
  logger.debug({ data }, "Input data")
  
  const uppercaseData: Record<string, string> = {};
  for (const key in data) {
    uppercaseData[key.toUpperCase()] = data[key].toUpperCase();
  }
  
  const configMap = {
    apiVersion: "v1",
    kind: "ConfigMap",
    metadata: {
      name: "example1-configmap",
      namespace: "test-skyhook",
      labels: {
        example: "true"
      }
    },
    data: uppercaseData
  };
  
  const desired = {
    resources: {
      configmap: {
        resource: {
          apiVersion: "kubernetes.crossplane.io/v1alpha2",
          kind: "Object",
          metadata: {
            name: "example1-configmap"
          },
          spec: {
            forProvider: {
              manifest: configMap
            },
            providerConfigRef: {
              name: "default"
            }
          }
        }
      }
    }
  };
  
  logger.info("Example1 composition function completed")
  
  return desired;
}