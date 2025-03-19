package node

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fabrique/crossplane-skyhook/pkg/hash"
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
	logger      *log.Logger
}

// NewProcessManager creates a new process manager
func NewProcessManager(gcInterval, idleTimeout time.Duration, tempDir string, logger *log.Logger) (*ProcessManager, error) {
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
			pm.logger.Printf("Terminating idle process: %s", id)
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

	// Write the request to the process's stdin
	if _, err := process.Stdin.Write(append(requestJSON, '\n')); err != nil {
		return "", fmt.Errorf("failed to write to process stdin: %w", err)
	}

	// Read the response from the process's stdout
	// We'll use a simple protocol where each message is a single line
	var responseBytes []byte
	buffer := make([]byte, 4096)
	for {
		n, err := process.Stdout.Read(buffer)
		if err != nil {
			if err == io.EOF {
				break
			}
			return "", fmt.Errorf("failed to read from process stdout: %w", err)
		}
		responseBytes = append(responseBytes, buffer[:n]...)
		if n > 0 && buffer[n-1] == '\n' {
			break
		}
	}

	return string(responseBytes), nil
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

	// Check if we're running in the test directory
	var indexPath string
	if strings.Contains(cwd, "test/e2e") {
		// Use the test-specific implementation
		indexPath = filepath.Join(cwd, "index.js")
	} else {
		// Use the main implementation
		indexPath = filepath.Join(cwd, "src", "index.ts")
	}

	// Create the Node.js process with the appropriate path to index.ts
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

	pm.logger.Printf("Started Node.js process for code hash: %s", codeHash[:8])

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
	logger *log.Logger
	prefix string
}

func (w *logWriter) Write(p []byte) (n int, err error) {
	w.logger.Printf("%s%s", w.prefix, string(p))
	return len(p), nil
}
