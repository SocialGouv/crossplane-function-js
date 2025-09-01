package node

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/socialgouv/xfuncjs-server/pkg/logger"
	"sigs.k8s.io/yaml"
)

// YarnInstallJob represents a yarn install job
type YarnInstallJob struct {
	WorkDir    string
	SpecHash   string
	Logger     logger.Logger
	Context    context.Context
	ResultChan chan error
	YarnPath   string
}

// YarnQueue manages concurrent yarn install operations
type YarnQueue struct {
	semaphore chan struct{} // Limits concurrent operations
	logger    logger.Logger
}

// Global yarn queue instance
var globalYarnQueue *YarnQueue

// InitializeYarnQueue initializes the global yarn queue with the specified concurrency limit
func InitializeYarnQueue(maxConcurrent int, logger logger.Logger) {
	globalYarnQueue = &YarnQueue{
		semaphore: make(chan struct{}, maxConcurrent),
		logger:    logger.WithField("component", "yarn-queue"),
	}
	logger.WithField("max_concurrent", maxConcurrent).Info("Initialized yarn install queue")
}

// GetYarnQueue returns the global yarn queue instance
func GetYarnQueue() *YarnQueue {
	return globalYarnQueue
}

// SubmitYarnInstall submits a yarn install job to the queue
func (yq *YarnQueue) SubmitYarnInstall(job *YarnInstallJob) {
	go func() {
		// Wait for a slot in the semaphore
		select {
		case yq.semaphore <- struct{}{}:
			// Got a slot, proceed with yarn install
			err := yq.executeYarnInstall(job)

			// Release the slot
			<-yq.semaphore

			// Send result back
			select {
			case job.ResultChan <- err:
			case <-job.Context.Done():
				// Context was cancelled, don't block
			}

		case <-job.Context.Done():
			// Context was cancelled while waiting
			job.ResultChan <- job.Context.Err()
		}
	}()
}

// executeYarnInstall performs the actual yarn install operation
func (yq *YarnQueue) executeYarnInstall(job *YarnInstallJob) error {
	jobLogger := job.Logger.WithField("yarn_queue", "install")
	jobLogger.Info("Starting queued yarn install")

	// Create yarn command
	yarnCmd := exec.CommandContext(job.Context, "yarn", "workspaces", "focus")
	yarnCmd.Dir = job.WorkDir
	yarnCmd.Env = os.Environ()

	// Create custom logWriters for yarn output
	yarnStdoutWriter := &logWriter{
		logger:     jobLogger.WithField("yarn", "stdout"),
		prefix:     fmt.Sprintf("yarn[%s]: ", job.SpecHash[:8]),
		streamType: "stdout",
	}
	yarnStderrWriter := &logWriter{
		logger:     jobLogger.WithField("yarn", "stderr"),
		prefix:     fmt.Sprintf("yarn[%s]: ", job.SpecHash[:8]),
		streamType: "stderr",
	}

	yarnCmd.Stdout = yarnStdoutWriter
	yarnCmd.Stderr = yarnStderrWriter

	// Run yarn install
	if err := yarnCmd.Run(); err != nil {
		jobLogger.WithField("error", err.Error()).Error("Yarn install failed")
		return fmt.Errorf("yarn install failed: %w", err)
	}

	jobLogger.Info("Yarn install completed successfully")
	return nil
}

// YarnInstaller handles yarn installation operations
type YarnInstaller struct {
	queue  *YarnQueue
	logger logger.Logger
}

// NewYarnInstaller creates a new yarn installer
func NewYarnInstaller(queue *YarnQueue, logger logger.Logger) *YarnInstaller {
	return &YarnInstaller{
		queue:  queue,
		logger: logger.WithField("component", "yarn-installer"),
	}
}

