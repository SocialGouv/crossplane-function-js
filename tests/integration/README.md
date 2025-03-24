# Integration Tests for Node Processes

This directory contains integration tests for the Node.js processes in the Crossplane Skyhook project. These tests focus on verifying that JavaScript/TypeScript code is correctly executed by the Node subprocess.

## Test Structure

The integration tests are organized as follows:

- `node-execution.test.ts`: Main test file containing test cases for various scenarios
- `fixtures/`: Directory containing test functions used by the tests
- `helpers/`: Directory containing helper functions for testing

## Test Cases

The integration tests cover the following scenarios:

1. **Basic Function Execution**: Tests that a simple JavaScript function is executed correctly
2. **Async Functions**: Tests that async functions with Promises are handled correctly
3. **Error Handling**: Tests error cases (syntax errors, runtime errors)
4. **Crossplane-Specific Input**: Tests functions that process Crossplane resource structures
5. **Large Inputs**: Tests handling of large input data

## Running the Tests

To run the integration tests:

```bash
yarn test:integration
```

## Implementation Details

The tests use a custom test executor (`test-executor.ts`) that simulates the behavior of the actual executor but uses ES modules instead of CommonJS. This allows us to test the code execution logic without the complexity of the process management.

Each test:
1. Prepares a test function and input data
2. Executes the function using the test executor
3. Verifies the output matches the expected result

## Adding New Tests

To add a new test case:

1. Create a new test function in the `fixtures/` directory
2. Add a new test case in `node-execution.test.ts`
3. Run the tests to verify the new test case works as expected
