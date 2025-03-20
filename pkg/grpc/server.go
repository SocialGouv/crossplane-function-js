package grpc

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"

	fnv1beta1 "github.com/crossplane/function-sdk-go/proto/v1beta1"
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

	// Register the Crossplane FunctionRunnerService
	// We're using a simpler approach by just registering the service name
	// and implementing the handler directly
	s.logger.Info("Registering Crossplane FunctionRunnerService")

	// Create a service description for the Crossplane FunctionRunnerService
	serviceDesc := &grpc.ServiceDesc{
		ServiceName: "apiextensions.fn.proto.v1beta1.FunctionRunnerService",
		HandlerType: (*interface{})(nil),
		Methods: []grpc.MethodDesc{
			{
				MethodName: "RunFunction",
				Handler:    s.runFunctionHandler,
			},
		},
		Streams:  []grpc.StreamDesc{},
		Metadata: "function.proto",
	}

	// Register the service
	s.server.RegisterService(serviceDesc, s)

	s.logger.Infof("Starting gRPC server on %s (TLS: %v)", address, tlsEnabled)
	return s.server.Serve(listener)
}

// runFunctionHandler is the handler for the RunFunction method of the FunctionRunnerService
func (s *Server) runFunctionHandler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	s.logger.Info("Handling Crossplane FunctionRunnerService.RunFunction request")

	// Use the Crossplane Function SDK to decode the request
	req := &fnv1beta1.RunFunctionRequest{}
	if err := dec(req); err != nil {
		s.logger.Errorf("Error decoding request: %v", err)
		return nil, status.Errorf(codes.Internal, "Error decoding request: %v", err)
	}

	s.logger.Info("Received Crossplane function request")

	// Process the request
	result, err := s.runFunctionCrossplane(ctx, req)
	if err != nil {
		return nil, err
	}

	// Return the result
	return result, nil
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

// runFunctionCrossplane implements the RunFunction method for the Crossplane FunctionRunnerService
func (s *Server) runFunctionCrossplane(ctx context.Context, req *fnv1beta1.RunFunctionRequest) (*fnv1beta1.RunFunctionResponse, error) {
	// Log request details
	s.logger.Debug("Crossplane FunctionRunnerService.RunFunction request received")

	// Convert the input to JSON
	inputBytes, err := json.Marshal(req.Input)
	if err != nil {
		s.logger.Errorf("Error marshaling input to JSON: %v", err)
		return &fnv1beta1.RunFunctionResponse{
			Meta: &fnv1beta1.ResponseMeta{},
			Results: []*fnv1beta1.Result{
				{
					Severity: fnv1beta1.Severity_SEVERITY_FATAL,
					Message:  fmt.Sprintf("Failed to marshal input to JSON: %v", err),
				},
			},
		}, nil
	}

	s.logger.WithField("input_length", len(inputBytes)).Debug("Input received")
	s.logger.WithField("raw_input", string(inputBytes)).Debug("Raw input")

	// Extract the code from the input
	// For Crossplane functions, the code is typically provided in the composition
	// We need to extract it from the input data
	var code string

	// Try to extract the code from the input
	// This is a simplified approach - in a real implementation, you would need to
	// parse the Crossplane function input format to extract the code
	var inputData map[string]interface{}
	if err := json.Unmarshal(inputBytes, &inputData); err != nil {
		s.logger.Errorf("Error parsing input JSON: %v", err)
		return &fnv1beta1.RunFunctionResponse{
			Meta: &fnv1beta1.ResponseMeta{},
			Results: []*fnv1beta1.Result{
				{
					Severity: fnv1beta1.Severity_SEVERITY_FATAL,
					Message:  fmt.Sprintf("Failed to parse input JSON: %v", err),
				},
			},
		}, nil
	}

	// Log the input data for debugging
	inputJSON, _ := json.MarshalIndent(inputData, "", "  ")
	s.logger.Infof("Input data: %s", string(inputJSON))

	// Look for the code in the input
	// In a real implementation, you would need to know the exact path to the code
	// For now, we'll look for a "source" field with an "inline" field
	if specMap, ok := inputData["spec"].(map[string]interface{}); ok {
		if sourceMap, ok := specMap["source"].(map[string]interface{}); ok {
			if inlineCode, ok := sourceMap["inline"].(string); ok {
				code = inlineCode
				// We're not removing the code from the input anymore
				// This ensures the JavaScript code receives the expected input structure
			}
		}
	}

	if code == "" {
		s.logger.Error("No code found in the input")
		return &fnv1beta1.RunFunctionResponse{
			Meta: &fnv1beta1.ResponseMeta{},
			Results: []*fnv1beta1.Result{
				{
					Severity: fnv1beta1.Severity_SEVERITY_FATAL,
					Message:  "No code found in the input",
				},
			},
		}, nil
	}

	// Execute the function using the process manager with the original input
	// This preserves the structure that the JavaScript code expects
	result, err := s.processManager.ExecuteFunction(ctx, code, string(inputBytes))
	if err != nil {
		s.logger.Errorf("Error executing function: %v", err)
		return &fnv1beta1.RunFunctionResponse{
			Meta: &fnv1beta1.ResponseMeta{},
			Results: []*fnv1beta1.Result{
				{
					Severity: fnv1beta1.Severity_SEVERITY_FATAL,
					Message:  err.Error(),
				},
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
		return &fnv1beta1.RunFunctionResponse{
			Meta: &fnv1beta1.ResponseMeta{},
			Results: []*fnv1beta1.Result{
				{
					Severity: fnv1beta1.Severity_SEVERITY_FATAL,
					Message:  fmt.Sprintf("Failed to parse Node.js response: %v", err),
				},
			},
		}, nil
	}

	if nodeResp.Error != nil {
		return &fnv1beta1.RunFunctionResponse{
			Meta: &fnv1beta1.ResponseMeta{},
			Results: []*fnv1beta1.Result{
				{
					Severity: fnv1beta1.Severity_SEVERITY_FATAL,
					Message:  nodeResp.Error.Message,
				},
			},
		}, nil
	}

	// Create a new State object
	state := &fnv1beta1.State{
		Composite: &fnv1beta1.Resource{
			Resource: req.Input,
		},
		Resources: make(map[string]*fnv1beta1.Resource),
	}

	// Return the result as a proper protobuf message
	return &fnv1beta1.RunFunctionResponse{
		Meta:    &fnv1beta1.ResponseMeta{},
		Desired: state,
	}, nil
}