// PrepareYarnEnvironment prepares the yarn environment in the specified directory
func (yi *YarnInstaller) PrepareYarnEnvironment(workDir string, yarnLock string, tsConfig string, logger logger.Logger) (string, error) {
	// If yarn.lock is provided, write it to the temporary directory
	if yarnLock != "" {
		yarnLockPath := filepath.Join(workDir, "yarn.lock")
		if err := os.WriteFile(yarnLockPath, []byte(yarnLock), 0644); err != nil {
			logger.WithField("error", err.Error()).
				Warn("Failed to write yarn.lock to temporary directory")
		} else {
			logger.Info("Created yarn.lock in temporary directory")
		}
	}

	// If tsconfig.json is provided, write it to the temporary directory
	if tsConfig != "" {
		tsConfigPath := filepath.Join(workDir, "tsconfig.json")
		if err := os.WriteFile(tsConfigPath, []byte(tsConfig), 0644); err != nil {
			logger.WithField("error", err.Error()).
				Warn("Failed to write tsconfig.json to temporary directory")
		} else {
			logger.Info("Created tsconfig.json in temporary directory")
		}
	}

	// Read the original .yarnrc.yml, extract yarnPath, and remove the plugins section
	yarnrcSrc := "/app/.yarnrc.yml"
	yarnrcContent, err := os.ReadFile(yarnrcSrc)
	if err != nil {
		logger.WithField("error", err.Error()).Warn("Failed to read .yarnrc.yml")
		return "", nil // Return empty yarnPath but don't fail
	}

	var yarnPath string

	// Parse the YAML content
	var yarnConfig map[string]interface{}
	if err := yaml.Unmarshal(yarnrcContent, &yarnConfig); err != nil {
		logger.WithField("error", err.Error()).
			Warn("Failed to parse .yarnrc.yml")
		return "", nil // Return empty yarnPath but don't fail
	}

	// Extract yarnPath from the config
	if path, ok := yarnConfig["yarnPath"].(string); ok {
		yarnPath = path
		logger.WithField("yarnPath", yarnPath).Info("Found yarnPath in .yarnrc.yml")
	}

	// Remove the plugins section
	delete(yarnConfig, "plugins")

	// Marshal back to YAML
	modifiedYarnrc, err := yaml.Marshal(yarnConfig)
	if err != nil {
		logger.WithField("error", err.Error()).
			Warn("Failed to marshal modified .yarnrc.yml")
	} else {
		// Write the modified .yarnrc.yml to the temporary directory
		yarnrcDst := filepath.Join(workDir, ".yarnrc.yml")
		if err := os.WriteFile(yarnrcDst, modifiedYarnrc, 0644); err != nil {
			logger.WithField("error", err.Error()).
				Warn("Failed to write modified .yarnrc.yml to temporary directory")
		} else {
			logger.Info("Created modified .yarnrc.yml in temporary directory (plugins removed)")
		}
	}

	// Create .yarn directory in the temp directory and copy contents from /app/.yarn
	// Exclude install-state.gz which is environment-specific
	yarnSrcDir := "/app/.yarn"
	yarnDstDir := filepath.Join(workDir, ".yarn")
	if err := os.MkdirAll(yarnDstDir, 0755); err != nil {
		logger.WithField("error", err.Error()).
			Warn("Failed to create .yarn directory in temporary directory")
	} else if err := copyDir(yarnSrcDir, yarnDstDir, "install-state.gz"); err != nil {
		logger.WithField("error", err.Error()).
			Warn("Failed to copy .yarn directory to temporary directory")
	} else {
		logger.Info("Copied .yarn directory to temporary directory")
	}

	return yarnPath, nil
}

// InstallDependencies installs dependencies using the yarn queue
func (yi *YarnInstaller) InstallDependencies(ctx context.Context, workDir, specHash, yarnPath string, logger logger.Logger) error {
	// Create a job for the queue
	resultChan := make(chan error, 1)
	job := &YarnInstallJob{
		WorkDir:    workDir,
		SpecHash:   specHash,
		Logger:     logger,
		Context:    ctx,
		ResultChan: resultChan,
		YarnPath:   yarnPath,
	}

	// Submit to queue
	yi.queue.SubmitYarnInstall(job)

	// Wait for result
	select {
	case err := <-resultChan:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// CreatePackageJSON creates a package.json file with the specified dependencies
func (yi *YarnInstaller) CreatePackageJSON(workDir string, dependencies map[string]interface{}, logger logger.Logger) error {
	// Create simple package.json with dependencies
	packageJSON := map[string]interface{}{
		"name":         "xfuncjs-function",
		"type":         "module",
		"version":      "0.0.0",
		"private":      true,
		"dependencies": dependencies,
	}

	packageJSONBytes, err := json.MarshalIndent(packageJSON, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal package.json: %w", err)
	}

	packageJSONPath := filepath.Join(workDir, "package.json")
	if err := os.WriteFile(packageJSONPath, packageJSONBytes, 0644); err != nil {
		return fmt.Errorf("failed to write package.json to %s: %w", packageJSONPath, err)
	}

	logger.Info("Created package.json with workspace dependencies")
	return nil
}
