package grpc

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"time"

	fnv1 "github.com/crossplane/function-sdk-go/proto/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/socialgouv/crossplane-skyhook/pkg/logger"
	"github.com/socialgouv/crossplane-skyhook/pkg/node"
	"github.com/socialgouv/crossplane-skyhook/pkg/types"
)

// Server is the gRPC server for the Skyhook service
type Server struct {
	UnimplementedSkyhookServiceServer
	processManager *node.ProcessManager
	server         *grpc.Server
	logger         logger.Logger
	nodeServerPort int
}

// NewServer creates a new Skyhook gRPC server
func NewServer(processManager *node.ProcessManager, logger logger.Logger) *Server {
	return &Server{
		processManager: processManager,
		logger:         logger,
		nodeServerPort: 3000, // Default port
	}
}

// SetNodeServerPort sets the port for the Node.js HTTP server
func (s *Server) SetNodeServerPort(port int) {
	s.nodeServerPort = port
	s.processManager.SetNodeServerPort(port)
}

// SetNodeHealthCheckConfig sets the health check configuration for the Node.js HTTP server
func (s *Server) SetNodeHealthCheckConfig(wait, interval time.Duration) {
	s.processManager.SetHealthCheckWait(wait)
	s.processManager.SetHealthCheckInterval(interval)
}

// SetNodeRequestTimeout sets the request timeout for the Node.js HTTP server
func (s *Server) SetNodeRequestTimeout(timeout time.Duration) {
	s.processManager.SetRequestTimeout(timeout)
}

