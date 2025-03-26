import path from "path"

import { Command } from "commander"
import fs from "fs-extra"
import { createLogger } from "skyhook-libs"
import { parse } from "yaml"

import compo from "./compo/index.ts"
import { createServer, shutdownServer } from "./server.ts"
// import yarn from './yarn/index.ts';

// Create a logger for this module
const moduleLogger = createLogger("index")

// Default port for the HTTP server
const DEFAULT_PORT = 3000

// Process state
let isShuttingDown = false
let server: ReturnType<typeof createServer> | null = null

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
    // Shutdown the server if it's running
    if (server) {
      await shutdownServer(server)
    }

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

const skyhookRootPath =
  path.basename(__dirname) === "build" ? path.join(__dirname, "..") : __dirname

const main = async () => {
  const pkgFile = path.join(skyhookRootPath, "package.json")
  const pkgJSON = await fs.readFile(pkgFile, { encoding: "utf-8" })
  const pkg = JSON.parse(pkgJSON)

  program.name(pkg.name).description(pkg.description).version(pkg.version)

  // Server command (default)
  program
    .command("server", {
      isDefault: true,
    })
    .description("Start the HTTP server for executing code")
    .option("-c, --code-file-path <code-file-path>", "Path to the code file to execute")
    .option("-p, --port <number>", "Port to listen on", String(DEFAULT_PORT))
    .action(async options => {
      // Validate code file path
      if (!(await fs.exists(options.codeFilePath))) {
        moduleLogger.error(`Code file not found: ${options.codeFilePath}`)
        process.exit(1)
      }

      // Get the port from options or environment variable
      const port = parseInt(options.port || process.env.PORT || String(DEFAULT_PORT), 10)

      if (isNaN(port) || port < 1 || port > 65535) {
        moduleLogger.error(`Invalid port number: ${options.port}`)
        process.exit(1)
      }

      moduleLogger.info(`Code file path: ${options.codeFilePath}`)

      // Start the server
      server = createServer(port, options.codeFilePath)
      moduleLogger.info(`Node.js process started for code file: ${options.codeFilePath}`)
    })

  // Compo command for generating composition manifests
  program
    .command("compo")
    .description("Generate composition manifests from function directories")
    .action(async () => {
      try {
        await compo()
      } catch (err) {
        moduleLogger.error(`Error running compo command: ${err}`)
        process.exit(1)
      }
    })

  // Custom handler for the 'yarn' command
  const runYarn = async () => {
    try {
      // Read and parse .yarnrc.yml
      const yarnrcContent = await fs.readFile(".yarnrc.yml", { encoding: "utf-8" })
      const yarnrc = parse(yarnrcContent) as { yarnPath?: string }

      if (!yarnrc.yarnPath) {
        moduleLogger.error("No yarnPath found in .yarnrc.yml")
        process.exit(1)
      }

      const originalArgv = process.argv
      try {
        // Embedded yarn -- works but miss workspace focus command support as implemented actually
        // await yarn();

        // NODE-SEA require yarn release
        const yarnExecutable = `./${yarnrc.yarnPath}`
        const yarnIndex = process.argv.indexOf("yarn")
        const args = yarnIndex >= 0 ? process.argv.slice(yarnIndex + 1) : []
        process.argv = [process.execPath, yarnExecutable, ...args]
        // eslint-disable-next-line @typescript-eslint/no-require-imports
        const moduleRequire = require("module")
        const dynamicRequire = moduleRequire.createRequire(process.cwd())
        const possiblePaths = [path.join(`${process.cwd()}`, yarnExecutable)].filter(Boolean) // hack: feint the ncc compiler to keep dynamic require resolution
        const modulePath = possiblePaths.find(fs.existsSync)
        moduleLogger.info(`Running yarn from ${modulePath} with args: ${args.join(" ")}`)
        await dynamicRequire(modulePath)
      } finally {
        process.argv = originalArgv
      }
    } catch (err) {
      moduleLogger.error(`Error running yarn: ${err}`)
      process.exit(1)
    }
  }

  // Check if the first argument is 'yarn'
  if (process.argv.length > 2 && process.argv[2] === "yarn") {
    await runYarn()
  } else {
    // Parse command line arguments with Commander
    await program.parseAsync()
  }
}

main()

// Log memory usage periodically
// setInterval(() => {
//   if (isShuttingDown) return;

//   const memoryUsage = process.memoryUsage();
//   moduleLogger.debug(`Memory usage: RSS=${Math.round(memoryUsage.rss / 1024 / 1024)}MB, Heap=${Math.round(memoryUsage.heapUsed / 1024 / 1024)}/${Math.round(memoryUsage.heapTotal / 1024 / 1024)}MB`);
// }, 30000); // Every 30 seconds
