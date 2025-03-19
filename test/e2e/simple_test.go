package e2e

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"testing"
	"time"

	"github.com/fabrique/crossplane-skyhook/pkg/node"
)

func TestSimpleFunction(t *testing.T) {
	// Create a logger
	logger := log.New(os.Stdout, "[test] ", log.LstdFlags)

	// Create a temporary directory
	tempDir := os.TempDir()

	// Create a process manager
	processManager, err := node.NewProcessManager(5*time.Minute, 30*time.Minute, tempDir, logger)
	if err != nil {
		t.Fatalf("Failed to create process manager: %v", err)
	}

	// Create a simple JavaScript function
	code := `
	// A simple function that transforms the input
	const result = {};
	for (const [key, value] of Object.entries(input)) {
		result[key.toUpperCase()] = value.toString().toUpperCase();
	}
	return result;
	`

	// Create input data
	input := map[string]interface{}{
		"name":  "John Doe",
		"age":   30,
		"email": "john.doe@example.com",
	}

	// Convert input to JSON
	inputJSON, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("Failed to marshal input: %v", err)
	}

	// Execute the function
	ctx := context.Background()
	result, err := processManager.ExecuteFunction(ctx, code, string(inputJSON))
	if err != nil {
		t.Fatalf("Failed to execute function: %v", err)
	}

	// Parse the result
	var nodeResp struct {
		Result map[string]string `json:"result,omitempty"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
			Stack   string `json:"stack,omitempty"`
		} `json:"error,omitempty"`
	}

	if err := json.Unmarshal([]byte(result), &nodeResp); err != nil {
		t.Fatalf("Failed to parse result: %v", err)
	}

	// Check for errors
	if nodeResp.Error != nil {
		t.Fatalf("Function execution failed: %s", nodeResp.Error.Message)
	}

	// Verify the result
	expected := map[string]string{
		"NAME":  "JOHN DOE",
		"AGE":   "30",
		"EMAIL": "JOHN.DOE@EXAMPLE.COM",
	}

	for key, value := range expected {
		if nodeResp.Result[key] != value {
			t.Errorf("Expected %s=%s, got %s=%s", key, value, key, nodeResp.Result[key])
		}
	}
}
