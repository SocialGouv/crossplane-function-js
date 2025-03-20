package grpc

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"

	"github.com/socialgouv/crossplane-skyhook/pkg/logger"
	"github.com/socialgouv/crossplane-skyhook/pkg/node"
)

// Server is the gRPC server for the Skyhook service
type Server struct {
	UnimplementedSkyhookServiceServer
	processManager *node.ProcessManager
	server         *grpc.Server
	logger         logger.Logger
}

// NewServer creates a new Skyhook gRPC server
func NewServer(processManager *node.ProcessManager, logger logger.Logger) *Server {
	return &Server{
		processManager: processManager,
		logger:         logger,
	}
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
		creds, err := s.loadTLSCredentials(certFile, keyFile)
		if err != nil {
			return fmt.Errorf("failed to load TLS credentials: %w", err)
		}
		opts = append(opts, grpc.Creds(creds))
	}

	// Create a new gRPC server with the options
	s.server = grpc.NewServer(opts...)
	RegisterSkyhookServiceServer(s.server, s)

	s.logger.Infof("Starting gRPC server on %s (TLS: %v)", address, tlsEnabled)
	return s.server.Serve(listener)
}

// loadTLSCredentials loads TLS credentials from certificate and key files
func (s *Server) loadTLSCredentials(certFile, keyFile string) (credentials.TransportCredentials, error) {
	// Load server's certificate and private key
	serverCert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load server certificate and key: %w", err)
	}

	// Create the credentials and return it
	config := &tls.Config{
		Certificates: []tls.Certificate{serverCert},
		ClientAuth:   tls.NoClientCert,
	}

	return credentials.NewTLS(config), nil
}

// Stop stops the gRPC server
func (s *Server) Stop() {
	if s.server != nil {
		s.server.GracefulStop()
	}
}

// RunFunction implements the RunFunction RPC method
func (s *Server) RunFunction(ctx context.Context, req *RunFunctionRequest) (*RunFunctionResponse, error) {
	if req.Code == "" {
		return nil, status.Error(codes.InvalidArgument, "code is required")
	}

	// Log request details (code length only, not the full code)
	s.logger.WithField("code_length", len(req.Code)).Debug("RunFunction request received")

	// Execute the function using the process manager
	result, err := s.processManager.ExecuteFunction(ctx, req.Code, req.InputJson)
	if err != nil {
		s.logger.Errorf("Error executing function: %v", err)
		return &RunFunctionResponse{
			Error: &ErrorInfo{
				Code:       int32(codes.Internal),
				Message:    err.Error(),
				StackTrace: "",
			},
		}, nil
	}

	// Parse the result
	var nodeResp struct {
		Result json.RawMessage `json:"result,omitempty"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
			Stack   string `json:"stack,omitempty"`
		} `json:"error,omitempty"`
	}

	if err := json.Unmarshal([]byte(result), &nodeResp); err != nil {
		s.logger.Errorf("Error parsing Node.js response: %v", err)
		return &RunFunctionResponse{
			Error: &ErrorInfo{
				Code:       int32(codes.Internal),
				Message:    fmt.Sprintf("Failed to parse Node.js response: %v", err),
				StackTrace: "",
			},
		}, nil
	}

	response := &RunFunctionResponse{}

	if nodeResp.Error != nil {
		response.Error = &ErrorInfo{
			Code:       int32(nodeResp.Error.Code),
			Message:    nodeResp.Error.Message,
			StackTrace: nodeResp.Error.Stack,
		}
		s.logger.Errorf("Node.js execution error: %s", nodeResp.Error.Message)
	} else {
		response.OutputJson = string(nodeResp.Result)
	}

	return response, nil
}
