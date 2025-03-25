package node

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/socialgouv/crossplane-skyhook/pkg/hash"
	"github.com/socialgouv/crossplane-skyhook/pkg/logger"
	"github.com/socialgouv/crossplane-skyhook/pkg/types"
)

// ExecuteFunction executes a JavaScript/TypeScript function with the given input
func (pm *ProcessManager) ExecuteFunction(ctx context.Context, input *types.SkyhookInput, inputJSON string) (string, error) {
	// Maximum number of retries
	const maxRetries = 3

	// Initial retry delay (will be multiplied by 2^attempt for exponential backoff)
	retryDelay := 500 * time.Millisecond

	var lastErr error

	// Generate hash based on the entire input spec
	specBytes, err := json.Marshal(input.Spec)
	if err != nil {
		return "", fmt.Errorf("failed to marshal input spec: %w", err)
	}
	specHash := hash.GenerateInputHash(specBytes)

	// Extract resource information from the input JSON if available
	var resourceInfo *types.ResourceInfo
	var inputData map[string]interface{}
	if err := json.Unmarshal([]byte(inputJSON), &inputData); err == nil {
		resourceInfo = extractResourceInfo(inputData)
	}

	// Create a logger with spec hash and resource information
	execLogger := pm.logger.WithField(logger.FieldCodeHash, specHash[:8])
	if resourceInfo != nil {
		execLogger = logger.WithResource(execLogger, resourceInfo)
	}
	execLogger = execLogger.WithField(logger.FieldComponent, "node")
	execLogger = execLogger.WithField(logger.FieldOperation, "execute")

	// Try up to maxRetries times
	for attempt := 0; attempt <= maxRetries; attempt++ {
		// If this is a retry, log it
		if attempt > 0 {
			execLogger.WithFields(map[string]interface{}{
				logger.FieldRetryCount: attempt,
				logger.FieldError:      lastErr.Error(),
			}).Info("Retrying function execution")

			// Wait with exponential backoff before retrying
			backoffTime := retryDelay * time.Duration(1<<uint(attempt-1))
			time.Sleep(backoffTime)
		}

		// Get or create a process for this input
		process, err := pm.getOrCreateProcess(ctx, input)
		if err != nil {
			lastErr = fmt.Errorf("failed to get or create process: %w", err)
			continue // Retry
		}

		// Check if the process is healthy before proceeding
		if !pm.isProcessHealthy(process) {
			execLogger.Warn("Process appears unhealthy, restarting it")
			pm.restartProcess(process, specHash)
			lastErr = fmt.Errorf("process was unhealthy")
			continue // Retry with a new process
		}

		// Send the request to the process
		process.Lock.Lock()

		// Update the last used time
		process.LastUsed = time.Now()

		// Create a context with timeout for the operation
		execCtx, cancel := context.WithTimeout(ctx, pm.requestTimeout)

		// Execute the function
		execLogger.Debug("Sending request to Node.js server")
		result, err := process.Client.ExecuteFunction(execCtx, input.Spec.Source.Inline, input.Spec.Source.Dependencies, inputJSON)

		// Cleanup
		cancel() // Cancel the context

		if err != nil {
			process.Lock.Unlock()
			execLogger.WithField(logger.FieldError, err.Error()).Error("Error executing function")

			// If there's an error, we should restart the process
			pm.restartProcess(process, specHash)

			// Save the error for potential retry
			lastErr = err
			continue // Retry
		}

		// Success
		execLogger.Debug("Received result from Node.js server")
		process.Lock.Unlock()
		return result, nil
	}

	// If we've exhausted all retries, return a more detailed error
	if resourceInfo != nil {
		return "", fmt.Errorf("failed to execute function for resource %s/%s after %d attempts: %w",
			resourceInfo.Kind, resourceInfo.Name, maxRetries, lastErr)
	}

	return "", fmt.Errorf("all retry attempts failed for spec hash %s: %w", specHash[:8], lastErr)
}
