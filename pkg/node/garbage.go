package node

import (
	"os"
	"syscall"
	"time"

	"github.com/socialgouv/xfuncjs-server/pkg/logger"
)

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
				if err := info.Process.Process.Signal(syscall.SIGTERM); err != nil {
					processLogger.WithField("error", err.Error()).Warn("Failed to send SIGTERM to process")
				}
			}

			// Flush any buffered stderr data
			if stderrWriter, ok := info.Process.Stderr.(*logWriter); ok {
				stderrWriter.Flush()
				processLogger.Debug("Flushed stderr buffer for process")
			}

			// Kill the process if it doesn't exit gracefully
			if info.Process.Process != nil {
				processLogger.Debug("Sending SIGKILL to process")
				if err := info.Process.Process.Kill(); err != nil {
					processLogger.WithField("error", err.Error()).Warn("Failed to send SIGKILL to process")
				}
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
