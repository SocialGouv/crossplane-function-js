package node

import (
	"fmt"
	"net"
	"time"
)

// ProcessManagerOption is a function that configures a ProcessManager
type ProcessManagerOption func(*ProcessManager)

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

// WithYarnQueue sets the yarn installer with queue support
func WithYarnQueue(maxConcurrentYarnInstalls int) ProcessManagerOption {
	return func(pm *ProcessManager) {
		// Initialize the global yarn queue if not already initialized
		if GetYarnQueue() == nil {
			InitializeYarnQueue(maxConcurrentYarnInstalls, pm.logger)
		}
		pm.yarnInstaller = NewYarnInstaller(GetYarnQueue(), pm.logger)
		pm.dependencyResolver = NewDependencyResolver(pm.logger)
	}
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

// getAvailablePort asks the OS for a free ephemeral port.
func (pm *ProcessManager) getAvailablePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, fmt.Errorf("failed to find available port: %w", err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}
