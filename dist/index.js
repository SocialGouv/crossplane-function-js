#!/usr/bin/env node
import fs from 'fs';
import { runCode } from './runner.js';
// Check if a file path was provided
if (process.argv.length < 3) {
    console.error('Usage: node index.js <code-file-path>');
    process.exit(1);
}
// Get the code file path
const codeFilePath = process.argv[2];
// Read the code file
let code;
try {
    code = fs.readFileSync(codeFilePath, 'utf-8');
    console.error(`Successfully read code file: ${codeFilePath}`);
}
catch (err) {
    const error = err;
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
// Process stdin for requests
process.stdin.on('data', async (data) => {
    try {
        console.error(`Received data of length: ${data.length}`);
        // Parse the request
        const requestStr = data.toString();
        console.error(`Parsing request: ${requestStr.substring(0, 100)}...`);
        const request = JSON.parse(requestStr);
        console.error('Request parsed successfully');
        // The Go code sends both code and input, but we already have the code from the file
        // We'll use the code from the request if provided, otherwise use the code from the file
        const codeToRun = request.code || code;
        const input = request.input;
        console.error(`Running code with input: ${JSON.stringify(input).substring(0, 100)}...`);
        // Run the code
        const result = await runCode(codeToRun, input);
        console.error('Code execution completed');
        // Send the result back
        const resultStr = JSON.stringify(result);
        console.error(`Sending result: ${resultStr.substring(0, 100)}...`);
        process.stdout.write(resultStr + '\n');
        console.error('Result sent successfully');
    }
    catch (err) {
        // Send error back
        const error = err;
        console.error('Error processing request:', error);
        const nodeError = {
            code: 500,
            message: error.message || 'Unknown error',
            stack: error.stack,
        };
        const response = { error: nodeError };
        try {
            process.stdout.write(JSON.stringify(response) + '\n');
            console.error('Error response sent successfully');
        }
        catch (writeErr) {
            console.error('Failed to write error response:', writeErr);
        }
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
