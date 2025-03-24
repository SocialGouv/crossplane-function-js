/**
 * Response from running code
 */
export interface NodeResponse {
  /**
   * The result of running the code, if successful
   */
  result?: any;
  
  /**
   * Error information if the code execution failed
   */
  error?: NodeError;
  
  /**
   * Captured console logs from the code execution
   */
  logs?: Array<any>;
}

/**
 * Error information
 */
export interface NodeError {
  /**
   * Error code
   */
  code: number;
  
  /**
   * Error message
   */
  message: string;
  
  /**
   * Stack trace if available
   */
  stack?: string;
}

/**
 * Request to run code
 */
export interface NodeRequest {
  /**
   * The code to run
   */
  code: string;
  
  /**
   * Dependencies to install before running the code
   */
  dependencies?: Record<string, string>;
  
  /**
   * The input data for the code
   */
  input: any;
}
