package node

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/socialgouv/xfuncjs-server/pkg/hash"
	"github.com/socialgouv/xfuncjs-server/pkg/logger"
	"github.com/socialgouv/xfuncjs-server/pkg/types"
	"sigs.k8s.io/yaml"
)

// WorkspacePackage represents a package in the yarn workspace
type WorkspacePackage struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Path    string
}

// WorkspaceInfo represents the output from yarn workspaces list --json
type WorkspaceInfo struct {
	Location string `json:"location"`
	Name     string `json:"name"`
}

// getWorkspacePackages discovers workspace packages using yarn workspaces list --json
func getWorkspacePackages(workspaceRoot string, logger logger.Logger) (map[string]string, error) {
	// Run yarn workspaces list --json from the workspace root
	cmd := exec.Command("yarn", "workspaces", "list", "--json")
	cmd.Dir = workspaceRoot

	output, err := cmd.Output()
	if err != nil {
		logger.WithField("error", err.Error()).
			Error("Failed to run yarn workspaces list command")
		return nil, fmt.Errorf("failed to run yarn workspaces list: %w", err)
	}

	// Parse the output - each line is a separate JSON object
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	workspaceMap := make(map[string]string)

	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		var workspace WorkspaceInfo
		if err := json.Unmarshal([]byte(line), &workspace); err != nil {
			logger.WithField("line", line).
				WithField("error", err.Error()).
				Warn("Failed to parse workspace info line")
			continue
		}

		// Skip the root workspace (location is ".")
		if workspace.Location == "." {
			continue
		}

		workspaceMap[workspace.Name] = workspace.Location
	}

	logger.WithField("workspace_count", len(workspaceMap)).
		Info("Discovered workspace packages")

	return workspaceMap, nil
}

