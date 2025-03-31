import path from 'path';
import fs from 'fs';
import { fileURLToPath } from 'url';
import { NodeResponse, NodeError, FunctionInput } from '../../../packages/server/src/types.ts';

// Create a simple logger for testing
const logger = {
  debug: (message: string) => console.log(`[DEBUG] ${message}`),
  info: (message: string) => console.log(`[INFO] ${message}`),
  error: (message: string) => console.error(`[ERROR] ${message}`)
};

/**
 * Test version of executeCode that uses ES modules instead of CommonJS
 * This is specifically for testing purposes
 */
export async function executeCodeForTest(codeFilePath: string, input: FunctionInput): Promise<NodeResponse> {
  const executionTimeout = 5000; // Shorter timeout for tests
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
      
      // Import the module directly from the file path using ES modules
      logger.debug(`Importing module from file: ${codeFilePath}`);
      
      let module;
      try {
        // Convert the file path to a URL for ES module imports
        const fileUrl = pathToFileURL(codeFilePath).href;
        module = await import(fileUrl);
      } catch (importErr) {
        logger.error(`Error importing module: ${(importErr as Error).message}`);
        throw new Error(`Module import failed: ${(importErr as Error).message}`);
      }
      
      if (!module.default || typeof module.default !== 'function') {
        throw new Error('Module does not export a default function');
      }
      
      // Execute the default exported function with the input
      logger.debug('Executing default exported function');
      
      let result;
      try {
        result = await module.default(input);
        logger.debug('Function execution completed');
      } catch (execErr) {
        logger.error(`Error during function execution: ${(execErr as Error).message}`);
        throw new Error(`Function execution error: ${(execErr as Error).message}`);
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
    logger.error(`Error executing function: ${error.message}`);
    
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

// Helper function to convert a file path to a URL
function pathToFileURL(filePath: string): URL {
  const __filename = fileURLToPath(import.meta.url);
  const __dirname = path.dirname(__filename);
  
  // If the path is relative, resolve it relative to the current file
  if (!path.isAbsolute(filePath)) {
    filePath = path.resolve(__dirname, filePath);
  }
  
  // Check if the file exists
  if (!fs.existsSync(filePath)) {
    throw new Error(`File not found: ${filePath}`);
  }
  
  return new URL(`file://${filePath}`);
}
