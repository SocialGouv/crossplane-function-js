import {
  FunctionInput,
  CrossplaneDesiredResources
} from '../../../packages/server/src/types.ts';

export default function(input: FunctionInput): CrossplaneDesiredResources {
  // Use type assertion to access properties safely
  const inputAny = input as any;
  
  // Extract data from input
  const data = inputAny.input?.spec?.data || {};
  
  // Process the large data
  const processedData: Record<string, string> = {};
  for (const key in data) {
    // Convert keys to uppercase
    const uppercaseKey = key.toUpperCase();
    // Convert values to uppercase
    const uppercaseValue = String(data[key]).toUpperCase();
    processedData[uppercaseKey] = uppercaseValue;
  }
  
  // Create a resource with the processed data
  const resource = {
    apiVersion: "v1",
    kind: "ConfigMap",
    metadata: {
      name: "large-data",
      labels: {
        "large-data": "true"
      }
    },
    data: processedData
  };
  
  // Return the desired resources
  return {
    resources: {
      large: {
        resource: resource
      }
    }
  };
}
