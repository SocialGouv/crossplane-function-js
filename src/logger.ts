import { hostname } from 'os';
import pino from 'pino';

// Create a Pino logger that writes to stderr with sync flush on exit
export const logger = pino({
  name: 'skyhook',
  level: process.env.LOG_LEVEL || 'info',
  transport: {
    target: 'pino/file',
    options: { 
      destination: process.stderr.fd,
      sync: true  // Enable sync mode for more reliable flushing
    },
  },
  formatters: {
    level: (label) => {
      return { level: label.toUpperCase() }
    },
    bindings (_bindings) {
      return {}
    },
  },
  timestamp: false,
});

// Export a function to create child loggers
export function createLogger(name: string) {
  return logger.child({ name });
}

// Function to flush logs - can be called before process exit
export function flushLogs(): Promise<void> {
  return new Promise((resolve) => {
    try {
      // Pino doesn't have a direct flush method, but we can use a final log
      // with the sync option to ensure everything is flushed
      logger.info('Flushing logs...');
      
      // Force a sync write to stderr
      if (process.stderr.write) {
        try {
          // Try to force a flush by writing directly to stderr
          process.stderr.write('', () => {
            // Give some time for the logs to be processed
            setTimeout(resolve, 1000);
          });
        } catch (err) {
          logger.error(`Error writing to stderr during flush: ${err}`);
          // Still resolve after a delay
          setTimeout(resolve, 1000);
        }
      } else {
        // If stderr.write is not available, just wait
        setTimeout(resolve, 1000);
      }
    } catch (err) {
      logger.error(`Error during log flush: ${err}`);
      // Still resolve after a delay
      setTimeout(resolve, 1000);
    }
  });
}

// Redirect console.log and console.error to use the logger
const originalConsoleLog = console.log;
const originalConsoleError = console.error;

console.log = function(...args) {
  if(args.length===1){
    logger.info(args[0]);
  } else {
    logger.info({context: args});
  }
};

console.error = function(...args) {
  if(args.length===1){
    logger.error(args[0]);
  } else {
    logger.error({context: args});
  }
};

// Export the logger as default
export default logger;
