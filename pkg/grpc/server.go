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
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/runtime/protoimpl"

	"github.com/socialgouv/crossplane-skyhook/pkg/logger"
	"github.com/socialgouv/crossplane-skyhook/pkg/node"
)

// CrossplaneFunctionRunnerRequest is the request message for the Crossplane FunctionRunnerService
// It implements the proto.Message interface required by gRPC
type CrossplaneFunctionRunnerRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// The input data for the function
	Input []byte `protobuf:"bytes,1,opt,name=input,proto3" json:"input,omitempty"`
}

func (x *CrossplaneFunctionRunnerRequest) Reset() {
	*x = CrossplaneFunctionRunnerRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_function_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *CrossplaneFunctionRunnerRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

// ProtoMessage implements the proto.Message interface
func (*CrossplaneFunctionRunnerRequest) ProtoMessage() {}

func (x *CrossplaneFunctionRunnerRequest) ProtoReflect() protoreflect.Message {
	mi := &file_function_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (x *CrossplaneFunctionRunnerRequest) GetInput() []byte {
	if x != nil {
		return x.Input
	}
	return nil
}

// CrossplaneFunctionRunnerResponse is the response message for the Crossplane FunctionRunnerService
// It implements the proto.Message interface required by gRPC
type CrossplaneFunctionRunnerResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// The output data from the function
	Output []byte `protobuf:"bytes,1,opt,name=output,proto3" json:"output,omitempty"`

	// Error information if execution failed
	Error *struct {
		Code       int32  `protobuf:"varint,1,opt,name=code,proto3" json:"code,omitempty"`
		Message    string `protobuf:"bytes,2,opt,name=message,proto3" json:"message,omitempty"`
		StackTrace string `protobuf:"bytes,3,opt,name=stack_trace,json=stackTrace,proto3" json:"stack_trace,omitempty"`
	} `protobuf:"bytes,2,opt,name=error,proto3" json:"error,omitempty"`
}

func (x *CrossplaneFunctionRunnerResponse) Reset() {
	*x = CrossplaneFunctionRunnerResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_function_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *CrossplaneFunctionRunnerResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

// ProtoMessage implements the proto.Message interface
func (*CrossplaneFunctionRunnerResponse) ProtoMessage() {}

func (x *CrossplaneFunctionRunnerResponse) ProtoReflect() protoreflect.Message {
	mi := &file_function_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (x *CrossplaneFunctionRunnerResponse) GetOutput() []byte {
	if x != nil {
		return x.Output
	}
	return nil
}

// Dummy variable to avoid unused import errors
var (
	file_function_proto_msgTypes = make([]protoimpl.MessageInfo, 2)
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

	// Create a proper protobuf message to decode the request
	req := &CrossplaneFunctionRunnerRequest{}

	// Decode the request
	if err := dec(req); err != nil {
		s.logger.Errorf("Error decoding request: %v", err)
		return nil, status.Errorf(codes.Internal, "Error decoding request: %v", err)
	}

	s.logger.Infof("Received request with input length: %d", len(req.Input))

	// If the input is empty, use a default input
	input := req.Input
	if len(input) == 0 {
		s.logger.Warn("Empty input received, using default input")
		input = []byte("{}")
	}

	// Process the request
	result, err := s.runFunctionCrossplane(ctx, input)
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
func (s *Server) runFunctionCrossplane(ctx context.Context, input []byte) (interface{}, error) {
	// Log request details
	s.logger.Debug("Crossplane FunctionRunnerService.RunFunction request received")

	// Extract the input data
	inputJSON := string(input)
	s.logger.WithField("input_length", len(inputJSON)).Debug("Input JSON received")

	// Parse the input to extract the code
	var inputStruct struct {
		Observed struct {
			Composite struct {
				Resource struct {
				} `json:"resource"`
			} `json:"composite"`
		} `json:"observed"`
		Desired struct {
		} `json:"desired"`
	}

	if err := json.Unmarshal(input, &inputStruct); err != nil {
		s.logger.Errorf("Error parsing input JSON: %v", err)
		return &CrossplaneFunctionRunnerResponse{
			Error: &struct {
				Code       int32  `protobuf:"varint,1,opt,name=code,proto3" json:"code,omitempty"`
				Message    string `protobuf:"bytes,2,opt,name=message,proto3" json:"message,omitempty"`
				StackTrace string `protobuf:"bytes,3,opt,name=stack_trace,json=stackTrace,proto3" json:"stack_trace,omitempty"`
			}{
				Code:       int32(codes.InvalidArgument),
				Message:    fmt.Sprintf("Failed to parse input JSON: %v", err),
				StackTrace: "",
			},
		}, nil
	}

	// Extract the code from the input
	// For Crossplane functions, the code is typically provided in the composition
	// We need to extract it from the input data
	var code string
	var extractedInputJSON string

	// Try to extract the code from the input
	// This is a simplified approach - in a real implementation, you would need to
	// parse the Crossplane function input format to extract the code
	var inputData map[string]interface{}
	if err := json.Unmarshal(input, &inputData); err != nil {
		s.logger.Errorf("Error parsing input JSON: %v", err)
		return &CrossplaneFunctionRunnerResponse{
			Error: &struct {
				Code       int32  `protobuf:"varint,1,opt,name=code,proto3" json:"code,omitempty"`
				Message    string `protobuf:"bytes,2,opt,name=message,proto3" json:"message,omitempty"`
				StackTrace string `protobuf:"bytes,3,opt,name=stack_trace,json=stackTrace,proto3" json:"stack_trace,omitempty"`
			}{
				Code:       int32(codes.InvalidArgument),
				Message:    fmt.Sprintf("Failed to parse input JSON: %v", err),
				StackTrace: "",
			},
		}, nil
	}

	// Look for the code in the input
	// In a real implementation, you would need to know the exact path to the code
	// For now, we'll look for a "source" field with an "inline" field
	if inputMap, ok := inputData["input"].(map[string]interface{}); ok {
		if specMap, ok := inputMap["spec"].(map[string]interface{}); ok {
			if sourceMap, ok := specMap["source"].(map[string]interface{}); ok {
				if inlineCode, ok := sourceMap["inline"].(string); ok {
					code = inlineCode
					// Remove the code from the input to avoid duplication
					delete(sourceMap, "inline")
					// Re-serialize the input without the code
					newInputJSON, err := json.Marshal(inputData)
					if err != nil {
						s.logger.Errorf("Error re-serializing input JSON: %v", err)
						return &CrossplaneFunctionRunnerResponse{
							Error: &struct {
								Code       int32  `protobuf:"varint,1,opt,name=code,proto3" json:"code,omitempty"`
								Message    string `protobuf:"bytes,2,opt,name=message,proto3" json:"message,omitempty"`
								StackTrace string `protobuf:"bytes,3,opt,name=stack_trace,json=stackTrace,proto3" json:"stack_trace,omitempty"`
							}{
								Code:       int32(codes.Internal),
								Message:    fmt.Sprintf("Failed to re-serialize input JSON: %v", err),
								StackTrace: "",
							},
						}, nil
					}
					extractedInputJSON = string(newInputJSON)
				}
			}
		}
	}

	if code == "" {
		s.logger.Error("No code found in the input")
		return &CrossplaneFunctionRunnerResponse{
			Error: &struct {
				Code       int32  `protobuf:"varint,1,opt,name=code,proto3" json:"code,omitempty"`
				Message    string `protobuf:"bytes,2,opt,name=message,proto3" json:"message,omitempty"`
				StackTrace string `protobuf:"bytes,3,opt,name=stack_trace,json=stackTrace,proto3" json:"stack_trace,omitempty"`
			}{
				Code:       int32(codes.InvalidArgument),
				Message:    "No code found in the input",
				StackTrace: "",
			},
		}, nil
	}

	// Execute the function using the process manager
	result, err := s.processManager.ExecuteFunction(ctx, code, extractedInputJSON)
	if err != nil {
		s.logger.Errorf("Error executing function: %v", err)
		return &CrossplaneFunctionRunnerResponse{
			Error: &struct {
				Code       int32  `protobuf:"varint,1,opt,name=code,proto3" json:"code,omitempty"`
				Message    string `protobuf:"bytes,2,opt,name=message,proto3" json:"message,omitempty"`
				StackTrace string `protobuf:"bytes,3,opt,name=stack_trace,json=stackTrace,proto3" json:"stack_trace,omitempty"`
			}{
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
		return &CrossplaneFunctionRunnerResponse{
			Error: &struct {
				Code       int32  `protobuf:"varint,1,opt,name=code,proto3" json:"code,omitempty"`
				Message    string `protobuf:"bytes,2,opt,name=message,proto3" json:"message,omitempty"`
				StackTrace string `protobuf:"bytes,3,opt,name=stack_trace,json=stackTrace,proto3" json:"stack_trace,omitempty"`
			}{
				Code:       int32(codes.Internal),
				Message:    fmt.Sprintf("Failed to parse Node.js response: %v", err),
				StackTrace: "",
			},
		}, nil
	}

	if nodeResp.Error != nil {
		return &CrossplaneFunctionRunnerResponse{
			Error: &struct {
				Code       int32  `protobuf:"varint,1,opt,name=code,proto3" json:"code,omitempty"`
				Message    string `protobuf:"bytes,2,opt,name=message,proto3" json:"message,omitempty"`
				StackTrace string `protobuf:"bytes,3,opt,name=stack_trace,json=stackTrace,proto3" json:"stack_trace,omitempty"`
			}{
				Code:       int32(nodeResp.Error.Code),
				Message:    nodeResp.Error.Message,
				StackTrace: nodeResp.Error.Stack,
			},
		}, nil
	}

	// Return the result as a proper protobuf message
	return &CrossplaneFunctionRunnerResponse{
		Output: []byte(nodeResp.Result),
	}, nil
}