// resolveWorkspacePackage resolves a link: dependency to a workspace package
func resolveWorkspacePackage(dependencyValue, workspaceRoot string, workspaceMap map[string]string, logger logger.Logger) (string, string, error) {
	var targetPath string

	if strings.HasPrefix(dependencyValue, "link:") {
		targetPath = strings.TrimPrefix(dependencyValue, "link:")
	} else {
		return dependencyValue, "", nil
	}

	// Clean the target path
	targetPath = filepath.Clean(targetPath)

	// Normalize the target path to handle relative paths like ../../../packages/sdk
	// Convert to absolute path and then back to relative from workspace root
	absoluteTargetPath := filepath.Join(workspaceRoot, targetPath)
	cleanAbsolutePath := filepath.Clean(absoluteTargetPath)
	normalizedTargetPath, err := filepath.Rel(workspaceRoot, cleanAbsolutePath)
	if err != nil {
		normalizedTargetPath = targetPath
	}

	// If the normalized path still contains "..", it means it's going outside the workspace
	// In this case, try to extract just the final part (e.g., "packages/sdk" from "../packages/sdk")
	if strings.Contains(normalizedTargetPath, "..") {
		// Split the path and find the part that doesn't start with ".."
		parts := strings.Split(normalizedTargetPath, string(filepath.Separator))
		var cleanParts []string
		for _, part := range parts {
			if part != ".." && part != "." && part != "" {
				cleanParts = append(cleanParts, part)
			}
		}
		if len(cleanParts) > 0 {
			candidatePath := filepath.Join(cleanParts...)
			normalizedTargetPath = candidatePath
		}
	}

	// Find the workspace package at this location
	var packageName string
	var packageLocation string

	for name, location := range workspaceMap {
		if location == normalizedTargetPath {
			packageName = name
			packageLocation = location
			break
		}
	}

	if packageName == "" {
		// Try to resolve by reading package.json at the target path
		absolutePath := filepath.Join(workspaceRoot, targetPath)
		packageJSONPath := filepath.Join(absolutePath, "package.json")

		if _, err := os.Stat(packageJSONPath); os.IsNotExist(err) {
			return "", "", fmt.Errorf("no workspace package found at %s", targetPath)
		}

		// Read the package.json to get the package name
		packageJSONBytes, err := os.ReadFile(packageJSONPath)
		if err != nil {
			logger.WithField("package_json_path", packageJSONPath).
				WithField("error", err.Error()).
				Error("Failed to read package.json")
			return "", "", fmt.Errorf("failed to read package.json at %s: %w", packageJSONPath, err)
		}

		var pkg WorkspacePackage
		if err := json.Unmarshal(packageJSONBytes, &pkg); err != nil {
			logger.WithField("package_json_path", packageJSONPath).
				WithField("error", err.Error()).
				Error("Failed to parse package.json")
			return "", "", fmt.Errorf("failed to parse package.json at %s: %w", packageJSONPath, err)
		}

		if pkg.Name == "" {
			return "", "", fmt.Errorf("package name not found in package.json at %s", packageJSONPath)
		}

		// Check if this package is actually in the workspace
		if actualLocation, exists := workspaceMap[pkg.Name]; exists {
			if actualLocation != targetPath {
				logger.WithField("package_name", pkg.Name).
					WithField("expected_location", targetPath).
					WithField("actual_location", actualLocation).
					Warn("Package location mismatch, using actual location")
				packageLocation = actualLocation
			} else {
				packageLocation = targetPath
			}
			packageName = pkg.Name
		} else {
			return "", "", fmt.Errorf("package %s at %s is not in the workspace", pkg.Name, targetPath)
		}
	}

	// Return as link dependency with absolute path to /app
	linkRef := fmt.Sprintf("link:/app/%s", packageLocation)
	logger.WithField("package_name", packageName).
		WithField("resolved_to", linkRef).
		Info("Resolved workspace dependency")

	return linkRef, packageLocation, nil
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

	// Discover workspace packages using yarn workspaces list --json
	workspaceMap, err := getWorkspacePackages("/app", procLogger)
	if err != nil {
		procLogger.WithField(logger.FieldError, err.Error()).
			Warn("Failed to discover workspace packages, workspace dependencies may not work correctly")
		workspaceMap = make(map[string]string) // Use empty map as fallback
	}

	// Create dependencies map
	dependencies := make(map[string]interface{})

	procLogger.WithField("input_dependencies", input.Spec.Source.Dependencies).
		Info("Starting dependency processing")

	// Add user-specified dependencies
	for k, v := range input.Spec.Source.Dependencies {
		resolvedDep := v

		if strings.HasPrefix(v, "link:") {
			// Resolve workspace package dependencies
			if resolved, _, resolveErr := resolveWorkspacePackage(v, "/app", workspaceMap, procLogger); resolveErr != nil {
				procLogger.WithField("dependency", k).
					WithField("value", v).
					WithField(logger.FieldError, resolveErr.Error()).
					Warn("Failed to resolve workspace dependency, using original value")
			} else {
				resolvedDep = resolved
			}
		}

		dependencies[k] = resolvedDep
	}

	procLogger.WithField("total_dependencies", len(dependencies)).
		Info("Completed dependency processing")

	// Create simple package.json with link dependencies
	packageJSON := map[string]interface{}{
		"name":         "xfuncjs-function",
		"version":      "0.0.0",
		"private":      true,
		"dependencies": dependencies,
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
	procLogger.Info("Created package.json with workspace dependencies")

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

	// Run yarn install to resolve workspace dependencies
	procLogger.Info("Running yarn install")
	yarnCmd := exec.Command("yarn", "install")
	yarnCmd.Dir = uniqueDirPath // Set the working directory
	yarnCmd.Env = append(os.Environ(),
		"NODE_OPTIONS=--experimental-strip-types --experimental-transform-types --no-warnings",
		"NODE_NO_WARNINGS=1",
	)

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

	port := pm.getNextPort()
	procLogger = procLogger.WithField(logger.FieldPort, port)

	// Create the Node.js process with the appropriate path to the index file
	// Use a background context that won't be canceled when the request is done
	processCtx := context.Background()

	// Use Node.js with TypeScript sources directly
	cmd := exec.CommandContext(processCtx, "node", "/app/packages/server/src/index.ts", "--code-file-path", tempFilePath)

	cmd.Env = append(os.Environ(),
		fmt.Sprintf("PORT=%d", port),
		"NODE_OPTIONS=--experimental-strip-types --experimental-transform-types --no-warnings",
		"NODE_NO_WARNINGS=1",
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