// Start starts the gRPC server on the specified address
func (s *Server) Start(address string, tlsEnabled bool, certFile, keyFile string) error {
	// Configure the Node.js process manager with the server port
	s.processManager.SetNodeServerPort(s.nodeServerPort)
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
	req := &fnv1.RunFunctionRequest{}
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

// Helper function to convert structpb.Struct to map[string]interface{}
func structpbToMap(s *structpb.Struct) map[string]interface{} {
	if s == nil || s.Fields == nil {
		return nil
	}

	result := make(map[string]interface{})
	for k, v := range s.Fields {
		result[k] = structpbValueToInterface(v)
	}
	return result
}

// Helper function to convert structpb.Value to interface{}
func structpbValueToInterface(v *structpb.Value) interface{} {
	switch v.Kind.(type) {
	case *structpb.Value_NullValue:
		return nil
	case *structpb.Value_NumberValue:
		return v.GetNumberValue()
	case *structpb.Value_StringValue:
		return v.GetStringValue()
	case *structpb.Value_BoolValue:
		return v.GetBoolValue()
	case *structpb.Value_StructValue:
		return structpbToMap(v.GetStructValue())
	case *structpb.Value_ListValue:
		list := v.GetListValue()
		result := make([]interface{}, len(list.Values))
		for i, item := range list.Values {
			result[i] = structpbValueToInterface(item)
		}
		return result
	default:
		return nil
	}
}

// runFunctionCrossplane implements the RunFunction method for the Crossplane FunctionRunnerService
func (s *Server) runFunctionCrossplane(ctx context.Context, req *fnv1.RunFunctionRequest) (*fnv1.RunFunctionResponse, error) {
	// Log request details
	s.logger.Debug("Crossplane FunctionRunnerService.RunFunction request received")

	// Convert the input to JSON
	inputBytes, err := json.Marshal(req.Input)
	if err != nil {
		s.logger.Errorf("Error marshaling input to JSON: %v", err)
		return &fnv1.RunFunctionResponse{
			Meta: &fnv1.ResponseMeta{},
			Results: []*fnv1.Result{
				{
					Severity: fnv1.Severity_SEVERITY_FATAL,
					Message:  fmt.Sprintf("Failed to marshal input to JSON: %v", err),
				},
			},
		}, nil
	}

	// s.logger.WithField("input_length", len(inputBytes)).Debug("Input received")
	// s.logger.WithField("raw_input", string(inputBytes)).Debug("Raw input")

	// Parse the input into a SkyhookInput
	var inputData map[string]interface{}
	if err := json.Unmarshal(inputBytes, &inputData); err != nil {
		s.logger.Errorf("Error parsing input JSON: %v", err)
		return &fnv1.RunFunctionResponse{
			Meta: &fnv1.ResponseMeta{},
			Results: []*fnv1.Result{
				{
					Severity: fnv1.Severity_SEVERITY_FATAL,
					Message:  fmt.Sprintf("Failed to parse input JSON: %v", err),
				},
			},
		}, nil
	}

	// Log the input data for debugging
	inputJSON, _ := json.MarshalIndent(inputData, "", "  ")
	s.logger.Debugf("Input data: %s", string(inputJSON))

	// Create a SkyhookInput from the input data
	skyhookInput := &types.SkyhookInput{}
	if err := json.Unmarshal(inputBytes, skyhookInput); err != nil {
		s.logger.Errorf("Error parsing input into SkyhookInput: %v", err)
		return &fnv1.RunFunctionResponse{
			Meta: &fnv1.ResponseMeta{},
			Results: []*fnv1.Result{
				{
					Severity: fnv1.Severity_SEVERITY_FATAL,
					Message:  fmt.Sprintf("Failed to parse input into SkyhookInput: %v", err),
				},
			},
		}, nil
	}

	// Validate the input
	if err := skyhookInput.Validate(); err != nil {
		s.logger.Errorf("Invalid input: %v", err)
		return &fnv1.RunFunctionResponse{
			Meta: &fnv1.ResponseMeta{},
			Results: []*fnv1.Result{
				{
					Severity: fnv1.Severity_SEVERITY_FATAL,
					Message:  fmt.Sprintf("Invalid input: %v", err),
				},
			},
		}, nil
	}

	// Extract the observed state from the RunFunctionRequest
	var enhancedInput map[string]interface{}

	// Create a structured input that includes both the original input and the observed state
	enhancedInput = map[string]interface{}{
		"input": inputData,
	}

	// Add observed state if available
	if req.Observed != nil {
		observed := make(map[string]interface{})

		// Add observed composite resource if available
		if req.Observed.Composite != nil && req.Observed.Composite.Resource != nil {
			compositeMap := structpbToMap(req.Observed.Composite.Resource)
			observed["composite"] = map[string]interface{}{
				"resource": compositeMap,
			}
		}

		// Add observed resources if available
		if len(req.Observed.Resources) > 0 {
			resources := make(map[string]interface{})
			for name, resource := range req.Observed.Resources {
				if resource != nil && resource.Resource != nil {
					resourceMap := structpbToMap(resource.Resource)
					resources[name] = map[string]interface{}{
						"resource": resourceMap,
					}
				}
			}
			observed["resources"] = resources
		}

		enhancedInput["observed"] = observed
	}

	// Convert the enhanced input to JSON
	enhancedInputJSON, err := json.Marshal(enhancedInput)
	if err != nil {
		s.logger.Errorf("Error marshaling enhanced input to JSON: %v", err)
		return &fnv1.RunFunctionResponse{
			Meta: &fnv1.ResponseMeta{},
			Results: []*fnv1.Result{
				{
					Severity: fnv1.Severity_SEVERITY_FATAL,
					Message:  fmt.Sprintf("Failed to marshal enhanced input to JSON: %v", err),
				},
			},
		}, nil
	}

	// Execute the function using the process manager with the enhanced input
	result, err := s.processManager.ExecuteFunction(ctx, skyhookInput, string(enhancedInputJSON))
	if err != nil {
		s.logger.Errorf("Error executing function: %v", err)
		return &fnv1.RunFunctionResponse{
			Meta: &fnv1.ResponseMeta{},
			Results: []*fnv1.Result{
				{
					Severity: fnv1.Severity_SEVERITY_FATAL,
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
		return &fnv1.RunFunctionResponse{
			Meta: &fnv1.ResponseMeta{},
			Results: []*fnv1.Result{
				{
					Severity: fnv1.Severity_SEVERITY_FATAL,
					Message:  fmt.Sprintf("Failed to parse Node.js response: %v", err),
				},
			},
		}, nil
	}

	if nodeResp.Error != nil {
		return &fnv1.RunFunctionResponse{
			Meta: &fnv1.ResponseMeta{},
			Results: []*fnv1.Result{
				{
					Severity: fnv1.Severity_SEVERITY_FATAL,
					Message:  nodeResp.Error.Message,
				},
			},
		}, nil
	}

	// Parse the JavaScript function's response to extract the resources
	var jsResponse struct {
		Resources map[string]struct {
			Resource json.RawMessage `json:"resource"`
		} `json:"resources"`
	}

	if err := json.Unmarshal(nodeResp.Result, &jsResponse); err != nil {
		s.logger.Errorf("Error parsing JavaScript function response: %v", err)
		return &fnv1.RunFunctionResponse{
			Meta: &fnv1.ResponseMeta{},
			Results: []*fnv1.Result{
				{
					Severity: fnv1.Severity_SEVERITY_FATAL,
					Message:  fmt.Sprintf("Failed to parse JavaScript function response: %v", err),
				},
			},
		}, nil
	}

	// Create a new State object
	state := &fnv1.State{
		Resources: make(map[string]*fnv1.Resource),
	}

	// Create a copy of the input struct
	compositeResource := proto.Clone(req.Input).(*structpb.Struct)

	// Remove the spec.source field if it exists
	if specValue, ok := compositeResource.Fields["spec"]; ok {
		if specStruct, ok := specValue.Kind.(*structpb.Value_StructValue); ok {
			delete(specStruct.StructValue.Fields, "source")
		}
	}

	// Set the cleaned composite resource
	state.Composite = &fnv1.Resource{
		Resource: compositeResource,
	}

	// Add the resources from the JavaScript function's response
	for name, resourceObj := range jsResponse.Resources {
		// Convert the JSON resource to a structpb.Struct
		var resourceMap map[string]interface{}
		if err := json.Unmarshal(resourceObj.Resource, &resourceMap); err != nil {
			s.logger.Errorf("Error unmarshaling resource %s: %v", name, err)
			continue
		}

		// Remove the namespace from the resource metadata if it exists
		// This prevents Crossplane from trying to add it to resourceRefs
		if metadata, ok := resourceMap["metadata"].(map[string]interface{}); ok {
			if _, ok := metadata["namespace"].(string); ok {
				// Remove the namespace from the resource metadata
				delete(metadata, "namespace")
			}
		}

		resourceStruct, err := structpb.NewStruct(resourceMap)
		if err != nil {
			s.logger.Errorf("Error converting resource %s to struct: %v", name, err)
			continue
		}

		// Create a resource without namespace in resourceRefs
		state.Resources[name] = &fnv1.Resource{
			Resource: resourceStruct,
		}
	}

	// Log the final state for debugging
	stateJSON, _ := json.MarshalIndent(state, "", "  ")
	s.logger.Debugf("Final state being returned to Crossplane: %s", string(stateJSON))

	// Return the result as a proper protobuf message
	response := &fnv1.RunFunctionResponse{
		Meta:    &fnv1.ResponseMeta{},
		Desired: state,
	}

	return response, nil
}
