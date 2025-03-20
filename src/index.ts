import fs from 'fs';
import path from 'path';
import { runCode } from './runner.ts';
import type { NodeRequest, NodeResponse, NodeError } from './types.ts';

// Check if a file path was provided
if (process.argv.length < 3) {
  console.error('Usage: node index.js <code-file-path>');
  process.exit(1);
}

// Get the code file path
const codeFilePath = process.argv[2];

// Read the code file
let code: string;
try {
  code = fs.readFileSync(codeFilePath, 'utf-8');
  console.error(`Successfully read code file: ${codeFilePath}`);
} catch (err: unknown) {
  const error = err as Error;
  console.error(`Failed to read code file: ${error.message}`);
  process.exit(1);
}

// Set up error handling for uncaught exceptions
process.on('uncaughtException', (err) => {
  console.error('Uncaught exception:', err);
  // Don't exit the process, just log the error
});

// Set up error handling for unhandled promise rejections
process.on('unhandledRejection', (reason, promise) => {
  console.error('Unhandled promise rejection:', reason);
  // Don't exit the process, just log the error
});

// Create a queue for processing requests
const requestQueue: { data: Buffer, processed: boolean }[] = [];
let isProcessing = false;

// Function to process the next request in the queue
async function processNextRequest() {
  if (isProcessing || requestQueue.length === 0) {
    return;
  }
  
  isProcessing = true;
  const nextRequest = requestQueue.find(req => !req.processed);
  
  if (!nextRequest) {
    isProcessing = false;
    return;
  }
  
  nextRequest.processed = true;
  
  try {
    const data = nextRequest.data;
    console.error(`Processing request of length: ${data.length}`);
    
    // Parse the request
    const requestStr = data.toString();
    console.error(`Parsing request: ${requestStr.substring(0, 100)}...`);
    
    const request = JSON.parse(requestStr) as NodeRequest;
    console.error('Request parsed successfully');
    
    // The Go code sends both code and input, but we already have the code from the file
    // We'll use the code from the request if provided, otherwise use the code from the file
    const codeToRun = request.code || code;
    const input = request.input;
    
    console.error(`Running code with input: ${JSON.stringify(input).substring(0, 100)}...`);

    // Run the code
    const result = await runCode(codeToRun, input);
    console.error('Code execution completed');

    // Filter out logs from the result to avoid JSON parsing issues
    const resultToSend = {
      result: result.result,
      error: result.error
    };
    
    // Send the result back
    const resultStr = JSON.stringify(resultToSend);
    console.error(`Sending result: ${resultStr.substring(0, 100)}...`);
    
    // Ensure we flush the output before sending more data
    process.stdout.write(resultStr + '\n', () => {
      console.error('Result sent successfully');
    });
  } catch (err: unknown) {
    // Send error back
    const error = err as Error;
    console.error('Error processing request:', error);
    
    const nodeError: NodeError = {
      code: 500,
      message: error.message || 'Unknown error',
      stack: error.stack,
    };
    const response: NodeResponse = { error: nodeError };
    
    try {
      process.stdout.write(JSON.stringify(response) + '\n');
      console.error('Error response sent successfully');
    } catch (writeErr) {
      console.error('Failed to write error response:', writeErr);
    }
  } finally {
    isProcessing = false;
    
    // Remove processed requests from the queue
    while (requestQueue.length > 0 && requestQueue[0].processed) {
      requestQueue.shift();
    }
    
    // Process the next request if there are any
    if (requestQueue.length > 0) {
      setImmediate(processNextRequest);
    }
  }
}

// Process stdin for requests
process.stdin.on('data', (data) => {
  console.error(`Received data of length: ${data.length}`);
  
  // Add the request to the queue
  requestQueue.push({ data, processed: false });
  
  // Process the next request if we're not already processing one
  if (!isProcessing) {
    processNextRequest();
  }
});

// Handle process termination
process.on('SIGTERM', () => {
  console.error('Received SIGTERM, exiting...');
  process.exit(0);
});

process.on('SIGINT', () => {
  console.error('Received SIGINT, exiting...');
  process.exit(0);
});

// Log that we're ready
console.error(`Node.js process started for code file: ${codeFilePath}`);
