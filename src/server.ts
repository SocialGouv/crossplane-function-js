import express from 'express';
import type { Request, Response, NextFunction, RequestHandler } from 'express';
import { createLogger } from './logger.ts';
import type { NodeRequest, NodeResponse } from './types.ts';
import { executeCode } from './executor.ts';

// Create a logger for this module
const moduleLogger = createLogger('server');

/**
 * Creates and configures an Express server
 * @param port The port to listen on
 * @returns The configured Express app
 */
export function createServer(port: number) {
  const app = express();
  
  // Configure middleware
  app.use(express.json({ limit: '10mb' }));
  
  // Add request logging
  app.use((req: Request, res: Response, next: NextFunction) => {
    moduleLogger.debug(`${req.method} ${req.path}`);
    next();
  });
  
  // Healthcheck endpoint
  app.get('/healthcheck', (req: Request, res: Response) => {
    res.status(200).json({ 
      status: 'ok',
      timestamp: new Date().toISOString()
    });
  });
  
  // Readiness endpoint - used by Go server to check if Node.js server is ready
  app.get('/ready', (req: Request, res: Response) => {
    res.status(200).json({ 
      status: 'ready',
      timestamp: new Date().toISOString()
    });
  });
  
  // Execute code endpoint
  const executeHandler: RequestHandler = async (req, res) => {
    try {
      const { code, input } = req.body as NodeRequest;
      
      // Log the request for debugging
      moduleLogger.debug(`Execute request received: ${JSON.stringify({ 
        code_length: code?.length || 0,
        input: input 
      }, null, 2)}`);
      
      if (!code) {
        return res.status(400).json({
          error: {
            code: 400,
            message: 'Code is required'
          }
        });
      }
      
      moduleLogger.info(`Executing code with input length: ${JSON.stringify(input).length}`);

      const result = await executeCode(code, input);
      
      moduleLogger.info('Code execution completed');
      
      // Log the response for debugging
      moduleLogger.debug(`Execute response: ${JSON.stringify(result, null, 2)}`);
      
      return res.json(result);
    } catch (err: unknown) {
      const error = err as Error;
      moduleLogger.error(`Error executing code: ${error.message}`);
      
      return res.status(500).json({
        error: {
          code: 500,
          message: error.message || 'Unknown error',
          stack: error.stack
        }
      });
    }
  };
  
  app.post('/execute', executeHandler);
  
  // Error handling middleware
  app.use((err: any, req: Request, res: Response, next: NextFunction) => {
    moduleLogger.error(`Unhandled error: ${err.message}`);
    res.status(500).json({
      error: {
        code: 500,
        message: err.message || 'Internal server error',
        stack: err.stack
      }
    });
  });
  
  // Start the server - bind to all interfaces (0.0.0.0) to ensure it's accessible
  const server = app.listen(port, '0.0.0.0', () => {
    moduleLogger.info(`Server listening on port ${port} on all interfaces`);
  });
  
  // Handle server errors
  server.on('error', (err: Error) => {
    moduleLogger.error(`Server error: ${err.message}`);
  });
  
  return server;
}

/**
 * Gracefully shuts down the server
 * @param server The server to shut down
 * @returns A promise that resolves when the server is shut down
 */
export async function shutdownServer(server: ReturnType<typeof createServer>): Promise<void> {
  return new Promise((resolve, reject) => {
    moduleLogger.info('Shutting down server...');
    
    server.close((err) => {
      if (err) {
        moduleLogger.error(`Error shutting down server: ${err.message}`);
        reject(err);
      } else {
        moduleLogger.info('Server shut down successfully');
        resolve();
      }
    });
    
    // Force close after timeout
    setTimeout(() => {
      moduleLogger.warn('Forcing server shutdown after timeout');
      resolve();
    }, 5000);
  });
}
