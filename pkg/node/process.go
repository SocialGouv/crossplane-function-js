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
	"sigs.k8s.io/yaml"
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

				// Create a minimal copy of /app for xfuncjs-server
				// We only need to copy essential files, not the entire /app directory
				appDstPath := filepath.Join(packagesDir, "xfuncjs-server")
				if err := os.MkdirAll(appDstPath, 0755); err != nil {
					procLogger.WithField(logger.FieldError, err.Error()).
						Warn("Failed to create xfuncjs-server directory")
				} else {
					// Copy package.json and other essential files
					essentialFiles := []string{"package.json", "tsconfig.json"}
					for _, file := range essentialFiles {
						srcFilePath := filepath.Join("/app", file)
						dstFilePath := filepath.Join(appDstPath, file)
						if err := copyFile(srcFilePath, dstFilePath); err != nil {
							procLogger.WithField(logger.FieldError, err.Error()).
								WithField("file", file).
								Warn("Failed to copy essential file to xfuncjs-server directory")
						}
					}
					procLogger.Info("Created xfuncjs-server directory with essential files")
				}
			}
		}

		dependencies := make(map[string]interface{})

		dependencies["@crossplane-js/sdk"] = "workspace:^"

		// Create a copy of the dependencies map from the input
		for k, v := range input.Spec.Source.Dependencies {
			dependencies[k] = v
		}

		workspaces := make(map[string]interface{})
		workspaces["packages"] = []string{"packages/*"}

		packageJSON := map[string]interface{}{
			"name":         "xfuncjs-function",
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
		if err != nil {
			procLogger.WithField("error", err.Error()).Warn("Failed to read .yarnrc.yml")
		}
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
		yarnCmd := exec.Command("/app/xfuncjs-server-js", "yarn", "workspaces", "focus", "--production")
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
	cmd := exec.CommandContext(processCtx, "/app/xfuncjs-server-js", "--code-file-path", tempFilePath)

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
