import pino from 'pino';

export const logger = pino(
  {
    name: 'skyhook',
    level: process.env.SKYHOOK_LOG_LEVEL || process.env.LOG_LEVEL || 'info',
    formatters: {
      level: (label: string) => {
        return { level: label.toUpperCase() }
      },
      bindings (_bindings: Record<string, unknown>) {
        return {}
      },
    },
    timestamp: false,
  },
  pino.destination({
    dest: process.stderr.fd,
    sync: true,
  }
));

// Export a function to create child loggers
export function createLogger(name: string) {
  return logger.child({ name });
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
