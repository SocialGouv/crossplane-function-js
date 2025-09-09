package grpc

import (
	"fmt"
	"net"
	"time"

	fnv1 "github.com/crossplane/function-sdk-go/proto/v1"
	"google.golang.org/grpc"

	"github.com/socialgouv/xfuncjs-server/pkg/logger"
	"github.com/socialgouv/xfuncjs-server/pkg/node"
)

// Server is the gRPC server for the XFuncJS service
type Server struct {
	function *Function
	server   *grpc.Server
	logger   logger.Logger
}

// NewServer creates a new XFuncJS gRPC server
func NewServer(processManager *node.ProcessManager, logger logger.Logger) *Server {
	return &Server{
		function: NewFunction(processManager, logger),
		logger:   logger,
	}
}

// SetNodeHealthCheckConfig sets the health check configuration for the Node.js HTTP server
func (s *Server) SetNodeHealthCheckConfig(wait, interval time.Duration) {
	s.function.SetNodeHealthCheckConfig(wait, interval)
}

// SetNodeRequestTimeout sets the request timeout for the Node.js HTTP server
func (s *Server) SetNodeRequestTimeout(timeout time.Duration) {
	s.function.SetNodeRequestTimeout(timeout)
}

// Start starts the gRPC server on the specified address
func (s *Server) Start(address string, tlsEnabled bool, certFile, keyFile string) error {
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", address, err)
	}

	// Create server options
	var opts []grpc.ServerOption
	opts = append(opts, grpc.UnaryInterceptor(logger.UnaryServerInterceptor(s.logger)))

	// Add TLS credentials if enabled
	if tlsEnabled {
		creds, err := loadTLSCredentials(certFile, keyFile)
		if err != nil {
			return fmt.Errorf("failed to load TLS credentials: %w", err)
		}
		opts = append(opts, grpc.Creds(creds))
	}

	// Create a new gRPC server with the options
	s.server = grpc.NewServer(opts...)

	// Register the Crossplane FunctionRunnerService
	s.logger.Info("Registering Crossplane FunctionRunnerService")
	fnv1.RegisterFunctionRunnerServiceServer(s.server, s.function)

	s.logger.Infof("Starting gRPC server on %s (TLS: %v)", address, tlsEnabled)
	return s.server.Serve(listener)
}

// Stop stops the gRPC server
func (s *Server) Stop() {
	if s.server != nil {
		s.server.GracefulStop()
	}
}
