import { createServer, shutdownServer } from './server.ts';
import { createLogger, flushLogs } from './logger.ts';

// Create a logger for this module
const moduleLogger = createLogger('index');

// Default port for the HTTP server
const DEFAULT_PORT = 3000;

// Get the port from environment variable or use default
const port = parseInt(process.env.PORT || `${DEFAULT_PORT}`, 10);

if (process.argv.length < 3) {
  moduleLogger.error('Usage: node index.js <code-file-path>');
  process.exit(1);
}

// Get the code file path
const codeFilePath = process.argv[2];
moduleLogger.info(`Code file path: ${codeFilePath}`);

// Set up error handling for uncaught exceptions
process.on('uncaughtException', (err) => {
  moduleLogger.error(`Uncaught exception: ${err.message}`);
  if (err.stack) {
    moduleLogger.debug(`Stack trace: ${err.stack}`);
  }
  // Don't exit the process, just log the error
});

// Set up error handling for unhandled promise rejections
process.on('unhandledRejection', (reason, promise) => {
  moduleLogger.error(`Unhandled promise rejection: ${reason}`);
  // Don't exit the process, just log the error
});

// Process state
let isShuttingDown = false;

// Function to gracefully shutdown the process
async function gracefulShutdown(signal: string) {
  // Prevent multiple shutdown attempts
  if (isShuttingDown) {
    moduleLogger.info('Shutdown already in progress, ignoring additional signal');
    return;
  }
  
  isShuttingDown = true;
  moduleLogger.info(`Received ${signal}, starting graceful shutdown...`);
  
  // Set a timeout to force exit if graceful shutdown takes too long
  const forceExitTimeout = setTimeout(() => {
    moduleLogger.error('Forced exit due to timeout during graceful shutdown');
    process.exit(1);
  }, 5000); // 5 seconds timeout
  
  try {
    // Shutdown the server if it's running
    if (server) {
      await shutdownServer(server);
    }
    
    // Explicitly flush logs
    moduleLogger.info('Explicitly flushing logs before shutdown...');
    await flushLogs();
    
    // Additional delay to ensure everything is written
    await new Promise(resolve => setTimeout(resolve, 1000));
    
    moduleLogger.info('Graceful shutdown complete');
    clearTimeout(forceExitTimeout);
    process.exit(0);
  } catch (err) {
    moduleLogger.error(`Error during graceful shutdown: ${err}`);
    process.exit(1);
  }
}

// Handle termination signals
process.on('SIGTERM', () => gracefulShutdown('SIGTERM'));
process.on('SIGINT', () => gracefulShutdown('SIGINT'));

// Log memory usage periodically
setInterval(() => {
  if (isShuttingDown) return;
  
  const memoryUsage = process.memoryUsage();
  moduleLogger.debug(`Memory usage: RSS=${Math.round(memoryUsage.rss / 1024 / 1024)}MB, Heap=${Math.round(memoryUsage.heapUsed / 1024 / 1024)}/${Math.round(memoryUsage.heapTotal / 1024 / 1024)}MB`);
}, 30000); // Every 30 seconds

const server = createServer(port, codeFilePath);

moduleLogger.info(`Node.js process started for code file: ${codeFilePath}`);
