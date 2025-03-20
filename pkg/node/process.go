package node

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/socialgouv/crossplane-skyhook/pkg/hash"
	"github.com/socialgouv/crossplane-skyhook/pkg/logger"
)

// ProcessInfo holds information about a Node.js process
type ProcessInfo struct {
	Process  *exec.Cmd
	Client   *NodeClient
	LastUsed time.Time
	Lock     sync.Mutex
}

// ProcessManager manages Node.js processes
type ProcessManager struct {
	processes           map[string]*ProcessInfo
	lock                sync.RWMutex
	gcInterval          time.Duration
	idleTimeout         time.Duration
	tempDir             string
	logger              logger.Logger
	nodeServerPort      int
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

// WithNodeServerPort sets the port for the Node.js HTTP server
func WithNodeServerPort(port int) ProcessManagerOption {
	return func(pm *ProcessManager) {
		pm.nodeServerPort = port
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

// SetNodeServerPort sets the port for the Node.js HTTP server
func (pm *ProcessManager) SetNodeServerPort(port int) {
	pm.nodeServerPort = port
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
	pm.lock.Lock()
	defer pm.lock.Unlock()

	for id, info := range pm.processes {
		info.Lock.Lock()
		if now.Sub(info.LastUsed) > pm.idleTimeout {
			pm.logger.Infof("Terminating idle process: %s", id)
			// Send SIGTERM to signal the process to exit gracefully
			if info.Process.Process != nil {
				info.Process.Process.Signal(syscall.SIGTERM)
			}

			// Add a longer delay to allow pino async logger to flush logs before killing the process
			pm.logger.Infof("Waiting for logs to flush before killing idle process: %s", id)
			time.Sleep(2 * time.Second)

			// Flush any buffered stderr data
			if stderrWriter, ok := info.Process.Stderr.(*logWriter); ok {
				stderrWriter.Flush()
				pm.logger.Infof("Flushed stderr buffer for idle process: %s", id)
			}

			// Kill the process if it doesn't exit gracefully
			if info.Process.Process != nil {
				info.Process.Process.Kill()
			}
			delete(pm.processes, id)
		}
		info.Lock.Unlock()
	}
}

// ExecuteFunction executes a JavaScript/TypeScript function with the given input
func (pm *ProcessManager) ExecuteFunction(ctx context.Context, code, inputJSON string) (string, error) {
	// Maximum number of retries
	const maxRetries = 3

	// Initial retry delay (will be multiplied by 2^attempt for exponential backoff)
	retryDelay := 500 * time.Millisecond

	var lastErr error

	// Try up to maxRetries times
	for attempt := 0; attempt <= maxRetries; attempt++ {
		// If this is a retry, log it
		if attempt > 0 {
			pm.logger.Infof("Retry attempt %d/%d after error: %v", attempt, maxRetries, lastErr)
			// Wait with exponential backoff before retrying
			backoffTime := retryDelay * time.Duration(1<<uint(attempt-1))
			time.Sleep(backoffTime)
		}

		// Get or create a process for this code
		process, err := pm.getOrCreateProcess(ctx, code)
		if err != nil {
			lastErr = fmt.Errorf("failed to get or create process: %w", err)
			continue // Retry
		}

		// Check if the process is healthy before proceeding
		if !pm.isProcessHealthy(process) {
			pm.logger.Warn("Process appears unhealthy, restarting it")
			pm.restartProcess(process, code)
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
		pm.logger.Debug("Sending request to Node.js server")
		result, err := process.Client.ExecuteFunction(execCtx, code, inputJSON)

		// Cleanup
		cancel() // Cancel the context

		if err != nil {
			process.Lock.Unlock()
			pm.logger.Errorf("Error executing function: %v", err)

			// If there's an error, we should restart the process
			pm.restartProcess(process, code)

			// Save the error for potential retry
			lastErr = err
			continue // Retry
		}

		// Success
		pm.logger.Debug("Received result from Node.js server")
		process.Lock.Unlock()
		return result, nil
	}

	// If we've exhausted all retries, return the last error
	return "", fmt.Errorf("all retry attempts failed, last error: %w", lastErr)
}

// isProcessHealthy checks if a process is healthy and ready to receive input
func (pm *ProcessManager) isProcessHealthy(process *ProcessInfo) bool {
	// Check if the process is still running
	if process.Process.Process == nil {
		return false
	}

	// Check if the process has exited
	if err := process.Process.Process.Signal(syscall.Signal(0)); err != nil {
		pm.logger.Warnf("Process health check failed: %v", err)
		return false
	}

	// Check if the client is valid
	if process.Client == nil {
		return false
	}

	// Check if the process is still running and responsive
	exitCode := process.Process.ProcessState
	if exitCode != nil {
		pm.logger.Warnf("Process has exited with code: %v", exitCode)
		return false
	}

	// Check if the HTTP server is responsive
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := process.Client.CheckHealth(ctx); err != nil {
		pm.logger.Warnf("HTTP server health check failed: %v", err)
		return false
	}

	return true
}

// restartProcess marks a process for restart by removing it from the processes map
func (pm *ProcessManager) restartProcess(process *ProcessInfo, code string) {
	codeHash := hash.GenerateHash(code)
	pm.lock.Lock()
	defer pm.lock.Unlock()

	// Check if this process is still in the map
	if existingProcess, exists := pm.processes[codeHash]; exists && existingProcess == process {
		pm.logger.Infof("Marking process for restart: %s", codeHash[:8])

		// Send SIGTERM to signal the process to exit gracefully
		if process.Process.Process != nil {
			process.Process.Process.Signal(syscall.SIGTERM)
		}

		// Add a longer delay to allow pino async logger to flush logs before killing the process
		pm.logger.Infof("Waiting for logs to flush before killing process: %s", codeHash[:8])
		time.Sleep(2 * time.Second)

		// Flush any buffered stderr data
		if stderrWriter, ok := process.Process.Stderr.(*logWriter); ok {
			stderrWriter.Flush()
			pm.logger.Infof("Flushed stderr buffer for process: %s", codeHash[:8])
		}

		// Kill the process if it doesn't exit gracefully
		if process.Process.Process != nil {
			process.Process.Process.Kill()
		}
		delete(pm.processes, codeHash)
	}
}

// getOrCreateProcess gets an existing process for the given code or creates a new one
func (pm *ProcessManager) getOrCreateProcess(ctx context.Context, code string) (*ProcessInfo, error) {
	codeHash := hash.GenerateHash(code)

	// Check if we already have a process for this code
	pm.lock.RLock()
	process, exists := pm.processes[codeHash]
	pm.lock.RUnlock()

	if exists {
		// Verify the process is still healthy before returning it
		if !pm.isProcessHealthy(process) {
			pm.logger.Warn("Existing process is unhealthy, creating a new one")
			pm.restartProcess(process, code)
			exists = false
		} else {
			return process, nil
		}
	}

	// Create a new process
	pm.lock.Lock()
	defer pm.lock.Unlock()

	// Check again in case another goroutine created the process while we were waiting
	if process, exists = pm.processes[codeHash]; exists {
		if pm.isProcessHealthy(process) {
			return process, nil
		}
		// Process exists but is unhealthy, remove it
		pm.logger.Warn("Existing process is unhealthy, removing it")
		delete(pm.processes, codeHash)
	}

	// Create a temporary file for the code
	extension := ".ts"
	tempFilename := hash.GenerateTempFilename(code, extension)
	tempFilePath := filepath.Join(pm.tempDir, tempFilename)

	// Write the code to the temporary file
	if err := os.WriteFile(tempFilePath, []byte(code), 0644); err != nil {
		return nil, fmt.Errorf("failed to write code to temporary file: %w", err)
	}

	// Create the Node.js process
	// Get the current working directory
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get current working directory: %w", err)
	}

	// Determine the correct path to the index file
	var indexPath string

	// We're in the main project, prioritize TypeScript files in src directory
	rootPath := cwd
	srcTsPath := filepath.Join(rootPath, "src", "index.ts")

	indexPath = srcTsPath
	pm.logger.Infof("Using TypeScript source file: %s", indexPath)

	// Create the Node.js process with the appropriate path to the index file
	cmd := exec.CommandContext(ctx, "node", indexPath, tempFilePath)
	cmd.Env = append(os.Environ(),
		"NODE_OPTIONS=--no-warnings --experimental-strip-types",
		// Add environment variables to control Node.js behavior
		"NODE_NO_WARNINGS=1",
		// Set the port for the HTTP server
		fmt.Sprintf("PORT=%d", pm.nodeServerPort),
	)

	// Create a custom logWriter that can detect ready signals
	readyChannel := make(chan struct{})
	stderrWriter := &logWriter{
		logger:       pm.logger,
		prefix:       fmt.Sprintf("node[%s]: ", codeHash[:8]),
		readyChannel: readyChannel,
	}

	// Redirect stderr to our logger
	cmd.Stderr = stderrWriter

	// Start the process
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start Node.js process: %w", err)
	}

	pm.logger.Infof("Started Node.js process for code hash: %s", codeHash[:8])

	// Create the HTTP client
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", pm.nodeServerPort)
	client := NewNodeClient(baseURL, pm.requestTimeout, pm.logger)

	// Create the process info
	process = &ProcessInfo{
		Process:  cmd,
		Client:   client,
		LastUsed: time.Now(),
	}

	// Store the process
	pm.processes[codeHash] = process

	// Wait for the process to signal it's ready
	pm.logger.Info("Waiting for Node.js process to be ready...")

	// Set up a timeout for the ready signal
	readyTimeout := 10 * time.Second
	select {
	case <-readyChannel:
		pm.logger.Info("Node.js process signaled it's ready via stderr")
	case <-time.After(readyTimeout):
		pm.logger.Warn("Timeout waiting for Node.js process ready signal, checking HTTP healthcheck")
	}

	// Wait for the HTTP server to be ready
	waitCtx, cancel := context.WithTimeout(ctx, pm.healthCheckWait)
	defer cancel()

	if err := client.WaitForReady(waitCtx, pm.healthCheckWait, pm.healthCheckInterval); err != nil {
		pm.logger.Errorf("Failed to wait for Node.js HTTP server to be ready: %v", err)
		// Kill the process
		if process.Process.Process != nil {
			process.Process.Process.Kill()
		}
		delete(pm.processes, codeHash)
		return nil, fmt.Errorf("Node.js HTTP server failed to start: %w", err)
	}

	// Verify the process is still running after initialization
	if !pm.isProcessHealthy(process) {
		pm.logger.Error("Process is not healthy after initialization")
		delete(pm.processes, codeHash)
		return nil, fmt.Errorf("process failed to initialize properly")
	}

	return process, nil
}

// logWriter is a io.Writer that writes to a logger with buffering for partial lines
type logWriter struct {
	logger       logger.Logger
	prefix       string
	buffer       []byte
	bufferLock   sync.Mutex
	readyChannel chan struct{}
	isReady      bool
}

func (w *logWriter) Write(p []byte) (n int, err error) {
	w.bufferLock.Lock()
	defer w.bufferLock.Unlock()

	// Append the new data to our buffer
	w.buffer = append(w.buffer, p...)

	// Process complete lines from the buffer
	lines := w.processBuffer()

	// Log each complete line and check for special signals
	for _, line := range lines {
		// Check for special signals
		if line == "READY" {
			w.logger.Info("Received READY signal from Node.js process")
			if w.readyChannel != nil && !w.isReady {
				close(w.readyChannel)
				w.isReady = true
			}
			continue
		} else if line == "HEARTBEAT" {
			// Just a heartbeat, don't log it to avoid noise
			continue
		}

		// Regular log line
		if line != "" {
			w.logger.WithField("component", "node").Infof("%s%s", w.prefix, line)
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
		w.logger.WithField("component", "node").Infof("%s%s (incomplete line)", w.prefix, line)
		w.buffer = nil
	}
}
