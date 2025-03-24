package node

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/socialgouv/crossplane-skyhook/pkg/hash"
	"github.com/socialgouv/crossplane-skyhook/pkg/logger"
	"github.com/socialgouv/crossplane-skyhook/pkg/types"
	"sigs.k8s.io/yaml"
)

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		return fmt.Errorf("failed to copy file contents: %w", err)
	}

	// Preserve file permissions
	srcInfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("failed to get source file info: %w", err)
	}
	return os.Chmod(dst, srcInfo.Mode())
}

// copyDir recursively copies a directory from src to dst, with optional file exclusions
func copyDir(src, dst string, excludeFiles ...string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("failed to get source directory info: %w", err)
	}

	if !srcInfo.IsDir() {
		return fmt.Errorf("source is not a directory")
	}

	// Create destination directory
	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return fmt.Errorf("failed to read source directory: %w", err)
	}

	for _, entry := range entries {
		// Check if the file should be excluded
		shouldExclude := false
		for _, excludeFile := range excludeFiles {
			if entry.Name() == excludeFile {
				shouldExclude = true
				break
			}
		}
		if shouldExclude {
			continue
		}

		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := copyDir(srcPath, dstPath, excludeFiles...); err != nil {
				return err
			}
		} else {
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}

	return nil
}

// ProcessInfo holds information about a Node.js process
type ProcessInfo struct {
	Process     *exec.Cmd
	Client      *NodeClient
	LastUsed    time.Time
	Lock        sync.Mutex
	Port        int    // Store the assigned port for this process
	TempDirPath string // Path to the temporary directory for this process
}

// ProcessManager manages Node.js processes
type ProcessManager struct {
	processes           map[string]*ProcessInfo
	lock                sync.RWMutex
	gcInterval          time.Duration
	idleTimeout         time.Duration
	tempDir             string
	logger              logger.Logger
	nodeServerPort      int        // Base port for Node.js servers
	nextPort            int        // Next port to assign
	portMutex           sync.Mutex // Mutex for port assignment
	healthCheckWait     time.Duration
	healthCheckInterval time.Duration
	requestTimeout      time.Duration
}

// NewProcessManager creates a new process manager
func NewProcessManager(gcInterval, idleTimeout time.Duration, tempDir string, logger logger.Logger, opts ...ProcessManagerOption) (*ProcessManager, error) {
	// Create temp directory if it doesn't exist
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}

	pm := &ProcessManager{
		processes:           make(map[string]*ProcessInfo),
		gcInterval:          gcInterval,
		idleTimeout:         idleTimeout,
		tempDir:             tempDir,
		logger:              logger,
		nodeServerPort:      3000,                   // Default port
		nextPort:            3000,                   // Initialize next port to the default port
		healthCheckWait:     30 * time.Second,       // Default timeout for health check
		healthCheckInterval: 500 * time.Millisecond, // Default interval for health check polling
		requestTimeout:      30 * time.Second,       // Default timeout for requests
	}

	// Apply options
	for _, opt := range opts {
		opt(pm)
	}

	// Start the garbage collector
	pm.startGarbageCollector()

	return pm, nil
}

// ProcessManagerOption is a function that configures a ProcessManager
type ProcessManagerOption func(*ProcessManager)

// WithNodeServerPort sets the base port for the Node.js HTTP servers
func WithNodeServerPort(port int) ProcessManagerOption {
	return func(pm *ProcessManager) {
		pm.nodeServerPort = port
		pm.nextPort = port // Also update the next port to be assigned
	}
}

// WithHealthCheckWait sets the timeout for health check
func WithHealthCheckWait(timeout time.Duration) ProcessManagerOption {
	return func(pm *ProcessManager) {
		pm.healthCheckWait = timeout
	}
}

// WithHealthCheckInterval sets the interval for health check polling
func WithHealthCheckInterval(interval time.Duration) ProcessManagerOption {
	return func(pm *ProcessManager) {
		pm.healthCheckInterval = interval
	}
}

// WithRequestTimeout sets the timeout for requests
func WithRequestTimeout(timeout time.Duration) ProcessManagerOption {
	return func(pm *ProcessManager) {
		pm.requestTimeout = timeout
	}
}

