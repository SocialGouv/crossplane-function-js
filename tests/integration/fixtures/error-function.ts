import {
  FunctionInput,
  CrossplaneDesiredResources
} from '../../../src/types.js';

export default function(input: FunctionInput): CrossplaneDesiredResources {
  // Use type assertion to access properties safely
  const inputAny = input as any;
  
  // Extract shouldError flag from input
  const shouldError = inputAny.input?.spec?.shouldError || false;
  
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
