import {
  FunctionInput,
  CrossplaneDesiredResources,
  KubernetesResource
} from '../../../packages/server/src/types.ts';

export default function(input: FunctionInput): CrossplaneDesiredResources {
  // Use type assertion to access properties safely
  const inputAny = input as any;
  
  // Extract data from the observed composite resource
  const data = inputAny.observed?.composite?.resource?.spec?.data || {};
  
  // Convert data keys and values to uppercase
  const uppercaseData: Record<string, string> = {};
  for (const key in data) {
    uppercaseData[key.toUpperCase()] = String(data[key]).toUpperCase();
  }
  
  // Create a ConfigMap
  const configMap = {
    apiVersion: "v1",
    kind: "ConfigMap",
    metadata: {
      name: "generated-configmap",
      namespace: "test-namespace",
      labels: {
        example: "true"
      }
    },
    data: uppercaseData
  };
  
  // Return the desired resources in Crossplane format
  return {
    resources: {
      configmap: {
        resource: {
          apiVersion: "kubernetes.crossplane.io/v1alpha2",
          kind: "Object",
          metadata: {
            name: "generated-configmap",
            annotations: {
              "uptest.upbound.io/timeout": "60"
            }
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
}
