import { NodeResponse, NodeError } from './types';

/**
 * Runs the provided JavaScript/TypeScript code with the given input
 * @param code The JavaScript/TypeScript code to run
 * @param input The input data for the code
 * @returns The result of running the code
 */
export async function runCode(code: string, input: any): Promise<NodeResponse> {
  try {
    // Add a wrapper around the code to provide better error handling
    const wrappedCode = `
      try {
        // Validate input structure before executing user code
        if (!input) {
          throw new Error('Input is undefined or null');
        }
        
        // Wrap the user code in a try-catch block
        ${code}
      } catch (err) {
        // Provide detailed error information
        if (err.message && err.message.includes('Cannot read properties')) {
          // Enhance error message for property access errors
          throw new Error(\`Property access error: \${err.message}. This may be due to missing properties in the input structure.\`);
        }
        throw err;
      }
    `;
    
    // Create a function from the wrapped code
    const AsyncFunction = Object.getPrototypeOf(async function(){}).constructor;
    const fn = new AsyncFunction('input', wrappedCode);
    
    // Execute the function with the input
    const result = await fn(input);
    return { result };
  } catch (err: unknown) {
    // Format the error
    const error = err as Error;
    console.error('Error executing function:', error.message);
    if (error.stack) {
      console.error('Stack trace:', error.stack);
    }
    
    const nodeError: NodeError = {
      code: 500,
      message: error.message || 'Unknown error',
      stack: error.stack,
    };
    return { error: nodeError };
  }
}
