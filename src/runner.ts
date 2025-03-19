import { NodeResponse, NodeError } from './types';

/**
 * Runs the provided JavaScript/TypeScript code with the given input
 * @param code The JavaScript/TypeScript code to run
 * @param input The input data for the code
 * @returns The result of running the code
 */
export async function runCode(code: string, input: any): Promise<NodeResponse> {
  try {
    // Create a function from the code
    const AsyncFunction = Object.getPrototypeOf(async function(){}).constructor;
    const fn = new AsyncFunction('input', code);
    
    // Execute the function with the input
    const result = await fn(input);
    return { result };
  } catch (err: unknown) {
    // Format the error
    const error = err as Error;
    const nodeError: NodeError = {
      code: 500,
      message: error.message || 'Unknown error',
      stack: error.stack,
    };
    return { error: nodeError };
  }
}
