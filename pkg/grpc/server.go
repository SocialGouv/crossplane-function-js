package grpc

import (
	"context"
	"encoding/json"
	"fmt"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
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
func (s *Server) Start(address string) error {
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", address, err)
	}

	// Create a new gRPC server with the logging interceptor
	s.server = grpc.NewServer(
		grpc.UnaryInterceptor(logger.UnaryServerInterceptor(s.logger)),
	)
	RegisterSkyhookServiceServer(s.server, s)

	s.logger.Infof("Starting gRPC server on %s", address)
	return s.server.Serve(listener)
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
