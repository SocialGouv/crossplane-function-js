package logger

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	pkgcontext "github.com/socialgouv/crossplane-skyhook/pkg/context"
	pkgerrors "github.com/socialgouv/crossplane-skyhook/pkg/errors"
	"github.com/socialgouv/crossplane-skyhook/pkg/types"
)

// UnaryServerInterceptor returns a new unary server interceptor that logs requests
func UnaryServerInterceptor(logger Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		startTime := time.Now()

		// Extract method name from the full method
		method := path.Base(info.FullMethod)

		// Generate a request ID if not present
		requestID := getOrGenerateRequestID(ctx)
		ctx = pkgcontext.WithRequestID(ctx, requestID)

		// Extract resource information from the request
		resourceInfo := extractResourceInfo(req)
		if resourceInfo != nil {
			ctx = pkgcontext.WithResourceInfo(ctx, resourceInfo)
		}

		// Create a context-aware logger
		ctxLogger := LoggerFromContext(ctx, logger)

		// Add method information
		ctxLogger = ctxLogger.WithFields(map[string]interface{}{
			FieldMethod:       method,
			FieldComponent:    "grpc",
			FieldOperation:    method,
			"grpc.start_time": startTime.Format(time.RFC3339),
		})

		ctxLogger.Info("Received request")

		// Process the request
		resp, err := handler(ctx, req)

		// Calculate duration
		duration := time.Since(startTime)

		// Determine status code
		statusCode := codes.OK
		if err != nil {
			statusCode = status.Code(err)

			// Log error with context
			errFields := map[string]interface{}{
				FieldDuration: float64(duration.Microseconds()) / 1000.0,
				FieldStatus:   statusCode.String(),
			}

			// Add error fields if it's a contextual error
			for k, v := range pkgerrors.GetFields(err) {
				errFields[k] = v
			}

			ctxLogger.WithFields(errFields).Error("Request failed")

			// Return the original error
			return resp, err
		}

		// Log successful response
		ctxLogger.WithFields(map[string]interface{}{
			FieldDuration: float64(duration.Microseconds()) / 1000.0,
			FieldStatus:   statusCode.String(),
		}).Info("Request completed successfully")

		return resp, nil
	}
}

// getOrGenerateRequestID gets the request ID from the context or generates a new one
func getOrGenerateRequestID(ctx context.Context) string {
	// Try to get the request ID from the metadata
	md, ok := metadata.FromIncomingContext(ctx)
	if ok {
		if values := md.Get("x-request-id"); len(values) > 0 {
			return values[0]
		}
	}

	// Generate a new request ID
	return uuid.New().String()
}

// extractResourceInfo extracts resource information from the request
func extractResourceInfo(req interface{}) *types.ResourceInfo {
	// Handle different request types
	switch r := req.(type) {
	case proto.Message:
		// For protobuf messages, try to extract resource information
		return extractResourceInfoFromProto(r)
	default:
		// For other types, we can't extract resource information
		return nil
	}
}

// extractResourceInfoFromProto extracts resource information from a protobuf message
func extractResourceInfoFromProto(msg proto.Message) *types.ResourceInfo {
	// Convert the protobuf message to JSON
	marshaler := protojson.MarshalOptions{
		UseProtoNames: true,
	}

	jsonBytes, err := marshaler.Marshal(msg)
	if err != nil {
		return nil
	}

	// Parse the JSON to extract resource information
	var data map[string]interface{}
	if err := json.Unmarshal(jsonBytes, &data); err != nil {
		return nil
	}

	// Try to extract resource information from common fields
	resourceInfo := &types.ResourceInfo{}

	// Check for Crossplane composite resource
	if input, ok := data["input"].(map[string]interface{}); ok {
		// Try to extract from apiVersion and kind
		if apiVersion, ok := input["apiVersion"].(string); ok {
			resourceInfo.Version = apiVersion
		}

		if kind, ok := input["kind"].(string); ok {
			resourceInfo.Kind = kind
		}

		// Try to extract from metadata
		if metadata, ok := input["metadata"].(map[string]interface{}); ok {
			if name, ok := metadata["name"].(string); ok {
				resourceInfo.Name = name
			}

			if namespace, ok := metadata["namespace"].(string); ok {
				resourceInfo.Namespace = namespace
			}
		}
	}

	// Check for observed resources
	if observed, ok := data["observed"].(map[string]interface{}); ok {
		if composite, ok := observed["composite"].(map[string]interface{}); ok {
			if resource, ok := composite["resource"].(map[string]interface{}); ok {
				// Try to extract from apiVersion and kind
				if apiVersion, ok := resource["apiVersion"].(string); ok && resourceInfo.Version == "" {
					resourceInfo.Version = apiVersion
				}

				if kind, ok := resource["kind"].(string); ok && resourceInfo.Kind == "" {
					resourceInfo.Kind = kind
				}

				// Try to extract from metadata
				if metadata, ok := resource["metadata"].(map[string]interface{}); ok {
					if name, ok := metadata["name"].(string); ok && resourceInfo.Name == "" {
						resourceInfo.Name = name
					}

					if namespace, ok := metadata["namespace"].(string); ok && resourceInfo.Namespace == "" {
						resourceInfo.Namespace = namespace
					}
				}
			}
		}
	}

	// If we couldn't extract any resource information, return nil
	if resourceInfo.Version == "" && resourceInfo.Kind == "" && resourceInfo.Name == "" && resourceInfo.Namespace == "" {
		return nil
	}

	return resourceInfo
}

// LogRequestDetails logs the details of a request if appropriate
func LogRequestDetails(logger Logger, method string, req interface{}) {
	// For RunFunction, we don't want to log the entire code payload as it could be large
	// Instead, log a summary or just the fact that a request was received
	switch method {
	case "RunFunction":
		if runReq, ok := req.(interface{ GetCode() string }); ok {
			codeLen := len(runReq.GetCode())
			logger.WithFields(map[string]interface{}{
				FieldCodeLen: codeLen,
				FieldMethod:  method,
			}).Debug("Request received")
		}
	default:
		// For other methods, we might log more details
		logger.WithFields(map[string]interface{}{
			"request":   fmt.Sprintf("%+v", req),
			FieldMethod: method,
		}).Debug("Request details")
	}
}
