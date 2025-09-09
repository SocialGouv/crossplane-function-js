import { logger, FieldRef, v1 } from "@crossplane-js/sdk"
import type { CrossplaneDesiredResources, CrossplaneObservedResources } from "@crossplane-js/sdk"

import type { XSimpleConfigMap } from "@/models/test.crossplane.io/v1beta1"

export default function(composite: XSimpleConfigMap, _resources: CrossplaneObservedResources): CrossplaneDesiredResources {
  logger.info("Composition function started")

  const namespace = composite.getNamespace()
  const isReady = composite.isReady()

  logger.info(`Ready: ${isReady}`)
  logger.info(composite)
  
  const data = composite.spec.data;
  logger.info({ data }, "Input data")
  
  const uppercaseData: Record<string, string> = {};
  for (const key in data) {
    uppercaseData[key.toUpperCase()] = data[key].toUpperCase();
  }
  
  const testConfigMap = new v1.ConfigMap({
    metadata: {
      name: "generated-configmap",
      namespace: namespace,
      labels: {
        example: "true"
      },
    },
    data: {
      ...uppercaseData,
      // hello: new FieldRef<string>(composite, "$.status.conditions[?(@.type=='Ready')].status", ""),
    },
  })

  const desired = {
    resources: {
      configmap: {
        resource: {
          apiVersion: "kubernetes.crossplane.io/v1alpha1",
          kind: "Object",
          metadata: {
            // name: `${composite.getName()}-configmap`,
            name: `generated-configmap`,
            annotations: {
              "uptest.upbound.io/timeout": "60"
            }
          },
          spec: {
            forProvider: {
              manifest: testConfigMap,
            },
            providerConfigRef: {
              // name: "in-cluster",
              name: "default",
            },
          },
        },
      },
    },
  }
  
  logger.info("Composition function completed")
  logger.debug({ desired }, "Generated output")
  
  return desired;
}
