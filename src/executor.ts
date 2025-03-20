import type { NodeResponse, NodeError } from './types.ts';
import { createLogger } from './logger.ts';

// Create a logger for this module
const moduleLogger = createLogger('executor');

/**
 * Executes JavaScript/TypeScript code from a file with the given input
 * @param code The code to execute
 * @param input The input data for the code
 * @returns The result of running the code
 */
export async function executeCode(code: string, input: any): Promise<NodeResponse> {
  // Set up a timeout to prevent infinite loops or long-running code
  const executionTimeout = 25000; // 25 seconds (less than the 30s in Go to ensure we can respond)
  let timeoutId: NodeJS.Timeout | null = null;
  
  try {
    // Create a promise that rejects after the timeout
    const timeoutPromise = new Promise<never>((_, reject) => {
      timeoutId = setTimeout(() => {
        reject(new Error(`Function execution timed out after ${executionTimeout/1000} seconds`));
      }, executionTimeout);
    });
    
    // Create the actual execution promise
    const executionPromise = (async () => {
      // Validate input
      if (!input) {
        throw new Error('Input is undefined or null');
      }
      
      if (!code) {
        throw new Error('Code is required');
      }

      // Create a temporary file for the code
      const { writeFile, mkdtemp } = await import('fs/promises');
      const { join } = await import('path');
      const { tmpdir } = await import('os');
      const { randomBytes } = await import('crypto');
      
      // Create a temporary directory
      const tempDir = await mkdtemp(join(tmpdir(), 'skyhook-'));
      const tempFilePath = join(tempDir, `code-${randomBytes(8).toString('hex')}.ts`);
      
      // Write the code to the temporary file
      await writeFile(tempFilePath, code);
      moduleLogger.debug(`Code written to temporary file: ${tempFilePath}`);
      
      // Import the module directly from the file path
      const fileUrl = `file://${tempFilePath}`;
      moduleLogger.debug(`Importing module from file: ${fileUrl}`);
      
      let module;
      try {
        module = await import(fileUrl);
      } catch (importErr) {
        moduleLogger.error(`Error importing module: ${(importErr as Error).message}`);
        if ((importErr as Error).stack) {
          moduleLogger.debug(`Import error stack trace: ${(importErr as Error).stack}`);
        }
        throw new Error(`Module import failed: ${(importErr as Error).message}`);
      }
      
      if (!module.default || typeof module.default !== 'function') {
        throw new Error('Module does not export a default function');
      }
      
      // Execute the default exported function with the input
      moduleLogger.debug('Executing default exported function');
      
      let result;
      try {
        result = await module.default(input);
        moduleLogger.debug('Function execution completed');
      } catch (execErr) {
        moduleLogger.error(`Error during function execution: ${(execErr as Error).message}`);
        if ((execErr as Error).stack) {
          moduleLogger.debug(`Execution error stack trace: ${(execErr as Error).stack}`);
        }
        throw new Error(`Function execution error: ${(execErr as Error).message}`);
      }
      
      // Clean up the temporary file
      try {
        const { unlink, rmdir } = await import('fs/promises');
        await unlink(tempFilePath);
        await rmdir(tempDir);
        moduleLogger.debug(`Temporary file and directory cleaned up: ${tempFilePath}`);
      } catch (cleanupErr) {
        moduleLogger.warn(`Error cleaning up temporary file: ${(cleanupErr as Error).message}`);
      }
      
      // Return the result
      return { result };
    })();
    
    // Race the execution against the timeout
    const result = await Promise.race([executionPromise, timeoutPromise]);
    
    // Clear the timeout if execution completed successfully
    if (timeoutId) {
      clearTimeout(timeoutId);
      timeoutId = null;
    }
    
    return result as NodeResponse;
  } catch (err: unknown) {
    // Clear the timeout if it's still active
    if (timeoutId) {
      clearTimeout(timeoutId);
      timeoutId = null;
    }
    
    // Format the error
    const error = err as Error;
    moduleLogger.error(`Error executing function: ${error.message}`);
    if (error.stack) {
      moduleLogger.debug(`Stack trace: ${error.stack}`);
    }
    
    // Ensure logs are flushed for errors
    try {
      const { flushLogs } = await import('./logger.ts');
      await flushLogs();
    } catch (flushErr) {
      moduleLogger.error(`Error flushing logs: ${(flushErr as Error).message}`);
    }
    
    // Categorize the error
    let errorCode = 500;
    if (error.message.includes('timed out')) {
      errorCode = 408; // Request Timeout
    } else if (error.message.includes('Module import failed')) {
      errorCode = 400; // Bad Request - code issue
    } else if (error.message.includes('Function execution error')) {
      errorCode = 422; // Unprocessable Entity - runtime error in user code
    }
    
    const nodeError: NodeError = {
      code: errorCode,
      message: error.message || 'Unknown error',
      stack: error.stack,
    };
    return { error: nodeError };
  }
}
