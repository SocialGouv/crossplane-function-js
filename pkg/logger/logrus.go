package logger

import (
	"os"

	"github.com/sirupsen/logrus"
)

// LogrusLogger is an implementation of the Logger interface using logrus
type LogrusLogger struct {
	logger *logrus.Logger
	entry  *logrus.Entry
}

// NewLogrusLogger creates a new LogrusLogger
func NewLogrusLogger(level string, format string) Logger {
	logger := logrus.New()

	// Set output to stdout
	logger.SetOutput(os.Stdout)

	// Set log level
	switch level {
	case "debug":
		logger.SetLevel(logrus.DebugLevel)
	case "info":
		logger.SetLevel(logrus.InfoLevel)
	case "warn":
		logger.SetLevel(logrus.WarnLevel)
	case "error":
		logger.SetLevel(logrus.ErrorLevel)
	default:
		logger.SetLevel(logrus.InfoLevel)
	}

	// Set log format based on format parameter and TTY detection
	if format == "json" {
		logger.SetFormatter(&logrus.JSONFormatter{
			DisableTimestamp: true, // Don't show timestamp by default
		})
	} else if format == "text" {
		logger.SetFormatter(&logrus.TextFormatter{
			FullTimestamp:    false, // Don't show timestamp by default
			DisableTimestamp: true,
		})
	} else {
		// Auto-detect: use text for TTY, JSON otherwise
		if fileInfo, _ := os.Stdout.Stat(); (fileInfo.Mode() & os.ModeCharDevice) != 0 {
			// TTY detected, use text format
			logger.SetFormatter(&logrus.TextFormatter{
				FullTimestamp:    false, // Don't show timestamp by default
				DisableTimestamp: true,
			})
		} else {
			// Not a TTY, use JSON format
			logger.SetFormatter(&logrus.JSONFormatter{
				DisableTimestamp: true, // Don't show timestamp by default
			})
		}
	}

	return &LogrusLogger{
		logger: logger,
		entry:  logrus.NewEntry(logger),
	}
}

// Debug logs a message at level Debug
func (l *LogrusLogger) Debug(args ...interface{}) {
	l.entry.Debug(args...)
}

// Debugf logs a formatted message at level Debug
func (l *LogrusLogger) Debugf(format string, args ...interface{}) {
	l.entry.Debugf(format, args...)
}

// Info logs a message at level Info
func (l *LogrusLogger) Info(args ...interface{}) {
	l.entry.Info(args...)
}

// Infof logs a formatted message at level Info
func (l *LogrusLogger) Infof(format string, args ...interface{}) {
	l.entry.Infof(format, args...)
}

// Warn logs a message at level Warn
func (l *LogrusLogger) Warn(args ...interface{}) {
	l.entry.Warn(args...)
}

// Warnf logs a formatted message at level Warn
func (l *LogrusLogger) Warnf(format string, args ...interface{}) {
	l.entry.Warnf(format, args...)
}

// Error logs a message at level Error
func (l *LogrusLogger) Error(args ...interface{}) {
	l.entry.Error(args...)
}

// Errorf logs a formatted message at level Error
func (l *LogrusLogger) Errorf(format string, args ...interface{}) {
	l.entry.Errorf(format, args...)
}

// Fatal logs a message at level Fatal then the process will exit with status set to 1
func (l *LogrusLogger) Fatal(args ...interface{}) {
	l.entry.Fatal(args...)
}

// Fatalf logs a formatted message at level Fatal then the process will exit with status set to 1
func (l *LogrusLogger) Fatalf(format string, args ...interface{}) {
	l.entry.Fatalf(format, args...)
}

// WithField adds a field to the logger
func (l *LogrusLogger) WithField(key string, value interface{}) Logger {
	return &LogrusLogger{
		logger: l.logger,
		entry:  l.entry.WithField(key, value),
	}
}

// WithFields adds multiple fields to the logger
func (l *LogrusLogger) WithFields(fields map[string]interface{}) Logger {
	return &LogrusLogger{
		logger: l.logger,
		entry:  l.entry.WithFields(logrus.Fields(fields)),
	}
}
