package logger

// Logger is the interface that wraps the basic logging methods
type Logger interface {
	// Debug logs a message at level Debug
	Debug(args ...interface{})
	// Debugf logs a formatted message at level Debug
	Debugf(format string, args ...interface{})
	// Info logs a message at level Info
	Info(args ...interface{})
	// Infof logs a formatted message at level Info
	Infof(format string, args ...interface{})
	// Warn logs a message at level Warn
	Warn(args ...interface{})
	// Warnf logs a formatted message at level Warn
	Warnf(format string, args ...interface{})
	// Error logs a message at level Error
	Error(args ...interface{})
	// Errorf logs a formatted message at level Error
	Errorf(format string, args ...interface{})
	// Fatal logs a message at level Fatal then the process will exit with status set to 1
	Fatal(args ...interface{})
	// Fatalf logs a formatted message at level Fatal then the process will exit with status set to 1
	Fatalf(format string, args ...interface{})
	// WithField adds a field to the logger
	WithField(key string, value interface{}) Logger
	// WithFields adds multiple fields to the logger
	WithFields(fields map[string]interface{}) Logger
	// WithValues adds key-value pairs to the logger
	WithValues(keysAndValues ...interface{}) Logger
}

// WithValues adds key-value pairs to the logger
// This is a helper function that converts key-value pairs to a map and calls WithFields
func WithValues(log Logger, keysAndValues ...interface{}) Logger {
	if len(keysAndValues)%2 != 0 {
		return log.WithField("error", "odd number of arguments passed as key-value pairs for logging")
	}

	fields := make(map[string]interface{}, len(keysAndValues)/2)
	for i := 0; i < len(keysAndValues); i += 2 {
		key, ok := keysAndValues[i].(string)
		if !ok {
			key = "unknown"
		}
		fields[key] = keysAndValues[i+1]
	}

	return log.WithFields(fields)
}
