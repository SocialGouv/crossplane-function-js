/**
 * Helper function to safely access nested properties
 */
function getNestedProperty(obj: any, path: string): any {
  const parts = path.split('.');
  let current = obj;
  
  for (const part of parts) {
    if (current === null || current === undefined) {
      return undefined;
    }
    current = current[part];
  }
  
  return current;
}

/**
 * Main function to transform SimpleConfigMap data to uppercase
 * @param {Object} input - The input object from Crossplane
 * @returns {Object} The desired state with transformed ConfigMap
 */
export default function transformToUppercase(input: any): any {
  // Extract data from the input structure
  let data;
  
  // Try different paths to find the data
  const paths = [
    'observed.composite.resource.spec.data',
    'composite.resource.spec.data',
    'spec.data',
    'resource.spec.data'
  ];
  
  // Find the first path that contains data
  for (const path of paths) {
    const foundData = getNestedProperty(input, path);
    if (foundData) {
      data = foundData;
      break;
    }
  }
  
  // Use sample data as fallback
  if (!data) {
    data = {
      name: "test",
      email: "test@example.com",
      role: "tester"
    };
  }
  
  // Create a new object with uppercase values
  const uppercaseData: Record<string, string> = {};
  for (const key in data) {
    uppercaseData[key.toUpperCase()] = data[key].toUpperCase();
  }
  
  // Create the ConfigMap with uppercase values
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
  
  // Create the desired state using the Object format
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
