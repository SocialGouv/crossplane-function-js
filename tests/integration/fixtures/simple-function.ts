export default function(input: any): any {
  const data = input.input.spec.data;
  
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
