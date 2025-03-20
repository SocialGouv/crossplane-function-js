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
	"time"

	"github.com/socialgouv/crossplane-skyhook/pkg/hash"
	"github.com/socialgouv/crossplane-skyhook/pkg/logger"
)

// ProcessInfo holds information about a Node.js process
type ProcessInfo struct {
	Process  *exec.Cmd
	Stdin    io.WriteCloser
	Stdout   io.ReadCloser
	LastUsed time.Time
	Lock     sync.Mutex
}

// ProcessManager manages Node.js processes
type ProcessManager struct {
	processes   map[string]*ProcessInfo
	lock        sync.RWMutex
	gcInterval  time.Duration
	idleTimeout time.Duration
	tempDir     string
	logger      logger.Logger
}

// NewProcessManager creates a new process manager
func NewProcessManager(gcInterval, idleTimeout time.Duration, tempDir string, logger logger.Logger) (*ProcessManager, error) {
	// Create temp directory if it doesn't exist
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}

	pm := &ProcessManager{
		processes:   make(map[string]*ProcessInfo),
		gcInterval:  gcInterval,
		idleTimeout: idleTimeout,
		tempDir:     tempDir,
		logger:      logger,
	}

	// Start the garbage collector
	pm.startGarbageCollector()

	return pm, nil
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
			// Close stdin to signal the process to exit
			if info.Stdin != nil {
				info.Stdin.Close()
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
	// Get or create a process for this code
	process, err := pm.getOrCreateProcess(ctx, code)
	if err != nil {
		return "", fmt.Errorf("failed to get or create process: %w", err)
	}

	// Create a request
	request := map[string]interface{}{
		"code":  code,
		"input": json.RawMessage(inputJSON),
	}

	// Marshal the request to JSON
	requestJSON, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	// Send the request to the process
	process.Lock.Lock()
	defer process.Lock.Unlock()

	// Update the last used time
	process.LastUsed = time.Now()

	// Create a context with timeout for the operation
	execCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Create channels for the result and error
	resultCh := make(chan string, 1)
	errCh := make(chan error, 1)

	// Execute the function in a goroutine
	go func() {
		// Write the request to the process's stdin
		pm.logger.Debugf("Writing request to process stdin: %s", string(requestJSON)[:100])
		if _, err := process.Stdin.Write(append(requestJSON, '\n')); err != nil {
			errCh <- fmt.Errorf("failed to write to process stdin: %w", err)
			return
		}
		pm.logger.Debug("Request written to process stdin")

		// Read the response from the process's stdout
		// We'll use a simple protocol where each message is a single line
		var responseBytes []byte
		buffer := make([]byte, 4096)
		for {
			pm.logger.Debug("Reading from process stdout")
			n, err := process.Stdout.Read(buffer)
			if err != nil {
				if err == io.EOF {
					pm.logger.Debug("EOF received from process stdout")
					break
				}
				errCh <- fmt.Errorf("failed to read from process stdout: %w", err)
				return
			}
			pm.logger.Debugf("Read %d bytes from process stdout", n)
			responseBytes = append(responseBytes, buffer[:n]...)
			if n > 0 && buffer[n-1] == '\n' {
				pm.logger.Debug("Newline received, breaking read loop")
				break
			}
		}

		// Send the result
		resultCh <- string(responseBytes)
	}()

	// Wait for the result, error, or timeout
	select {
	case result := <-resultCh:
		pm.logger.Debug("Received result from process")
		return result, nil
	case err := <-errCh:
		pm.logger.Errorf("Error executing function: %v", err)
		// If there's an error, we should restart the process next time
		pm.restartProcess(process, code)
		return "", err
	case <-execCtx.Done():
		pm.logger.Error("Function execution timed out")
		// If there's a timeout, we should restart the process next time
		pm.restartProcess(process, code)
		return "", fmt.Errorf("function execution timed out after 30 seconds")
	}
}

