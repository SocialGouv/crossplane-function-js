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
	logCrossplaneIO bool
}

// NewFunction creates a new Function
func NewFunction(processManager *node.ProcessManager, logger logger.Logger) *Function {
	return &Function{
		processManager: processManager,
		logger:         logger,
	}
}

// SetLogCrossplaneIO enables/disables logging of full Crossplane RunFunction
// request/response payloads (redacted) at DEBUG level.
func (f *Function) SetLogCrossplaneIO(enabled bool) {
	f.logCrossplaneIO = enabled
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
