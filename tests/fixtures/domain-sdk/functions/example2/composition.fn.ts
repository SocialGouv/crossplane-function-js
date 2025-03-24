import { logger } from "skyhook-sdk"

export default function(input: any): any {
  logger.info("Example2 composition function started")
  
  const data = input.observed?.composite?.resource?.spec?.data || {};
  logger.debug({ data }, "Input data")
  
  // Just return the data as is
  const configMap = {
    apiVersion: "v1",
    kind: "ConfigMap",
    metadata: {
      name: "example2-configmap",
      namespace: "test-skyhook"
    },
    data: data
  };
  
  const desired = {
    resources: {
      configmap: {
        resource: {
          apiVersion: "kubernetes.crossplane.io/v1alpha2",
          kind: "Object",
          metadata: {
            name: "example2-configmap"
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
  
  logger.info("Example2 composition function completed")
  
  return desired;
}