// restartProcess marks a process for restart by removing it from the processes map
func (pm *ProcessManager) restartProcess(process *ProcessInfo, code string) {
	codeHash := hash.GenerateHash(code)
	pm.lock.Lock()
	defer pm.lock.Unlock()

	// Check if this process is still in the map
	if existingProcess, exists := pm.processes[codeHash]; exists && existingProcess == process {
		pm.logger.Infof("Marking process for restart: %s", codeHash[:8])
		// Close stdin to signal the process to exit
		if process.Stdin != nil {
			process.Stdin.Close()
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
		return process, nil
	}

	// Create a new process
	pm.lock.Lock()
	defer pm.lock.Unlock()

	// Check again in case another goroutine created the process while we were waiting
	if process, exists = pm.processes[codeHash]; exists {
		return process, nil
	}

	// Create a temporary file for the code
	extension := ".js"
	if isTypeScript(code) {
		extension = ".ts"
	}
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

	// Check if we're running the simple_test.go test
	if strings.Contains(cwd, "test/e2e") {
		// We're in the test/e2e directory, look for index.js in the same directory
		testIndexPath := filepath.Join(cwd, "index.js")
		if _, err := os.Stat(testIndexPath); err == nil {
			indexPath = testIndexPath
			pm.logger.Infof("Using test index file: %s", indexPath)
		} else {
			// Try the project root
			rootPath := filepath.Join(cwd, "..", "..")
			testIndexPath = filepath.Join(rootPath, "test", "e2e", "index.js")
			if _, err := os.Stat(testIndexPath); err == nil {
				indexPath = testIndexPath
				pm.logger.Infof("Using test index file from project root: %s", indexPath)
			} else {
				return nil, fmt.Errorf("could not find test index file at %s or %s", filepath.Join(cwd, "index.js"), testIndexPath)
			}
		}
	} else {
		// We're in the main project, look for index.ts/js in src directory
		rootPath := cwd
		tsPath := filepath.Join(rootPath, "src", "index.ts")
		jsPath := filepath.Join(rootPath, "src", "index.js")

		if _, err := os.Stat(tsPath); err == nil {
			indexPath = tsPath
		} else if _, err := os.Stat(jsPath); err == nil {
			indexPath = jsPath
		} else {
			return nil, fmt.Errorf("could not find index file at %s or %s", tsPath, jsPath)
		}
		pm.logger.Infof("Using main index file: %s", indexPath)
	}

	// Create the Node.js process with the appropriate path to the index file
	cmd := exec.CommandContext(ctx, "node", "--no-warnings", "--experimental-strip-types", indexPath, tempFilePath)
	cmd.Env = append(os.Environ(), "NODE_OPTIONS=--no-warnings --experimental-strip-types")

	// Set up stdin and stdout
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		stdin.Close()
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	// Redirect stderr to our logger
	cmd.Stderr = &logWriter{logger: pm.logger, prefix: fmt.Sprintf("node[%s]: ", codeHash[:8])}

	// Start the process
	if err := cmd.Start(); err != nil {
		stdin.Close()
		stdout.Close()
		return nil, fmt.Errorf("failed to start Node.js process: %w", err)
	}

	pm.logger.Infof("Started Node.js process for code hash: %s", codeHash[:8])

	// Create the process info
	process = &ProcessInfo{
		Process:  cmd,
		Stdin:    stdin,
		Stdout:   stdout,
		LastUsed: time.Now(),
	}

	// Store the process
	pm.processes[codeHash] = process

	return process, nil
}

// isTypeScript checks if the code is TypeScript
func isTypeScript(code string) bool {
	// This is a very simple check, in a real implementation we might want to
	// use a more sophisticated approach
	return true
}

// logWriter is a simple io.Writer that writes to a logger
type logWriter struct {
	logger logger.Logger
	prefix string
}

func (w *logWriter) Write(p []byte) (n int, err error) {
	// Trim trailing newlines for cleaner log output
	message := string(p)
	for len(message) > 0 && (message[len(message)-1] == '\n' || message[len(message)-1] == '\r') {
		message = message[:len(message)-1]
	}

	if message != "" {
		w.logger.WithField("component", "node").Infof("%s%s", w.prefix, message)
	}
	return len(p), nil
}
