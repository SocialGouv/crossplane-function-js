import {
  FunctionInput,
  CrossplaneDesiredResources
} from '../../../src/types.js';

export default async function(input: FunctionInput): Promise<CrossplaneDesiredResources> {
  // Use type assertion to access properties safely
  const inputAny = input as any;
  
  // Extract delay and message from input
  const delay = inputAny.input?.spec?.delay || 0;
  const message = inputAny.input?.spec?.message || '';
  
  // Simulate an async operation with a delay
  await new Promise(resolve => setTimeout(resolve, delay));
  
  // Process the message
  const processedMessage = message.toUpperCase();
  
  // Create a resource with the processed data
  const resource = {
    apiVersion: "v1",
    kind: "ConfigMap",
    metadata: {
      name: "async-result",
      labels: {
        async: "true"
      }
    },
    data: {
      message: processedMessage,
      processed: true,
      timestamp: new Date().toISOString()
    }
  };
  
  // Return the desired resources
  return {
    resources: {
      delayed: {
        resource: resource
      }
    }
  };
}
