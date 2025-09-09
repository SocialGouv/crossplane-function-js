import { expect } from 'chai';
import { describe, it } from 'mocha';
import path from 'path';
import { fileURLToPath } from 'url';
import { executeCodeForTest } from './helpers/test-executor.js';

// Helper function to safely access nested properties
function assertHasResources(result: any): asserts result is { resources: Record<string, any> } {
  if (!result || typeof result !== 'object' || !('resources' in result)) {
    throw new Error('Result does not have resources property');
  }
}

// Get the directory name of the current module
const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

describe('JavaScript Code Execution', () => {
  it('should execute a simple function correctly', async () => {
    // Path to a test function
    const codePath = path.resolve(__dirname, 'fixtures/simple-function.ts');
    
    // Sample input similar to what would come from the gRPC server
    const input = {
      input: {
        spec: {
          data: {
            name: 'test',
            value: 123
          }
        }
      }
    };
    
    // Execute the code
    const result = await executeCodeForTest(codePath, input);
    
    // Verify the result
    expect(result).to.have.property('result');
    
    // Use type assertion to safely access nested properties
    const resultObj = result.result as Record<string, any>;
    assertHasResources(resultObj);
    
    expect(resultObj.resources).to.have.property('configmap');
    expect(resultObj.resources.configmap).to.have.property('resource');
    
    // Verify the resource has the expected structure
    const resource = resultObj.resources.configmap.resource;
    expect(resource).to.have.property('apiVersion', 'v1');
    expect(resource).to.have.property('kind', 'ConfigMap');
    expect(resource).to.have.property('metadata');
    expect(resource.metadata).to.have.property('name', 'test-configmap');
    expect(resource).to.have.property('data');
    expect(resource.data).to.have.property('NAME', 'TEST');
    expect(resource.data).to.have.property('VALUE', '123');
  });

  it('should handle async functions correctly', async () => {
    // Path to a test function with async operations
    const codePath = path.resolve(__dirname, 'fixtures/async-function.ts');
    
    // Sample input
    const input = {
      input: {
        spec: {
          delay: 100,
          message: 'hello world'
        }
      }
    };
    
    // Execute the code
    const result = await executeCodeForTest(codePath, input);
    
    // Verify the result
    expect(result).to.have.property('result');
    
    // Use type assertion to safely access nested properties
    const resultObj = result.result as Record<string, any>;
    assertHasResources(resultObj);
    
    expect(resultObj.resources).to.have.property('delayed');
    expect(resultObj.resources.delayed.resource).to.have.property('data');
    expect(resultObj.resources.delayed.resource.data).to.have.property('message', 'HELLO WORLD');
    expect(resultObj.resources.delayed.resource.data).to.have.property('processed', true);
  });

  it('should handle errors in the code correctly', async () => {
    // Path to a test function that throws an error
    const codePath = path.resolve(__dirname, 'fixtures/error-function.ts');
    
    // Sample input
    const input = {
      input: {
        spec: {
          shouldError: true
        }
      }
    };
    
    // Execute the code
    const result = await executeCodeForTest(codePath, input);
    
    // Verify the error is captured correctly
    expect(result).to.have.property('error');
    expect(result.error).to.have.property('code', 422);
    expect(result.error).to.have.property('message').that.includes('Function execution error');
    expect(result.error).to.have.property('stack');
  });

  it('should process Crossplane-specific input correctly', async () => {
    // Path to a test function that processes Crossplane resources
    const codePath = path.resolve(__dirname, 'fixtures/crossplane-function.ts');
    
    // Sample input with Crossplane structure
    const input = {
      input: {
        apiVersion: 'test.crossplane.io/v1beta1',
        kind: 'SimpleConfigMap',
        metadata: {
          name: 'test-simple-configmap',
          namespace: 'test-namespace'
        },
        spec: {
          data: {
            name: 'John Doe',
            email: 'john.doe@example.com',
            role: 'developer'
          }
        }
      },
      observed: {
        composite: {
          resource: {
            apiVersion: 'test.crossplane.io/v1beta1',
            kind: 'SimpleConfigMap',
            metadata: {
              name: 'test-simple-configmap',
              namespace: 'test-namespace'
            },
            spec: {
              data: {
                name: 'John Doe',
                email: 'john.doe@example.com',
                role: 'developer'
              }
            }
          }
        }
      }
    };
    
    // Execute the code
    const result = await executeCodeForTest(codePath, input);
    
    // Verify the result has the expected Crossplane structure
    expect(result).to.have.property('result');
    
    // Use type assertion to safely access nested properties
    const resultObj = result.result as Record<string, any>;
    assertHasResources(resultObj);
    
    // Check the Crossplane Object resource
    expect(resultObj.resources).to.have.property('configmap');
    const crossplaneObj = resultObj.resources.configmap.resource;
    expect(crossplaneObj).to.have.property('apiVersion', 'kubernetes.crossplane.io/v1alpha1');
    expect(crossplaneObj).to.have.property('kind', 'Object');
    expect(crossplaneObj).to.have.property('metadata');
    expect(crossplaneObj.metadata).to.have.property('name', 'generated-configmap');
    expect(crossplaneObj).to.have.property('spec');
    expect(crossplaneObj.spec).to.have.property('forProvider');
    expect(crossplaneObj.spec.forProvider).to.have.property('manifest');
    
    // Check the embedded ConfigMap
    const configMap = crossplaneObj.spec.forProvider.manifest;
    expect(configMap).to.have.property('apiVersion', 'v1');
    expect(configMap).to.have.property('kind', 'ConfigMap');
    expect(configMap).to.have.property('metadata');
    expect(configMap.metadata).to.have.property('name', 'generated-configmap');
    expect(configMap).to.have.property('data');
    expect(configMap.data).to.have.property('NAME', 'JOHN DOE');
    expect(configMap.data).to.have.property('EMAIL', 'JOHN.DOE@EXAMPLE.COM');
    expect(configMap.data).to.have.property('ROLE', 'DEVELOPER');
  });

  it('should handle large inputs correctly', async () => {
    // Path to a test function that handles large inputs
    const codePath = path.resolve(__dirname, 'fixtures/large-input-function.ts');
    
    // Create a large input
    const largeData: Record<string, string> = {};
    for (let i = 0; i < 100; i++) {
      largeData[`key${i}`] = `value${i}`.repeat(100);
    }
    
    const input = {
      input: {
        spec: {
          data: largeData
        }
      }
    };
    
    // Execute the code
    const result = await executeCodeForTest(codePath, input);
    
    // Verify the result
    expect(result).to.have.property('result');
    
    // Use type assertion to safely access nested properties
    const resultObj = result.result as Record<string, any>;
    assertHasResources(resultObj);
    
    expect(resultObj.resources).to.have.property('large');
    expect(resultObj.resources.large.resource).to.have.property('metadata');
    expect(resultObj.resources.large.resource.metadata).to.have.property('name', 'large-data');
    expect(resultObj.resources.large.resource).to.have.property('data');
    
    // Verify the data was processed correctly
    const resultData = resultObj.resources.large.resource.data;
    expect(Object.keys(resultData).length).to.equal(100);
    expect(resultData.KEY0).to.equal('VALUE0'.repeat(100));
  });
});
