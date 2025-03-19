#!/usr/bin/env node

import fs from 'fs';
import path from 'path';
import { runCode } from './runner';
import { NodeRequest, NodeResponse, NodeError } from './types';

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
} catch (err: unknown) {
  const error = err as Error;
  console.error(`Failed to read code file: ${error.message}`);
  process.exit(1);
}

// Process stdin for requests
process.stdin.on('data', async (data) => {
  try {
    // Parse the request
    const request = JSON.parse(data.toString()) as NodeRequest;
    const { input } = request;

    // Run the code
    const result = await runCode(code, input);

    // Send the result back
    process.stdout.write(JSON.stringify(result) + '\n');
  } catch (err: unknown) {
    // Send error back
    const error = err as Error;
    const nodeError: NodeError = {
      code: 500,
      message: error.message || 'Unknown error',
      stack: error.stack,
    };
    const response: NodeResponse = { error: nodeError };
    process.stdout.write(JSON.stringify(response) + '\n');
  }
});

// Handle process termination
process.on('SIGTERM', () => {
  process.exit(0);
});

process.on('SIGINT', () => {
  process.exit(0);
});

// Log that we're ready
console.error(`Node.js process started for code file: ${codeFilePath}`);
