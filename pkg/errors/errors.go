package errors

import (
	"errors"
	"fmt"
	"runtime"
	"strings"

	"github.com/socialgouv/crossplane-skyhook/pkg/types"
)

// Field names for structured logging
const (
	FieldError      = "error"
	FieldErrorCode  = "error_code"
	FieldStackTrace = "stack_trace"
)

// Standard error codes for Crossplane
const (
	// ErrorCodeUnknown is used when the error type is unknown
	ErrorCodeUnknown = "ERR_UNKNOWN"
	// ErrorCodeInvalidInput is used when the input is invalid
	ErrorCodeInvalidInput = "ERR_INVALID_INPUT"
	// ErrorCodeProcessFailed is used when a process fails
	ErrorCodeProcessFailed = "ERR_PROCESS_FAILED"
	// ErrorCodeTimeout is used when an operation times out
	ErrorCodeTimeout = "ERR_TIMEOUT"
	// ErrorCodeResourceNotFound is used when a resource is not found
	ErrorCodeResourceNotFound = "ERR_RESOURCE_NOT_FOUND"
	// ErrorCodeInternalError is used for internal errors
	ErrorCodeInternalError = "ERR_INTERNAL"
	// ErrorCodeNetworkError is used for network-related errors
	ErrorCodeNetworkError = "ERR_NETWORK"
)

// ContextualError is an error with additional context
type ContextualError struct {
	// Original is the original error
	Original error
	// Message is the contextual message
	Message string
	// Code is the error code
	Code string
	// Resource is the resource information
	Resource *types.ResourceInfo
	// Fields contains additional fields for logging
	Fields map[string]interface{}
	// Stack contains the stack trace
	Stack string
}

// Error returns the error message
func (e *ContextualError) Error() string {
	if e.Original != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Original)
	}
	return e.Message
}

// Unwrap returns the original error
func (e *ContextualError) Unwrap() error {
	return e.Original
}

// Is reports whether any error in err's tree matches target
func (e *ContextualError) Is(target error) bool {
	if e.Original == nil {
		return false
	}
	return errors.Is(e.Original, target)
}

// ToFields converts the error to a map of logger fields
func (e *ContextualError) ToFields() map[string]interface{} {
	fields := make(map[string]interface{})

	// Add error message
	fields[FieldError] = e.Error()

	// Add error code if available
	if e.Code != "" {
		fields[FieldErrorCode] = e.Code
	}

	// Add stack trace if available
	if e.Stack != "" {
		fields[FieldStackTrace] = e.Stack
	}

	// Add resource information if available
	if e.Resource != nil {
		for k, v := range e.Resource.ToFields() {
			fields[k] = v
		}
	}

	// Add additional fields
	for k, v := range e.Fields {
		fields[k] = v
	}

	return fields
}

// Wrap wraps an error with a message and returns a ContextualError
func Wrap(err error, message string) error {
	if err == nil {
		return nil
	}

	// Check if the error is already a ContextualError
	var contextualErr *ContextualError
	if errors.As(err, &contextualErr) {
		// Create a new ContextualError with the updated message
		return &ContextualError{
			Original: contextualErr.Original,
			Message:  fmt.Sprintf("%s: %s", message, contextualErr.Message),
			Code:     contextualErr.Code,
			Resource: contextualErr.Resource,
			Fields:   contextualErr.Fields,
			Stack:    contextualErr.Stack,
		}
	}

	// Create a new ContextualError
	return &ContextualError{
		Original: err,
		Message:  message,
		Code:     ErrorCodeUnknown,
		Fields:   make(map[string]interface{}),
		Stack:    captureStack(2), // Skip this function and the caller
	}
}

// WrapWithCode wraps an error with a message and error code
func WrapWithCode(err error, code, message string) error {
	if err == nil {
		return nil
	}

	// Check if the error is already a ContextualError
	var contextualErr *ContextualError
	if errors.As(err, &contextualErr) {
		// Create a new ContextualError with the updated message and code
		return &ContextualError{
			Original: contextualErr.Original,
			Message:  fmt.Sprintf("%s: %s", message, contextualErr.Message),
			Code:     code,
			Resource: contextualErr.Resource,
			Fields:   contextualErr.Fields,
			Stack:    contextualErr.Stack,
		}
	}

	// Create a new ContextualError
	return &ContextualError{
		Original: err,
		Message:  message,
		Code:     code,
		Fields:   make(map[string]interface{}),
		Stack:    captureStack(2), // Skip this function and the caller
	}
}

