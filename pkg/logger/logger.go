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
}
