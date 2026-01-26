import { createLogger } from "@crossplane-js/libs"

import { createModel } from "./model.ts"
import type { NodeResponse, NodeError, FunctionInput, RunFunctionRequest } from "./types.ts"

// Create a logger for this module
const moduleLogger = createLogger("executor")

/**
 * Executes JavaScript/TypeScript code from a file with the given input
 * @param code The code to execute
 * @param input The input data for the code
 * @returns The result of running the code
 */
export async function executeCode(
  codeFilePath: string,
  input: FunctionInput
): Promise<NodeResponse> {
  // Set up a timeout to prevent infinite loops or long-running code
  const executionTimeout = 25000 // 25 seconds (less than the 30s in Go to ensure we can respond)
  let timeoutId: NodeJS.Timeout | null = null

  try {
    // Create a promise that rejects after the timeout
    const timeoutPromise = new Promise<never>((_, reject) => {
      timeoutId = setTimeout(() => {
        reject(new Error(`Function execution timed out after ${executionTimeout / 1000} seconds`))
      }, executionTimeout)
    })

    // Create the actual execution promise
    const executionPromise = (async () => {
      // Validate input
      if (!input) {
        throw new Error("Input is undefined or null")
      }

      // Import the module directly from the file path
      moduleLogger.debug(`Importing module from file: ${codeFilePath}`)

      let module
      try {
        // Use standard Node.js dynamic import
        module = await import(codeFilePath)
      } catch (importErr) {
        moduleLogger.error(`Error importing module: ${(importErr as Error).message}`)
        if ((importErr as Error).stack) {
          moduleLogger.debug(`Import error stack trace: ${(importErr as Error).stack}`)
        }
        throw new Error(`Module import failed: ${(importErr as Error).message}`)
      }

      if (!module.default || typeof module.default !== "function") {
        throw new Error("Module does not export a default function")
      }

      // Execute the default exported function with the input
      moduleLogger.debug("Executing default exported function")

      let result
      try {
        const inputData = input as any

        const compositeResource = inputData?.observed?.composite?.resource
        const composite = createModel(compositeResource)

        // // Add observed resources
        const observedResources = inputData?.observed?.resources
        moduleLogger.info({observedResources}, "Observed resources")

        // Extra resources injected by Crossplane based on previously requested
        // extraResourceRequirements. These are forwarded to the user function
        // alongside the composite inside a single RunFunctionRequest object.
        const extraResources = inputData?.extraResources

        const req: RunFunctionRequest = {
          composite,
          observed: observedResources,
          extraResources,
        }

        result = await module.default(req)

        moduleLogger.debug("Function execution completed")
      } catch (execErr) {
        moduleLogger.error(`Error during function execution: ${(execErr as Error).message}`)
        if ((execErr as Error).stack) {
          moduleLogger.debug(`Execution error stack trace: ${(execErr as Error).stack}`)
        }
        throw new Error(`Function execution error: ${(execErr as Error).message}`)
      }

      // moduleLogger.debug({result}, "Result");

      // Return the result
      return { result }
    })()

    // Race the execution against the timeout
    const result = await Promise.race([executionPromise, timeoutPromise])

    // Clear the timeout if execution completed successfully
    if (timeoutId) {
      clearTimeout(timeoutId)
      timeoutId = null
    }

    return result as NodeResponse
  } catch (err: unknown) {
    // Clear the timeout if it's still active
    if (timeoutId) {
      clearTimeout(timeoutId)
      timeoutId = null
    }

    // Format the error
    const error = err as Error
    moduleLogger.error(`Error executing function: ${error.message}`)
    if (error.stack) {
      moduleLogger.debug(`Stack trace: ${error.stack}`)
    }

    // Categorize the error
    let errorCode = 500
    if (error.message.includes("timed out")) {
      errorCode = 408 // Request Timeout
    } else if (error.message.includes("Module import failed")) {
      errorCode = 400 // Bad Request - code issue
    } else if (error.message.includes("Function execution error")) {
      errorCode = 422 // Unprocessable Entity - runtime error in user code
    }

    const nodeError: NodeError = {
      code: errorCode,
      message: error.message || "Unknown error",
      stack: error.stack,
    }
    return { error: nodeError }
  }
}