// WrapWithResource wraps an error with a message and resource information
func WrapWithResource(err error, resource *types.ResourceInfo, message string) error {
	if err == nil {
		return nil
	}

	// Check if the error is already a ContextualError
	var contextualErr *ContextualError
	if errors.As(err, &contextualErr) {
		// Create a new ContextualError with the updated message and resource
		return &ContextualError{
			Original: contextualErr.Original,
			Message:  fmt.Sprintf("%s: %s", message, contextualErr.Message),
			Code:     contextualErr.Code,
			Resource: resource,
			Fields:   contextualErr.Fields,
			Stack:    contextualErr.Stack,
		}
	}

	// Create a new ContextualError
	return &ContextualError{
		Original: err,
		Message:  message,
		Code:     ErrorCodeUnknown,
		Resource: resource,
		Fields:   make(map[string]interface{}),
		Stack:    captureStack(2), // Skip this function and the caller
	}
}

// WrapWithField wraps an error with a message and additional field
func WrapWithField(err error, key string, value interface{}, message string) error {
	if err == nil {
		return nil
	}

	// Check if the error is already a ContextualError
	var contextualErr *ContextualError
	if errors.As(err, &contextualErr) {
		// Create a new ContextualError with the updated message and field
		newFields := make(map[string]interface{})
		for k, v := range contextualErr.Fields {
			newFields[k] = v
		}
		newFields[key] = value

		return &ContextualError{
			Original: contextualErr.Original,
			Message:  fmt.Sprintf("%s: %s", message, contextualErr.Message),
			Code:     contextualErr.Code,
			Resource: contextualErr.Resource,
			Fields:   newFields,
			Stack:    contextualErr.Stack,
		}
	}

	// Create a new ContextualError
	fields := make(map[string]interface{})
	fields[key] = value

	return &ContextualError{
		Original: err,
		Message:  message,
		Code:     ErrorCodeUnknown,
		Fields:   fields,
		Stack:    captureStack(2), // Skip this function and the caller
	}
}

// New creates a new error with a message
func New(message string) error {
	return &ContextualError{
		Message: message,
		Code:    ErrorCodeUnknown,
		Fields:  make(map[string]interface{}),
		Stack:   captureStack(2), // Skip this function and the caller
	}
}

// NewWithCode creates a new error with a message and error code
func NewWithCode(code, message string) error {
	return &ContextualError{
		Message: message,
		Code:    code,
		Fields:  make(map[string]interface{}),
		Stack:   captureStack(2), // Skip this function and the caller
	}
}

// GetCode returns the error code from an error
func GetCode(err error) string {
	if err == nil {
		return ""
	}

	var contextualErr *ContextualError
	if errors.As(err, &contextualErr) {
		return contextualErr.Code
	}

	return ErrorCodeUnknown
}

// GetResource returns the resource information from an error
func GetResource(err error) *types.ResourceInfo {
	if err == nil {
		return nil
	}

	var contextualErr *ContextualError
	if errors.As(err, &contextualErr) {
		return contextualErr.Resource
	}

	return nil
}

// GetFields returns the fields from an error
func GetFields(err error) map[string]interface{} {
	if err == nil {
		return nil
	}

	var contextualErr *ContextualError
	if errors.As(err, &contextualErr) {
		return contextualErr.ToFields()
	}

	// For regular errors, just return the error message
	return map[string]interface{}{
		FieldError: err.Error(),
	}
}

// captureStack captures the current stack trace
func captureStack(skip int) string {
	const depth = 32
	var pcs [depth]uintptr
	n := runtime.Callers(skip, pcs[:])
	frames := runtime.CallersFrames(pcs[:n])

	var builder strings.Builder
	for {
		frame, more := frames.Next()
		if !more {
			break
		}

		// Skip runtime and testing packages
		if strings.Contains(frame.Function, "runtime.") || strings.Contains(frame.Function, "testing.") {
			continue
		}

		// Add the function and file information
		fmt.Fprintf(&builder, "%s\n\t%s:%d\n", frame.Function, frame.File, frame.Line)

		// Limit the stack trace to a reasonable depth
		if builder.Len() > 4096 {
			fmt.Fprintf(&builder, "...\n")
			break
		}
	}

	return builder.String()
}