// SetNodeServerPort sets the base port for the Node.js HTTP servers
func (pm *ProcessManager) SetNodeServerPort(port int) {
	pm.nodeServerPort = port
	pm.nextPort = port // Also update the next port to be assigned
}

// SetHealthCheckWait sets the timeout for health check
func (pm *ProcessManager) SetHealthCheckWait(timeout time.Duration) {
	pm.healthCheckWait = timeout
}

// SetHealthCheckInterval sets the interval for health check polling
func (pm *ProcessManager) SetHealthCheckInterval(interval time.Duration) {
	pm.healthCheckInterval = interval
}

// SetRequestTimeout sets the timeout for requests
func (pm *ProcessManager) SetRequestTimeout(timeout time.Duration) {
	pm.requestTimeout = timeout
}

// getNextPort returns the next available port and increments the counter
func (pm *ProcessManager) getNextPort() int {
	pm.portMutex.Lock()
	defer pm.portMutex.Unlock()

	port := pm.nextPort
	pm.nextPort++

	// If we've gone too high, wrap around to the base port + 1000
	// This gives a range of 1000 ports (e.g., 3000-3999)
	if pm.nextPort >= pm.nodeServerPort+1000 {
		pm.nextPort = pm.nodeServerPort
	}

	return port
}

// startGarbageCollector starts a goroutine that periodically collects garbage
func (pm *ProcessManager) startGarbageCollector() {
	ticker := time.NewTicker(pm.gcInterval)
	go func() {
		for {
			<-ticker.C
			pm.collectGarbage()
		}
	}()
}

// collectGarbage terminates processes that have been idle for too long
func (pm *ProcessManager) collectGarbage() {
	now := time.Now()

	// Create a logger for garbage collection
	gcLogger := pm.logger.WithField(logger.FieldComponent, "node")
	gcLogger = gcLogger.WithField(logger.FieldOperation, "garbage-collection")

	gcLogger.Debug("Starting garbage collection")

	pm.lock.Lock()
	defer pm.lock.Unlock()

	for id, info := range pm.processes {
		info.Lock.Lock()

		// Create a process-specific logger
		processLogger := gcLogger.WithField(logger.FieldCodeHash, id[:8])
		if info.Process.Process != nil {
			processLogger = processLogger.WithField(logger.FieldPID, info.Process.Process.Pid)
		}
		processLogger = processLogger.WithField(logger.FieldPort, info.Port)

		idleTime := now.Sub(info.LastUsed)
		processLogger = processLogger.WithField("idle_time_seconds", int(idleTime.Seconds()))

		if idleTime > pm.idleTimeout {
			processLogger.Info("Terminating idle process")

			// Send SIGTERM to signal the process to exit gracefully
			if info.Process.Process != nil {
				processLogger.Debug("Sending SIGTERM to process")
				info.Process.Process.Signal(syscall.SIGTERM)
			}

			// Flush any buffered stderr data
			if stderrWriter, ok := info.Process.Stderr.(*logWriter); ok {
				stderrWriter.Flush()
				processLogger.Debug("Flushed stderr buffer for process")
			}

			// Kill the process if it doesn't exit gracefully
			if info.Process.Process != nil {
				processLogger.Debug("Sending SIGKILL to process")
				info.Process.Process.Kill()
			}

			// Clean up the temporary directory if it exists
			if info.TempDirPath != "" {
				processLogger.Info("Removing temporary directory for idle process")
				if err := os.RemoveAll(info.TempDirPath); err != nil {
					processLogger.WithField(logger.FieldError, err.Error()).
						Warn("Failed to remove temporary directory for idle process")
				}
			}

			delete(pm.processes, id)
			processLogger.Info("Process successfully terminated")
		} else {
			processLogger.Debug("Process still active, skipping")
		}

		info.Lock.Unlock()
	}

	gcLogger.WithField("active_processes", len(pm.processes)).
		Debug("Garbage collection completed")
}

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

