import { createLogger } from "@crossplane-js/libs"
import express from "express"
import type { Request, Response, NextFunction, RequestHandler } from "express"

import { executeCode } from "./executor"
import type { NodeRequest } from "./types"

// Create a logger for this module
const moduleLogger = createLogger("server")

/**
 * Creates and configures an Express server
 * @param port The port to listen on
 * @returns The configured Express app
 */
export function createServer(port: number, codeFilePath: string) {
  const app = express()

  // Configure middleware
  app.use(express.json({ limit: "10mb" }))

  // Add request logging
  app.use((req: Request, res: Response, next: NextFunction) => {
    moduleLogger.debug(`${req.method} ${req.path}`)
    next()
  })

  // Readiness endpoint - used by Go server to check if Node.js server is ready
  app.get("/ready", (req: Request, res: Response) => {
    res.status(200).json({
      status: "ready",
      timestamp: new Date().toISOString(),
    })
  })

  // Execute code endpoint
  const executeHandler: RequestHandler = async (req, res, _next) => {
    try {
      const { input } = req.body as NodeRequest

      // Enhanced logging of the full request body
      moduleLogger.debug({ body: req.body }, "=== REQUEST RECEIVED ===")

      // Specifically log observed resources if present
      if (req.body.observed) {
        moduleLogger.debug("=== OBSERVED RESOURCES ===")

        // Log composite resource if present
        if (req.body.observed.composite) {
          moduleLogger.debug({ composite: req.body.observed.composite }, "Composite Resource:")
        }

        // Log individual resources if present
        if (req.body.observed.resources) {
          moduleLogger.info("Resources:")
          const resourceNames = Object.keys(req.body.observed.resources)
          moduleLogger.info(`Found ${resourceNames.length} resources: ${resourceNames.join(", ")}`)

          // Log each resource
          for (const [name, resource] of Object.entries(req.body.observed.resources)) {
            moduleLogger.info({ resource }, `Resource "${name}"`)
          }
        }
      }

      moduleLogger.info("=== EXECUTING CODE ===")
      moduleLogger.info(`Input length: ${JSON.stringify(input).length}`)

      const result = await executeCode(codeFilePath, input)

      moduleLogger.info("=== CODE EXECUTION COMPLETED ===")

      // Log the response for debugging
      moduleLogger.debug(`Execute response: ${JSON.stringify(result, null, 2)}`)

      res.json(result)
    } catch (err: unknown) {
      const error = err as Error
      moduleLogger.error(`Error executing code: ${error.message}`)

      res.status(500).json({
        error: {
          code: 500,
          message: error.message || "Unknown error",
          stack: error.stack,
        },
      })
    }
  }

  app.post("/execute", executeHandler)

  // Error handling middleware
  app.use(
    (err: Error | Record<string, unknown>, req: Request, res: Response, _next: NextFunction) => {
      moduleLogger.error(
        `Unhandled error: ${err instanceof Error ? err.message : JSON.stringify(err)}`
      )
      res.status(500).json({
        error: {
          code: 500,
          message: err instanceof Error ? err.message : "Internal server error",
          stack: err instanceof Error ? err.stack : undefined,
        },
      })
    }
  )

  // Start the server - bind to all interfaces (0.0.0.0) to ensure it's accessible
  const server = app.listen(port, "0.0.0.0", () => {
    moduleLogger.info(`Server listening on port ${port} on all interfaces`)
  })

  // Handle server errors
  server.on("error", (err: Error) => {
    moduleLogger.error(`Server error: ${err.message}`)
  })

  return server
}

/**
 * Gracefully shuts down the server
 * @param server The server to shut down
 * @returns A promise that resolves when the server is shut down
 */
export async function shutdownServer(server: ReturnType<typeof createServer>): Promise<void> {
  return new Promise((resolve, reject) => {
    moduleLogger.info("Shutting down server...")

    server.close(err => {
      if (err) {
        moduleLogger.error(`Error shutting down server: ${err.message}`)
        reject(err)
      } else {
        moduleLogger.info("Server shut down successfully")
        resolve()
      }
    })

    // Force close after timeout
    setTimeout(() => {
      moduleLogger.warn("Forcing server shutdown after timeout")
      resolve()
    }, 5000)
  })
}
