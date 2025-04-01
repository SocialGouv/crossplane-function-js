package node

import (
	"context"
	"os"
	"time"

	"github.com/socialgouv/xfuncjs-server/pkg/logger"
)

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

	// Check if the process is responsive by sending a health check request
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := process.Client.CheckReady(ctx); err != nil {
		healthLogger.WithField(logger.FieldError, err.Error()).
			Warn("Process is not healthy: health check failed")
		return false
	}

	healthLogger.Debug("Process is healthy")
	return true
}

// restartProcess restarts a process
func (pm *ProcessManager) restartProcess(process *ProcessInfo, hash string) {
	// Create a logger with process information
	restartLogger := pm.logger.WithField(logger.FieldComponent, "node")
	restartLogger = restartLogger.WithField(logger.FieldOperation, "restart")
	restartLogger = restartLogger.WithField(logger.FieldCodeHash, hash[:8])

	if process.Process != nil && process.Process.Process != nil {
		restartLogger = restartLogger.WithField(logger.FieldPID, process.Process.Process.Pid)
	}

	restartLogger = restartLogger.WithField(logger.FieldPort, process.Port)

	// Kill the process if it's still running
	if process.Process != nil && process.Process.Process != nil {
		restartLogger.Info("Killing process")
		if err := process.Process.Process.Kill(); err != nil {
			restartLogger.WithField("error", err.Error()).Warn("Failed to kill process")
		}
	}

	// Clean up the temporary directory if it exists
	if process.TempDirPath != "" {
		restartLogger.Info("Removing temporary directory")
		if err := os.RemoveAll(process.TempDirPath); err != nil {
			restartLogger.WithField(logger.FieldError, err.Error()).
				Warn("Failed to remove temporary directory")
		}
	}

	// Remove the process from the map
	pm.lock.Lock()
	for id, p := range pm.processes {
		if p == process {
			delete(pm.processes, id)
			break
		}
	}
	pm.lock.Unlock()

	restartLogger.Info("Process successfully restarted")
}