// extractResourceInfo extracts resource information from the input data
func extractResourceInfo(data map[string]interface{}) *types.ResourceInfo {
	resourceInfo := &types.ResourceInfo{}

	// Check for Crossplane composite resource in the input
	if input, ok := data["input"].(map[string]interface{}); ok {
		// Try to extract from apiVersion and kind
		if apiVersion, ok := input["apiVersion"].(string); ok {
			resourceInfo.Version = apiVersion
		}

		if kind, ok := input["kind"].(string); ok {
			resourceInfo.Kind = kind
		}

		// Try to extract from metadata
		if metadata, ok := input["metadata"].(map[string]interface{}); ok {
			if name, ok := metadata["name"].(string); ok {
				resourceInfo.Name = name
			}

			if namespace, ok := metadata["namespace"].(string); ok {
				resourceInfo.Namespace = namespace
			}
		}
	}

	// Check for observed resources
	if observed, ok := data["observed"].(map[string]interface{}); ok {
		if composite, ok := observed["composite"].(map[string]interface{}); ok {
			if resource, ok := composite["resource"].(map[string]interface{}); ok {
				// Try to extract from apiVersion and kind
				if apiVersion, ok := resource["apiVersion"].(string); ok && resourceInfo.Version == "" {
					resourceInfo.Version = apiVersion
				}

				if kind, ok := resource["kind"].(string); ok && resourceInfo.Kind == "" {
					resourceInfo.Kind = kind
				}

				// Try to extract from metadata
				if metadata, ok := resource["metadata"].(map[string]interface{}); ok {
					if name, ok := metadata["name"].(string); ok && resourceInfo.Name == "" {
						resourceInfo.Name = name
					}

					if namespace, ok := metadata["namespace"].(string); ok && resourceInfo.Namespace == "" {
						resourceInfo.Namespace = namespace
					}
				}
			}
		}
	}

	// If we couldn't extract any resource information, return nil
	if resourceInfo.Version == "" && resourceInfo.Kind == "" && resourceInfo.Name == "" && resourceInfo.Namespace == "" {
		return nil
	}

	return resourceInfo
}

// isProcessHealthy checks if a process is healthy and ready to receive input
func (pm *ProcessManager) isProcessHealthy(process *ProcessInfo) bool {
	// Create a logger with process information
	healthLogger := pm.logger.WithField(logger.FieldComponent, "node")
	healthLogger = healthLogger.WithField(logger.FieldOperation, "health-check")

	if process.Process != nil && process.Process.Process != nil {
		healthLogger = healthLogger.WithField(logger.FieldPID, process.Process.Process.Pid)
	}

	healthLogger = healthLogger.WithField(logger.FieldPort, process.Port)

	// Check if the process is still running
	if process.Process.Process == nil {
		healthLogger.Warn("Process is not healthy: process is nil")
		return false
	}

	// Check if the client is valid
	if process.Client == nil {
		healthLogger.Warn("Process is not healthy: client is nil")
		return false
	}

	// Check if the process is still running and responsive
	exitCode := process.Process.ProcessState
	if exitCode != nil {
		healthLogger.WithField("exit_code", exitCode.String()).
			Warn("Process is not healthy: process has exited")
		return false
	}

	healthLogger.Debug("Process is healthy")
	return true
}

// restartProcess marks a process for restart by removing it from the processes map
func (pm *ProcessManager) restartProcess(process *ProcessInfo, hash string) {
	// Create a logger with hash information
	restartLogger := pm.logger.WithField(logger.FieldCodeHash, hash[:8])
	restartLogger = restartLogger.WithField(logger.FieldComponent, "node")
	restartLogger = restartLogger.WithField(logger.FieldOperation, "process-restart")

	// Add PID to logger if available
	if process.Process.Process != nil {
		restartLogger = restartLogger.WithField(logger.FieldPID, process.Process.Process.Pid)
	}

	// Add port to logger
	restartLogger = restartLogger.WithField(logger.FieldPort, process.Port)

	pm.lock.Lock()
	defer pm.lock.Unlock()

	// Check if this process is still in the map
	if existingProcess, exists := pm.processes[hash]; exists && existingProcess == process {
		restartLogger.Info("Marking process for restart")

		// Send SIGTERM to signal the process to exit gracefully
		if process.Process.Process != nil {
			restartLogger.Debug("Sending SIGTERM to process")
			process.Process.Process.Signal(syscall.SIGTERM)
		}

		// Flush any buffered stderr data
		if stderrWriter, ok := process.Process.Stderr.(*logWriter); ok {
			stderrWriter.Flush()
			restartLogger.Debug("Flushed stderr buffer for process")
		}

		// Kill the process if it doesn't exit gracefully
		if process.Process.Process != nil {
			restartLogger.Debug("Sending SIGKILL to process")
			process.Process.Process.Kill()
		}

		// Clean up the temporary directory if it exists
		if process.TempDirPath != "" {
			restartLogger.Info("Removing temporary directory for restarted process")
			if err := os.RemoveAll(process.TempDirPath); err != nil {
				restartLogger.WithField(logger.FieldError, err.Error()).
					Warn("Failed to remove temporary directory for restarted process")
			}
		}

		delete(pm.processes, hash)
		restartLogger.Info("Process successfully marked for restart")
	} else {
		restartLogger.Debug("Process not found in map, no restart needed")
	}
}

