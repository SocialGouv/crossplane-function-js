package node

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/socialgouv/xfuncjs-server/pkg/hash"
	"github.com/socialgouv/xfuncjs-server/pkg/logger"
	"github.com/socialgouv/xfuncjs-server/pkg/types"
)

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
	yarnInstaller       *YarnInstaller
	dependencyResolver  *DependencyResolver
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

// getOrCreateProcess gets an existing process for the given input or creates a new one
func (pm *ProcessManager) getOrCreateProcess(ctx context.Context, input *types.XFuncJSInput) (*ProcessInfo, error) {
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

	// Discover workspace packages
	workspaceMap, err := GetWorkspacePackages("/app", procLogger)
	if err != nil {
		procLogger.WithField(logger.FieldError, err.Error()).
			Warn("Failed to discover workspace packages, workspace dependencies may not work correctly")
		workspaceMap = make(map[string]string) // Use empty map as fallback
	}

	// Resolve dependencies
	dependencies, err := pm.dependencyResolver.ResolveDependencies(input.Spec.Source.Dependencies, workspaceMap, "/app", procLogger)
	if err != nil {
		// Clean up the temporary directory
		if uniqueDirPath != "" {
			procLogger.Info("Removing temporary directory after dependency resolution failure")
			if cleanupErr := os.RemoveAll(uniqueDirPath); cleanupErr != nil {
				procLogger.WithField(logger.FieldError, cleanupErr.Error()).
					Warn("Failed to remove temporary directory after dependency resolution failure")
			}
		}
		return nil, fmt.Errorf("failed to resolve dependencies: %w", err)
	}

	// Create package.json
	if err := pm.yarnInstaller.CreatePackageJSON(uniqueDirPath, dependencies, procLogger); err != nil {
		// Clean up the temporary directory
		if uniqueDirPath != "" {
			procLogger.Info("Removing temporary directory after package.json creation failure")
			if cleanupErr := os.RemoveAll(uniqueDirPath); cleanupErr != nil {
				procLogger.WithField(logger.FieldError, cleanupErr.Error()).
					Warn("Failed to remove temporary directory after package.json creation failure")
			}
		}
		return nil, fmt.Errorf("failed to create package.json: %w", err)
	}

	// Prepare yarn environment
	yarnPath, err := pm.yarnInstaller.PrepareYarnEnvironment(uniqueDirPath, input.Spec.Source.YarnLock, input.Spec.Source.TsConfig, procLogger)
	if err != nil {
		procLogger.WithField(logger.FieldError, err.Error()).
			Warn("Failed to prepare yarn environment, but continuing anyway")
	}

	// Install dependencies using the queue
	if err := pm.yarnInstaller.InstallDependencies(ctx, uniqueDirPath, specHash, yarnPath, procLogger); err != nil {
		procLogger.WithField(logger.FieldError, err.Error()).
			Warn("Yarn install failed, but continuing anyway")
	}

	// Construct the absolute path to the yarn executable
	yarnExecPath := filepath.Join(uniqueDirPath, yarnPath)
	procLogger.WithField("yarnExecPath", yarnExecPath).Info("Using yarn executable")

	port, err := pm.getAvailablePort()
	if err != nil {
		// Clean up the temporary directory
		if uniqueDirPath != "" {
			procLogger.Info("Removing temporary directory after port selection failure")
			if cleanupErr := os.RemoveAll(uniqueDirPath); cleanupErr != nil {
				procLogger.WithField(logger.FieldError, cleanupErr.Error()).
					Warn("Failed to remove temporary directory after port selection failure")
			}
		}
		return nil, fmt.Errorf("failed to select a free port: %w", err)
	}
	procLogger = procLogger.WithField(logger.FieldPort, port)

	// Create the Node.js process with the appropriate path to the index file
	// Use a background context that won't be canceled when the request is done
	processCtx := context.Background()

	// Use Node.js with TypeScript sources directly
	cmd := exec.CommandContext(processCtx, "node", "/app/packages/server/src/index.ts")
	// Ensure Node resolves workspace deps; set working directory to the server package (ensures tsx resolution)
	cmd.Dir = "/app/packages/server"

	// Ensure our custom ESM alias loader is enabled via NODE_OPTIONS
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("PORT=%d", port),
		fmt.Sprintf("NODE_OPTIONS=%s", "--import tsx"),
		fmt.Sprintf("XFUNCJS_CODE_FILE_PATH=%s", tempFilePath),
		"XFUNCJS_LOG_LEVEL=debug", // Ensure we capture all logs from Node.js
		"LOG_LEVEL=debug",         // Fallback for Pino logger
		"BIND_ADDR=127.0.0.1",     // Bind server to loopback only
	)

	// Create custom logWriters for both stdout and stderr
	stdoutWriter := &logWriter{
		logger:     procLogger,
		prefix:     fmt.Sprintf("node[%s]: ", specHash[:8]),
		streamType: "stdout",
	}
	stderrWriter := &logWriter{
		logger:     procLogger,
		prefix:     fmt.Sprintf("node[%s]: ", specHash[:8]),
		streamType: "stderr",
	}

	// Redirect both stdout and stderr to our loggers
	cmd.Stdout = stdoutWriter
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
			if err := process.Process.Process.Kill(); err != nil {
				procLogger.WithField("error", err.Error()).Warn("Failed to kill process")
			}
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
