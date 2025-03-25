package node

import (
	"time"
)

// ProcessManagerOption is a function that configures a ProcessManager
type ProcessManagerOption func(*ProcessManager)

// WithNodeServerPort sets the base port for the Node.js HTTP servers
func WithNodeServerPort(port int) ProcessManagerOption {
	return func(pm *ProcessManager) {
		pm.nodeServerPort = port
		pm.nextPort = port // Also update the next port to be assigned
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

// SetNodeServerPort sets the base port for the Node.js HTTP servers
func (pm *ProcessManager) SetNodeServerPort(port int) {
	pm.nodeServerPort = port
	pm.nextPort = port // Also update the next port to be assigned
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

// getNextPort returns the next available port and increments the counter
func (pm *ProcessManager) getNextPort() int {
	pm.portMutex.Lock()
	defer pm.portMutex.Unlock()

	port := pm.nextPort
	pm.nextPort++

	// If we've gone too high, wrap around to the base port + 1000
	// This gives a range of 1000 ports (e.g., 3000-3999)
	if pm.nextPort >= pm.nodeServerPort+1000 {
		pm.nextPort = pm.nodeServerPort
	}

	return port
}