// getOrCreateProcess gets an existing process for the given input or creates a new one
func (pm *ProcessManager) getOrCreateProcess(ctx context.Context, input *types.SkyhookInput) (*ProcessInfo, error) {
	// Generate hash based on the entire input spec
	specBytes, err := json.Marshal(input.Spec)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal input spec: %w", err)
	}
	specHash := hash.GenerateInputHash(specBytes)

	// Create a logger with spec hash information
	procLogger := pm.logger.WithField(logger.FieldCodeHash, specHash[:8])
	procLogger = procLogger.WithField(logger.FieldComponent, "node")
	procLogger = procLogger.WithField(logger.FieldOperation, "process-create")

	// Check if we already have a process for this input
	pm.lock.RLock()
	process, exists := pm.processes[specHash]
	pm.lock.RUnlock()

	if exists {
		// Verify the process is still healthy before returning it
		if !pm.isProcessHealthy(process) {
			procLogger.Warn("Existing process is unhealthy, creating a new one")
			pm.restartProcess(process, specHash)
			exists = false
		} else {
			procLogger.Debug("Reusing existing process")
			return process, nil
		}
	}

	// Create a new process
	pm.lock.Lock()
	defer pm.lock.Unlock()

	// Check again in case another goroutine created the process while we were waiting
	if process, exists = pm.processes[specHash]; exists {
		if pm.isProcessHealthy(process) {
			procLogger.Debug("Reusing existing process (created by another goroutine)")
			return process, nil
		}
		// Process exists but is unhealthy, remove it
		procLogger.Warn("Existing process is unhealthy, removing it")
		delete(pm.processes, specHash)
	}

	// Create a unique directory for this input
	extension := ".ts"
	tempFilename := hash.GenerateTempFilename(input.Spec.Source.Inline, extension)
	uniqueDirName := specHash[:16] // Use first 16 chars of hash
	uniqueDirPath := filepath.Join(pm.tempDir, uniqueDirName)

	// Create the unique directory
	if err := os.MkdirAll(uniqueDirPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create unique directory %s: %w", uniqueDirPath, err)
	}

	// Place the temporary file in the unique directory
	tempFilePath := filepath.Join(uniqueDirPath, tempFilename)
	procLogger = procLogger.WithField("temp_file", tempFilePath)

	// Write the code to the temporary file
	if err := os.WriteFile(tempFilePath, []byte(input.Spec.Source.Inline), 0644); err != nil {
		return nil, fmt.Errorf("failed to write code to temporary file %s: %w", tempFilePath, err)
	}

	// Create a symlink for node_modules only if there are no dependencies
	// When dependencies are specified, we'll use yarn portal instead
	if len(input.Spec.Source.Dependencies) == 0 {
		nodeModulesPath := filepath.Join(uniqueDirPath, "node_modules")
		if err := os.Symlink("/app/node_modules", nodeModulesPath); err != nil {
			// Clean up the temporary directory
			if uniqueDirPath != "" {
				procLogger.Info("Removing temporary directory after symlink creation failure")
				if cleanupErr := os.RemoveAll(uniqueDirPath); cleanupErr != nil {
					procLogger.WithField(logger.FieldError, cleanupErr.Error()).
						Warn("Failed to remove temporary directory after symlink creation failure")
				}
			}
			return nil, fmt.Errorf("failed to create node_modules symlink in %s: %w", uniqueDirPath, err)
		}
		procLogger.Info("Copied node_modules directory to temporary directory")
	}

	// Create a package.json file if dependencies are specified
	if len(input.Spec.Source.Dependencies) > 0 {

		// Create packages directory and copy each folder from /app/packages
		packagesDir := filepath.Join(uniqueDirPath, "packages")
		if err := os.MkdirAll(packagesDir, 0755); err != nil {
			procLogger.WithField(logger.FieldError, err.Error()).
				Warn("Failed to create packages directory in temporary directory")
		} else {
			// List all folders in /app/packages
			appPackagesEntries, err := os.ReadDir("/app/packages")
			if err != nil {
				procLogger.WithField(logger.FieldError, err.Error()).
					Warn("Failed to read /app/packages directory")
			} else {
				// Copy each folder
				for _, entry := range appPackagesEntries {
					if entry.IsDir() {
						srcPath := filepath.Join("/app/packages", entry.Name())
						dstPath := filepath.Join(packagesDir, entry.Name())

						if err := copyDir(srcPath, dstPath); err != nil {
							procLogger.WithField(logger.FieldError, err.Error()).
								WithField("package", entry.Name()).
								Warn("Failed to copy package directory")
						} else {
							procLogger.WithField("package", entry.Name()).
								Info("Copied package directory")
						}
					}
				}

				// Create a minimal copy of /app for crossplane-skyhook
				// We only need to copy essential files, not the entire /app directory
				appDstPath := filepath.Join(packagesDir, "crossplane-skyhook")
				if err := os.MkdirAll(appDstPath, 0755); err != nil {
					procLogger.WithField(logger.FieldError, err.Error()).
						Warn("Failed to create crossplane-skyhook directory")
				} else {
					// Copy package.json and other essential files
					essentialFiles := []string{"package.json", "tsconfig.json"}
					for _, file := range essentialFiles {
						srcFilePath := filepath.Join("/app", file)
						dstFilePath := filepath.Join(appDstPath, file)
						if err := copyFile(srcFilePath, dstFilePath); err != nil {
							procLogger.WithField(logger.FieldError, err.Error()).
								WithField("file", file).
								Warn("Failed to copy essential file to crossplane-skyhook directory")
						}
					}
					procLogger.Info("Created crossplane-skyhook directory with essential files")
				}
			}
		}

		dependencies := make(map[string]interface{})

		dependencies["skyhook-sdk"] = "workspace:^"

		// Create a copy of the dependencies map from the input
		for k, v := range input.Spec.Source.Dependencies {
			dependencies[k] = v
		}

		// Read dependencies from skyhook-sdk package.json and merge them if not already defined
		// skyhookSDKPackageJSONPath := filepath.Join(packagesDir, "skyhook-sdk", "package.json")
		// if _, err := os.Stat(skyhookSDKPackageJSONPath); err == nil {
		// 	// File exists, read it
		// 	skyhookSDKPackageJSONBytes, err := os.ReadFile(skyhookSDKPackageJSONPath)
		// 	if err == nil {
		// 		var skyhookSDKPackageJSON map[string]interface{}
		// 		if err := json.Unmarshal(skyhookSDKPackageJSONBytes, &skyhookSDKPackageJSON); err == nil {
		// 			// Successfully parsed package.json
		// 			if sdkDeps, ok := skyhookSDKPackageJSON["dependencies"].(map[string]interface{}); ok {
		// 				// Merge dependencies that are not already defined
		// 				for depName, depVersion := range sdkDeps {
		// 					if _, exists := dependencies[depName]; !exists {
		// 						dependencies[depName] = depVersion
		// 						procLogger.WithField("dependency", depName).
		// 							WithField("version", depVersion).
		// 							Info("Added dependency from skyhook-sdk package.json")
		// 					}
		// 				}
		// 			}
		// 		} else {
		// 			procLogger.WithField(logger.FieldError, err.Error()).
		// 				Warn("Failed to parse skyhook-sdk package.json")
		// 		}
		// 	} else {
		// 		procLogger.WithField(logger.FieldError, err.Error()).
		// 			Warn("Failed to read skyhook-sdk package.json")
		// 	}
		// } else {
		// 	procLogger.WithField(logger.FieldError, err.Error()).
		// 		Warn("skyhook-sdk package.json not found")
		// }

		workspaces := make(map[string]interface{})
		workspaces["packages"] = []string{"packages/*"}

		packageJSON := map[string]interface{}{
			"name":         "skyhook-function",
			"version":      "0.0.0",
			"private":      true,
			"dependencies": dependencies,
			"workspaces":   workspaces,
		}
		packageJSONBytes, err := json.MarshalIndent(packageJSON, "", "  ")
		if err != nil {
			// Clean up the temporary directory
			if uniqueDirPath != "" {
				procLogger.Info("Removing temporary directory after package.json creation failure")
				if cleanupErr := os.RemoveAll(uniqueDirPath); cleanupErr != nil {
					procLogger.WithField(logger.FieldError, cleanupErr.Error()).
						Warn("Failed to remove temporary directory after package.json creation failure")
				}
			}
			return nil, fmt.Errorf("failed to marshal package.json: %w", err)
		}
		packageJSONPath := filepath.Join(uniqueDirPath, "package.json")
		if err := os.WriteFile(packageJSONPath, packageJSONBytes, 0644); err != nil {
			// Clean up the temporary directory
			if uniqueDirPath != "" {
				procLogger.Info("Removing temporary directory after package.json write failure")
				if cleanupErr := os.RemoveAll(uniqueDirPath); cleanupErr != nil {
					procLogger.WithField(logger.FieldError, cleanupErr.Error()).
						Warn("Failed to remove temporary directory after package.json write failure")
				}
			}
			return nil, fmt.Errorf("failed to write package.json to %s: %w", packageJSONPath, err)
		}
		procLogger.Info("Created package.json with dependencies")

		// If yarn.lock is provided, write it to the temporary directory
		if input.Spec.Source.YarnLock != "" {
			yarnLockPath := filepath.Join(uniqueDirPath, "yarn.lock")
			if err := os.WriteFile(yarnLockPath, []byte(input.Spec.Source.YarnLock), 0644); err != nil {
				procLogger.WithField(logger.FieldError, err.Error()).
					Warn("Failed to write yarn.lock to temporary directory")
			} else {
				procLogger.Info("Created yarn.lock in temporary directory")
			}
		}

		// Read the original .yarnrc.yml, extract yarnPath, and remove the plugins section
		yarnrcSrc := "/app/.yarnrc.yml"
		yarnrcContent, err := os.ReadFile(yarnrcSrc)
		var yarnPath string

		// Parse the YAML content
		var yarnConfig map[string]interface{}
		if err := yaml.Unmarshal(yarnrcContent, &yarnConfig); err != nil {
			procLogger.WithField(logger.FieldError, err.Error()).
				Warn("Failed to parse .yarnrc.yml")
		}

		// Extract yarnPath from the config
		if path, ok := yarnConfig["yarnPath"].(string); ok {
			yarnPath = path
			procLogger.WithField("yarnPath", yarnPath).Info("Found yarnPath in .yarnrc.yml")
		}

		// Remove the plugins section
		delete(yarnConfig, "plugins")

		// Marshal back to YAML
		modifiedYarnrc, err := yaml.Marshal(yarnConfig)
		if err != nil {
			procLogger.WithField(logger.FieldError, err.Error()).
				Warn("Failed to marshal modified .yarnrc.yml")
		} else {
			// Write the modified .yarnrc.yml to the temporary directory
			yarnrcDst := filepath.Join(uniqueDirPath, ".yarnrc.yml")
			if err := os.WriteFile(yarnrcDst, modifiedYarnrc, 0644); err != nil {
				procLogger.WithField(logger.FieldError, err.Error()).
					Warn("Failed to write modified .yarnrc.yml to temporary directory")
			} else {
				procLogger.Info("Created modified .yarnrc.yml in temporary directory (plugins removed)")
			}
		}

		// Create .yarn directory in the temp directory and copy contents from /app/.yarn
		// Exclude install-state.gz which is environment-specific
		yarnSrcDir := "/app/.yarn"
		yarnDstDir := filepath.Join(uniqueDirPath, ".yarn")
		if err := os.MkdirAll(yarnDstDir, 0755); err != nil {
			procLogger.WithField(logger.FieldError, err.Error()).
				Warn("Failed to create .yarn directory in temporary directory")
		} else if err := copyDir(yarnSrcDir, yarnDstDir, "install-state.gz"); err != nil {
			procLogger.WithField(logger.FieldError, err.Error()).
				Warn("Failed to copy .yarn directory to temporary directory")
		} else {
			procLogger.Info("Copied .yarn directory to temporary directory")
		}

		// Construct the absolute path to the yarn executable
		yarnExecPath := filepath.Join(uniqueDirPath, yarnPath)
		procLogger.WithField("yarnExecPath", yarnExecPath).Info("Using yarn executable")

		// Run yarn install using the extracted yarn executable
		procLogger.Info("Running yarn install")
		// yarnCmd := exec.Command(yarnExecPath)
		yarnCmd := exec.Command("/app/crossplane-skyhook", "yarn", "workspaces", "focus", "--production")
		yarnCmd.Dir = uniqueDirPath // Set the working directory

		// Create a custom logWriter for yarn output
		yarnStdoutWriter := &logWriter{
			logger: procLogger.WithField("yarn", "stdout"),
			prefix: fmt.Sprintf("yarn[%s]: ", specHash[:8]),
		}
		yarnStderrWriter := &logWriter{
			logger: procLogger.WithField("yarn", "stderr"),
			prefix: fmt.Sprintf("yarn[%s]: ", specHash[:8]),
		}

		yarnCmd.Stdout = yarnStdoutWriter
		yarnCmd.Stderr = yarnStderrWriter

		// Run yarn install
		if err := yarnCmd.Run(); err != nil {
			procLogger.WithField(logger.FieldError, err.Error()).
				Warn("Yarn install failed, but continuing anyway")
		} else {
			procLogger.Info("Yarn install completed successfully")
		}
	}

	port := pm.getNextPort()
	procLogger = procLogger.WithField(logger.FieldPort, port)

	// Create the Node.js process with the appropriate path to the index file
	// Use a background context that won't be canceled when the request is done
	processCtx := context.Background()

	// NODE-SEA
	cmd := exec.CommandContext(processCtx, "/app/crossplane-skyhook", "-c", tempFilePath)

	cmd.Env = append(os.Environ(),
		fmt.Sprintf("PORT=%d", port),
	)

	// Create a custom logWriter
	stderrWriter := &logWriter{
		logger: procLogger,
		prefix: fmt.Sprintf("node[%s]: ", specHash[:8]),
	}

	// Redirect stderr to our logger
	cmd.Stderr = stderrWriter

	// Start the process
	if err := cmd.Start(); err != nil {
		// Clean up the temporary directory
		if uniqueDirPath != "" {
			procLogger.Info("Removing temporary directory for failed process start")
			if cleanupErr := os.RemoveAll(uniqueDirPath); cleanupErr != nil {
				procLogger.WithField(logger.FieldError, cleanupErr.Error()).
					Warn("Failed to remove temporary directory for failed process start")
			}
		}
		return nil, fmt.Errorf("failed to start Node.js process: %w", err)
	}

	// Add PID to logger
	if cmd.Process != nil {
		procLogger = procLogger.WithField(logger.FieldPID, cmd.Process.Pid)
	}

	procLogger.Info("Started Node.js process")

	// Create the HTTP client
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	client := NewNodeClient(baseURL, pm.requestTimeout, procLogger)

	// Create the process info
	process = &ProcessInfo{
		Process:     cmd,
		Client:      client,
		LastUsed:    time.Now(),
		Port:        port,
		TempDirPath: uniqueDirPath,
	}

	// Store the process
	pm.processes[specHash] = process

	// Wait for the HTTP server to be ready
	procLogger.Info("Waiting for Node.js HTTP server to be ready")
	waitCtx, cancel := context.WithTimeout(ctx, pm.healthCheckWait)
	defer cancel()

	if err := client.WaitForReady(waitCtx, pm.healthCheckWait, pm.healthCheckInterval); err != nil {
		procLogger.WithField(logger.FieldError, err.Error()).
			Error("Failed to wait for Node.js HTTP server to be ready")

		// Kill the process
		if process.Process.Process != nil {
			process.Process.Process.Kill()
		}
		delete(pm.processes, specHash)

		// Clean up the temporary directory
		if uniqueDirPath != "" {
			procLogger.Info("Removing temporary directory for failed process")
			if cleanupErr := os.RemoveAll(uniqueDirPath); cleanupErr != nil {
				procLogger.WithField(logger.FieldError, cleanupErr.Error()).
					Warn("Failed to remove temporary directory for failed process")
			}
		}

		return nil, fmt.Errorf("Node.js HTTP server failed to start: %w", err)
	}

	// Verify the process is still running after initialization
	if !pm.isProcessHealthy(process) {
		procLogger.Error("Process is not healthy after initialization")
		delete(pm.processes, specHash)

		// Clean up the temporary directory
		if uniqueDirPath != "" {
			procLogger.Info("Removing temporary directory for unhealthy process")
			if cleanupErr := os.RemoveAll(uniqueDirPath); cleanupErr != nil {
				procLogger.WithField(logger.FieldError, cleanupErr.Error()).
					Warn("Failed to remove temporary directory for unhealthy process")
			}
		}

		return nil, fmt.Errorf("process failed to initialize properly")
	}

	procLogger.Info("Process successfully initialized and ready")
	return process, nil
}

