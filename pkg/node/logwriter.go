package node

import (
	"encoding/json"
	"strings"
	"sync"

	"github.com/socialgouv/xfuncjs-server/pkg/logger"
)

// logWriter is a io.Writer that writes to a logger with buffering for partial lines
type logWriter struct {
	logger     logger.Logger
	prefix     string
	streamType string // "stdout" or "stderr"
	buffer     []byte
	bufferLock sync.Mutex
}

func (w *logWriter) Write(p []byte) (n int, err error) {
	w.bufferLock.Lock()
	defer w.bufferLock.Unlock()

	// Append the new data to our buffer
	w.buffer = append(w.buffer, p...)

	// Process complete lines from the buffer
	lines := w.processBuffer()

	// Log each complete line
	for _, line := range lines {
		if line != "" {
			w.logLine(line)
		}
	}

	return len(p), nil
}

// logLine processes and logs a single line
func (w *logWriter) logLine(line string) {
	// Add stream type to logger context
	contextLogger := w.logger.WithField(logger.FieldComponent, "node").
		WithField("stream", w.streamType)

	// Try to parse the line as JSON (Pino format)
	var jsonData map[string]interface{}
	if err := json.Unmarshal([]byte(line), &jsonData); err == nil {
		// Successfully parsed as JSON - this is likely Pino output
		w.logPinoMessage(contextLogger, jsonData)
	} else {
		// Not JSON - handle as plain text
		w.logPlainText(contextLogger, line)
	}
}

// logPinoMessage handles structured Pino log messages
func (w *logWriter) logPinoMessage(contextLogger logger.Logger, jsonData map[string]interface{}) {
	// Create a new map with "js-" prefix for each key to avoid conflicts
	prefixedData := make(map[string]interface{})
	for k, v := range jsonData {
		prefixedData["js-"+k] = v
	}

	// Extract log level from Pino message
	var logLevel string
	if level, ok := jsonData["level"].(string); ok {
		logLevel = strings.ToUpper(level)
	} else if levelNum, ok := jsonData["level"].(float64); ok {
		// Pino uses numeric levels: 10=trace, 20=debug, 30=info, 40=warn, 50=error, 60=fatal
		switch int(levelNum) {
		case 10:
			logLevel = "TRACE"
		case 20:
			logLevel = "DEBUG"
		case 30:
			logLevel = "INFO"
		case 40:
			logLevel = "WARN"
		case 50:
			logLevel = "ERROR"
		case 60:
			logLevel = "FATAL"
		default:
			logLevel = "INFO"
		}
	}

	// Extract message
	var message string
	if msg, ok := jsonData["msg"].(string); ok {
		message = msg
	} else {
		message = w.prefix
	}

	// Log based on the original level and stream type
	loggerWithFields := contextLogger.WithFields(prefixedData)

	// For stderr or error levels, log as error
	if w.streamType == "stderr" || logLevel == "ERROR" || logLevel == "FATAL" {
		loggerWithFields.Error(message)
	} else if logLevel == "WARN" {
		loggerWithFields.Warn(message)
	} else if logLevel == "DEBUG" || logLevel == "TRACE" {
		loggerWithFields.Debug(message)
	} else {
		// Default to info for stdout and info level
		loggerWithFields.Info(message)
	}
}

// logPlainText handles unstructured text messages
func (w *logWriter) logPlainText(contextLogger logger.Logger, line string) {
	// Check if this looks like an error message or if it's from stderr
	isError := w.streamType == "stderr" ||
		strings.Contains(strings.ToLower(line), "error") ||
		strings.Contains(strings.ToLower(line), "exception") ||
		strings.Contains(strings.ToLower(line), "fail") ||
		strings.Contains(strings.ToLower(line), "fatal")

	if isError {
		contextLogger.WithField(logger.FieldError, line).
			Error("Node.js process output")
	} else {
		contextLogger.Infof("%s%s", w.prefix, line)
	}
}

// processBuffer processes the buffer and returns complete lines
// Any incomplete line at the end remains in the buffer
func (w *logWriter) processBuffer() []string {
	var lines []string
	var i, start int

	// Find complete lines in the buffer
	for i < len(w.buffer) {
		if w.buffer[i] == '\n' {
			// Extract the line (excluding the newline)
			line := string(w.buffer[start:i])
			// Trim carriage returns if present
			line = strings.TrimSuffix(line, "\r")
			lines = append(lines, line)

			// Move start to after this newline
			start = i + 1
		}
		i++
	}

	// If we processed any complete lines, update the buffer to contain only the remainder
	if start > 0 {
		w.buffer = w.buffer[start:]
	}

	return lines
}

// Flush forces any buffered data to be written
func (w *logWriter) Flush() {
	w.bufferLock.Lock()
	defer w.bufferLock.Unlock()

	// If there's any data in the buffer, log it even if it's not a complete line
	if len(w.buffer) > 0 {
		line := string(w.buffer)
		w.logger.WithField(logger.FieldComponent, "node").
			WithField("incomplete", true).
			Infof("%s%s (incomplete line)", w.prefix, line)
		w.buffer = nil
	}
}
