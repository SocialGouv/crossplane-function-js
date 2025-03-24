export default async function(input: any): Promise<any> {
  const { delay, message } = input.input.spec;
  
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
