package grpc

import (
	"time"

	fnv1 "github.com/crossplane/function-sdk-go/proto/v1"
	"github.com/socialgouv/xfuncjs-server/pkg/logger"
	"github.com/socialgouv/xfuncjs-server/pkg/node"
)

// Function implements the Crossplane Function interface
type Function struct {
	fnv1.UnimplementedFunctionRunnerServiceServer
	processManager *node.ProcessManager
	logger         logger.Logger
	nodeServerPort int
}

// NewFunction creates a new Function
func NewFunction(processManager *node.ProcessManager, logger logger.Logger) *Function {
	return &Function{
		processManager: processManager,
		logger:         logger,
		nodeServerPort: 3000, // Default port
	}
}

// SetNodeServerPort sets the port for the Node.js HTTP server
func (f *Function) SetNodeServerPort(port int) {
	f.nodeServerPort = port
	f.processManager.SetNodeServerPort(port)
}

// SetNodeHealthCheckConfig sets the health check configuration for the Node.js HTTP server
func (f *Function) SetNodeHealthCheckConfig(wait, interval time.Duration) {
	f.processManager.SetHealthCheckWait(wait)
	f.processManager.SetHealthCheckInterval(interval)
}

// SetNodeRequestTimeout sets the request timeout for the Node.js HTTP server
func (f *Function) SetNodeRequestTimeout(timeout time.Duration) {
	f.processManager.SetRequestTimeout(timeout)
}
