import lodash from "lodash"

export default function transformToUppercase(input: any): any {
  const data = lodash.get(input, 'observed.composite.resource.spec.data');
  
  const uppercaseData: Record<string, string> = {};
  for (const key in data) {
    uppercaseData[key.toUpperCase()] = data[key].toUpperCase();
  }
  
  const configMap = {
    apiVersion: "v1",
    kind: "ConfigMap",
    metadata: {
      name: "generated-configmap",
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
            name: "generated-configmap",
            annotations: {
              "uptest.upbound.io/timeout": "60"
            }
          },
          spec: {
            // Watch for changes to the ConfigMap object.
            // Watching resources is an alpha feature and needs to be enabled with --enable-watches
            // in the provider to get this configuration working.
            // watch: true
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
  
  return desired;
}
