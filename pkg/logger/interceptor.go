package logger

import (
	"context"
	"fmt"
	"path"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// UnaryServerInterceptor returns a new unary server interceptor that logs requests
func UnaryServerInterceptor(logger Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		startTime := time.Now()

		// Extract method name from the full method
		method := path.Base(info.FullMethod)

		// Log request
		reqLogger := logger.WithFields(map[string]interface{}{
			"grpc.method":     method,
			"grpc.start_time": startTime.Format(time.RFC3339),
		})

		reqLogger.Infof("Received request: %s", method)

		// Process the request
		resp, err := handler(ctx, req)

		// Calculate duration
		duration := time.Since(startTime)

		// Determine status code
		statusCode := codes.OK
		if err != nil {
			statusCode = status.Code(err)
		}

		// Log response
		reqLogger.WithFields(map[string]interface{}{
			"grpc.duration_ms": float64(duration.Microseconds()) / 1000.0,
			"grpc.status_code": statusCode.String(),
		}).Infof("Completed request: %s", method)

		return resp, err
	}
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
				"code_length": codeLen,
			}).Debug("RunFunction request received")
		}
	default:
		// For other methods, we might log more details
		logger.WithField("request", fmt.Sprintf("%+v", req)).Debug("Request details")
	}
}
