export default function(input: any): any {
  const data = input.input.spec.data;
  
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
