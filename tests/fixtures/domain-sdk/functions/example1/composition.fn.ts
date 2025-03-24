import { logger } from "skyhook-sdk"

export default function(input: any): any {
  logger.info("Example1 composition function started")
  
  const data = input.observed?.composite?.resource?.spec?.data || {};
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