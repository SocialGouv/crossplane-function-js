export default function(input: any): any {
  const { shouldError } = input.input.spec;
  
  // Throw an error if shouldError is true
  if (shouldError) {
    throw new Error("This is a deliberate error for testing");
  }
  
  // If we don't throw, return a normal result
  return {
    resources: {
      result: {
        resource: {
          apiVersion: "v1",
          kind: "ConfigMap",
          metadata: {
            name: "error-test-result",
          },
          data: {
            errorOccurred: "false"
          }
        }
      }
    }
  };
}
