package context

import (
	"context"

	"github.com/socialgouv/xfuncjs-server/pkg/types"
)

type contextKey string

const (
	// ResourceInfoKey is the key for resource information in the context
	resourceInfoKey contextKey = "resource-info"
	// RequestIDKey is the key for request ID in the context
	requestIDKey contextKey = "request-id"
)

// WithResourceInfo adds resource information to the context
func WithResourceInfo(ctx context.Context, resourceInfo *types.ResourceInfo) context.Context {
	if resourceInfo == nil {
		return ctx
	}
	return context.WithValue(ctx, resourceInfoKey, resourceInfo)
}

// GetResourceInfo retrieves resource information from the context
func GetResourceInfo(ctx context.Context) *types.ResourceInfo {
	if ctx == nil {
		return nil
	}
	value := ctx.Value(resourceInfoKey)
	if value == nil {
		return nil
	}
	resourceInfo, ok := value.(*types.ResourceInfo)
	if !ok {
		return nil
	}
	return resourceInfo
}

// WithRequestID adds a request ID to the context
func WithRequestID(ctx context.Context, requestID string) context.Context {
	if requestID == "" {
		return ctx
	}
	return context.WithValue(ctx, requestIDKey, requestID)
}

// GetRequestID retrieves the request ID from the context
func GetRequestID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	value := ctx.Value(requestIDKey)
	if value == nil {
		return ""
	}
	requestID, ok := value.(string)
	if !ok {
		return ""
	}
	return requestID
}
