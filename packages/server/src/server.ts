import { createLogger } from "@crossplane-js/libs"
import express from "express"
import type { Request, Response, NextFunction, RequestHandler } from "express"

import { executeCode } from "./executor.ts"
import type { NodeRequest } from "./types.ts"

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
      moduleLogger.info({ input }, "CrossplaneFunctionBody")

      // Log request metadata without sensitive content
      moduleLogger.debug("=== REQUEST RECEIVED ===")
      moduleLogger.debug(`Request method: ${req.method}, path: ${req.path}`)
      moduleLogger.debug(`Input data size: ${JSON.stringify(input).length} bytes`)

      // Specifically log observed resources if present (without full content)
      if (req.body.observed) {
        moduleLogger.debug("=== OBSERVED RESOURCES ===")

        // Log composite resource metadata if present
        if (req.body.observed.composite) {
          const composite = req.body.observed.composite
          moduleLogger.debug(
            {
              compositeKind: composite.resource?.kind,
              compositeName: composite.resource?.metadata?.name,
              compositeNamespace: composite.resource?.metadata?.namespace,
            },
            "Composite Resource metadata"
          )
        }

        // Log individual resources metadata if present
        if (req.body.observed.resources) {
          const resourceNames = Object.keys(req.body.observed.resources)
          moduleLogger.info(`Found ${resourceNames.length} resources: ${resourceNames.join(", ")}`)

          // Log each resource metadata (not full content)
          for (const [name, resource] of Object.entries(req.body.observed.resources)) {
            const resourceObj = resource as any
            moduleLogger.info(
              {
                resourceName: name,
                kind: resourceObj.resource?.kind,
                name: resourceObj.resource?.metadata?.name,
                namespace: resourceObj.resource?.metadata?.namespace,
              },
              `Resource "${name}" metadata`
            )
          }
        }
      }

      moduleLogger.info("=== EXECUTING CODE ===")

      const result = await executeCode(codeFilePath, input)

      moduleLogger.info("=== CODE EXECUTION COMPLETED ===")

      // Log response metadata without full content
      if (result.error) {
        moduleLogger.debug(`Execute response: error - ${result.error.message}`)
      } else {
        moduleLogger.debug(
          `Execute response: success - result size: ${JSON.stringify(result.result || {}).length} bytes`
        )
      }

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

  // Start the server - bind to loopback by default for local parent process usage
  const bindAddr = process.env.BIND_ADDR || "127.0.0.1"
  const server = app.listen(port, bindAddr, () => {
    moduleLogger.info(`Server listening on ${bindAddr}:${port}`)
  })

  // Handle server errors - exit fast so parent (Go) can retry with a new port
  server.on("error", (err: Error & { code?: string }) => {
    moduleLogger.error(`Server error: ${err.message}${err?.code ? ` (code: ${err.code})` : ""}`)
    // Exit immediately on listen/bind errors (e.g., EADDRINUSE/EACCES) to avoid long waits on readiness
    process.exit(1)
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
