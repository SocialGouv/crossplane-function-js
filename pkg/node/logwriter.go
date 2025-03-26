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
			// Try to parse the line as JSON
			var jsonData map[string]interface{}
			if err := json.Unmarshal([]byte(line), &jsonData); err == nil {
				// Successfully parsed as JSON
				// Create a new map with "js-" prefix for each key
				prefixedData := make(map[string]interface{})
				for k, v := range jsonData {
					prefixedData["js-"+k] = v
				}

				// Check if this is an error message
				if _, hasError := jsonData["error"]; hasError {
					// Log as an error with fields
					w.logger.WithFields(prefixedData).
						WithField(logger.FieldComponent, "node").
						Error("Node.js process reported an error")
				} else {
					// Log as info with fields
					w.logger.WithFields(prefixedData).
						WithField(logger.FieldComponent, "node").
						Info(w.prefix)
				}
			} else {
				// Check if this looks like an error message
				if strings.Contains(strings.ToLower(line), "error") ||
					strings.Contains(strings.ToLower(line), "exception") ||
					strings.Contains(strings.ToLower(line), "fail") {
					// Log as an error
					w.logger.WithField(logger.FieldComponent, "node").
						WithField(logger.FieldError, line).
						Error("Node.js process reported an error")
				} else {
					// Log as plain text
					w.logger.WithField(logger.FieldComponent, "node").
						Infof("%s%s", w.prefix, line)
				}
			}
		}
	}

	return len(p), nil
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
