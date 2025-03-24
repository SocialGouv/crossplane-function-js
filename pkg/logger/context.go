package logger

import (
	"context"

	pkgcontext "github.com/socialgouv/crossplane-skyhook/pkg/context"
)

// LoggerFromContext creates a logger with context information
func LoggerFromContext(ctx context.Context, baseLogger Logger) Logger {
	if ctx == nil {
		return baseLogger
	}

	// Add request ID if available
	requestID := pkgcontext.GetRequestID(ctx)
	if requestID != "" {
		baseLogger = baseLogger.WithField(FieldRequestID, requestID)
	}

	// Add resource information if available
	resourceInfo := pkgcontext.GetResourceInfo(ctx)
	if resourceInfo != nil {
		baseLogger = WithResource(baseLogger, resourceInfo)
	}

	return baseLogger
}
