package logger

import (
	"github.com/socialgouv/xfuncjs-server/pkg/types"
)

// Standard field names for structured logging
const (
	// Component fields
	FieldComponent = "component"
	FieldOperation = "operation"

	// Request fields
	FieldRequestID = "request_id"
	FieldMethod    = "method"
	FieldDuration  = "duration_ms"
	FieldStatus    = "status"

	// Resource fields
	FieldResourceVersion   = "resource.version"
	FieldResourceKind      = "resource.kind"
	FieldResourceName      = "resource.name"
	FieldResourceNamespace = "resource.namespace"

	// Code fields
	FieldCodeHash = "code_hash"
	FieldCodeLen  = "code_length"

	// Error fields
	FieldError      = "error"
	FieldErrorCode  = "error_code"
	FieldStackTrace = "stack_trace"

	// Process fields
	FieldProcessID  = "process_id"
	FieldPort       = "port"
	FieldPID        = "pid"
	FieldRetryCount = "retry_count"
)

// WithResource adds resource information to the logger
func WithResource(logger Logger, resource *types.ResourceInfo) Logger {
	if resource == nil {
		return logger
	}
	return logger.WithFields(resource.ToFields())
}

// WithComponent adds component information to the logger
func WithComponent(logger Logger, component string) Logger {
	return logger.WithField(FieldComponent, component)
}

// WithOperation adds operation information to the logger
func WithOperation(logger Logger, operation string) Logger {
	return logger.WithField(FieldOperation, operation)
}

// WithCodeHash adds code hash information to the logger
func WithCodeHash(logger Logger, codeHash string) Logger {
	return logger.WithField(FieldCodeHash, codeHash)
}

// WithError adds error information to the logger
func WithError(logger Logger, err error) Logger {
	if err == nil {
		return logger
	}
	return logger.WithField(FieldError, err.Error())
}