// logWriter is a io.Writer that writes to a logger with buffering for partial lines
type logWriter struct {
	logger     logger.Logger
	prefix     string
	buffer     []byte
	bufferLock sync.Mutex
}

func (w *logWriter) Write(p []byte) (n int, err error) {
	w.bufferLock.Lock()
	defer w.bufferLock.Unlock()

	// Append the new data to our buffer
	w.buffer = append(w.buffer, p...)

	// Process complete lines from the buffer
	lines := w.processBuffer()

	// Log each complete line
	for _, line := range lines {
		if line != "" {
			// Try to parse the line as JSON
			var jsonData map[string]interface{}
			if err := json.Unmarshal([]byte(line), &jsonData); err == nil {
				// Successfully parsed as JSON
				// Create a new map with "js-" prefix for each key
				prefixedData := make(map[string]interface{})
				for k, v := range jsonData {
					prefixedData["js-"+k] = v
				}

				// Check if this is an error message
				if _, hasError := jsonData["error"]; hasError {
					// Log as an error with fields
					w.logger.WithFields(prefixedData).
						WithField(logger.FieldComponent, "node").
						Error("Node.js process reported an error")
				} else {
					// Log as info with fields
					w.logger.WithFields(prefixedData).
						WithField(logger.FieldComponent, "node").
						Info(w.prefix)
				}
			} else {
				// Check if this looks like an error message
				if strings.Contains(strings.ToLower(line), "error") ||
					strings.Contains(strings.ToLower(line), "exception") ||
					strings.Contains(strings.ToLower(line), "fail") {
					// Log as an error
					w.logger.WithField(logger.FieldComponent, "node").
						WithField(logger.FieldError, line).
						Error("Node.js process reported an error")
				} else {
					// Log as plain text
					w.logger.WithField(logger.FieldComponent, "node").
						Infof("%s%s", w.prefix, line)
				}
			}
		}
	}

	return len(p), nil
}

// processBuffer processes the buffer and returns complete lines
// Any incomplete line at the end remains in the buffer
func (w *logWriter) processBuffer() []string {
	var lines []string
	var i, start int

	// Find complete lines in the buffer
	for i < len(w.buffer) {
		if w.buffer[i] == '\n' {
			// Extract the line (excluding the newline)
			line := string(w.buffer[start:i])
			// Trim carriage returns if present
			line = strings.TrimSuffix(line, "\r")
			lines = append(lines, line)

			// Move start to after this newline
			start = i + 1
		}
		i++
	}

	// If we processed any complete lines, update the buffer to contain only the remainder
	if start > 0 {
		w.buffer = w.buffer[start:]
	}

	return lines
}

// Flush forces any buffered data to be written
func (w *logWriter) Flush() {
	w.bufferLock.Lock()
	defer w.bufferLock.Unlock()

	// If there's any data in the buffer, log it even if it's not a complete line
	if len(w.buffer) > 0 {
		line := string(w.buffer)
		w.logger.WithField(logger.FieldComponent, "node").
			WithField("incomplete", true).
			Infof("%s%s (incomplete line)", w.prefix, line)
		w.buffer = nil
	}
}
