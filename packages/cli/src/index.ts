import path from "path"
import { fileURLToPath } from "url"

import { createLogger } from "@crossplane-js/libs"
import { Command } from "commander"
import fs from "fs-extra"

// Import commands
import compoCommand from "./commands/compo/index.ts"

// Create a logger for this module
const moduleLogger = createLogger("cli")

// Process state
let isShuttingDown = false

// Function to gracefully shutdown the process
async function gracefulShutdown(signal: string) {
  // Prevent multiple shutdown attempts
  if (isShuttingDown) {
    moduleLogger.info("Shutdown already in progress, ignoring additional signal")
    return
  }

  isShuttingDown = true
  moduleLogger.info(`Received ${signal}, starting graceful shutdown...`)

  // Set a timeout to force exit if graceful shutdown takes too long
  const forceExitTimeout = setTimeout(() => {
    moduleLogger.error("Forced exit due to timeout during graceful shutdown")
    process.exit(1)
  }, 5000) // 5 seconds timeout

  try {
    // Additional delay to ensure everything is written
    await new Promise(resolve => setTimeout(resolve, 1000))

    moduleLogger.info("Graceful shutdown complete")
    clearTimeout(forceExitTimeout)
    process.exit(0)
  } catch (err) {
    moduleLogger.error(`Error during graceful shutdown: ${err}`)
    process.exit(1)
  }
}

// Handle termination signals
process.on("SIGTERM", () => gracefulShutdown("SIGTERM"))
process.on("SIGINT", () => gracefulShutdown("SIGINT"))

// Set up error handling for uncaught exceptions
process.on("uncaughtException", err => {
  moduleLogger.error(`Uncaught exception: ${err.message}`)
  if (err.stack) {
    moduleLogger.debug(`Stack trace: ${err.stack}`)
  }
  // Don't exit the process, just log the error
})

// Set up error handling for unhandled promise rejections
process.on("unhandledRejection", (reason, _promise) => {
  moduleLogger.error(`Unhandled promise rejection: ${reason}`)
  // Don't exit the process, just log the error
})

// Create a new command instance
const program = new Command()

// Get the directory name from the import.meta.url
const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)
// In ES modules, we need to go up two levels to get to the cli package root
const xfuncjsRootPath = path.resolve(__dirname, "..")

const main = async () => {
  const pkgFile = path.join(xfuncjsRootPath, "package.json")
  const pkgJSON = await fs.readFile(pkgFile, { encoding: "utf-8" })
  const pkg = JSON.parse(pkgJSON)

  program.name("xfuncjs").description(pkg.description).version(pkg.version)

  // Register commands
  compoCommand(program)

  // Parse command line arguments with Commander
  await program.parseAsync()
}

main()
