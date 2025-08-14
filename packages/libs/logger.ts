import pino from "pino"

export const logger = pino({
  name: "xfuncjs",
  level: process.env.XFUNCJS_LOG_LEVEL || process.env.LOG_LEVEL || "info",
  formatters: {
    level: (label: string) => {
      return { level: label.toUpperCase() }
    },
    bindings(_bindings: Record<string, unknown>) {
      return {}
    },
  },
  timestamp: false,
})

// Export a function to create child loggers
export function createLogger(name: string) {
  return logger.child({ name })
}

// Redirect console.log and console.error to use the logger
const originalConsoleLog = console.log
const originalConsoleError = console.error

// Store original functions to allow restoring them if needed
console.log = function (...args) {
  if (args.length === 1) {
    logger.info(args[0])
  } else {
    logger.info({ context: args })
  }
}

console.error = function (...args) {
  if (args.length === 1) {
    logger.error(args[0])
  } else {
    logger.error({ context: args })
  }
}

// Export original console functions for potential restoration
export const restoreConsole = () => {
  console.log = originalConsoleLog
  console.error = originalConsoleError
}

// Export the logger as default
export default logger
