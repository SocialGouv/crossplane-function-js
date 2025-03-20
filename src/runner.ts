import type { NodeResponse, NodeError } from './types.ts';
import * as fs from 'fs/promises';
import * as path from 'path';
import * as os from 'os';

/**
 * Runs the provided JavaScript/TypeScript code with the given input
 * @param code The JavaScript/TypeScript code to run
 * @param input The input data for the code
 * @returns The result of running the code
 */
export async function runCode(code: string, input: any): Promise<NodeResponse> {
  try {
    // Validate input
    if (!input) {
      throw new Error('Input is undefined or null');
    }

    // Create a temporary dir
    const tempDir = await fs.mkdtemp(path.join(os.tmpdir(), 'crossplane-skyhook'));
    
    const tempFilePath = path.join(tempDir, `module-${Date.now()}.mjs`);
    
    try {
      // Modify the code to capture console.log output
      const moduleCode = `
        // Capture console output
        const logs = [];
        const originalConsoleLog = console.log;
        const originalConsoleError = console.error;
        
        console.log = function(...args) {
          logs.push(['log', ...args]);
          // originalConsoleLog(...args);
        };
        
        console.error = function(...args) {
          logs.push(['error', ...args]);
          originalConsoleError(...args);
        };
        
        ${code}
        
        // Export the logs array
        export const capturedLogs = logs;
      `;
      
      // Write the module to a temporary file
      await fs.writeFile(tempFilePath, moduleCode);
      
      // Import the module dynamically
      const moduleUrl = `file://${tempFilePath}`;
      const module = await import(moduleUrl);
      
      // Execute the default exported function with the input
      const result = await module.default(input);
      
      // Return the result along with any captured logs
      return { 
        result,
        logs: module.capturedLogs || []
      };
    } finally {
      // Clean up the temporary file
      await fs.unlink(tempFilePath).catch(() => void 0);
    }
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
