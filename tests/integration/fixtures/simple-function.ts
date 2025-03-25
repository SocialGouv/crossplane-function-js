import { FunctionInput, CrossplaneDesiredResources, KubernetesResource } from '../../../src/types.js';

export default function(input: FunctionInput): CrossplaneDesiredResources {
  // Extract data based on the input structure
  let data: Record<string, string | number | boolean>;
  
  // Use type assertion to access properties safely
  const inputAny = input as any;
  
  // Check if input has the test structure (simple-function.ts format)
  if (inputAny.input?.spec?.data) {
    data = inputAny.input.spec.data;
  }
  // Check if input has the Crossplane structure
  else if (inputAny.observed?.composite?.resource?.spec?.data) {
    data = inputAny.observed.composite.resource.spec.data;
  }
  // Fallback
  else {
    data = {};
  }
  
  // Convert data keys and values to uppercase
  const uppercaseData: Record<string, string> = {};
  for (const key in data) {
    uppercaseData[key.toUpperCase()] = String(data[key]).toUpperCase();
  }
  
  // Create a ConfigMap resource
  const configMap = {
    apiVersion: "v1",
    kind: "ConfigMap",
    metadata: {
      name: "test-configmap",
      labels: {
        test: "true"
      }
    },
    data: uppercaseData
  };
  
  // Return the desired resources
  return {
    resources: {
      configmap: {
        resource: configMap
      }
    }
  };
}
