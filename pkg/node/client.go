package node

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/socialgouv/crossplane-skyhook/pkg/logger"
)

// NodeClient communicates with the Node.js HTTP server
type NodeClient struct {
	baseURL    string
	httpClient *http.Client
	logger     logger.Logger
}

// NewNodeClient creates a new Node.js HTTP client
func NewNodeClient(baseURL string, timeout time.Duration, logger logger.Logger) *NodeClient {
	return &NodeClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		logger: logger,
	}
}

// ExecuteFunction sends a request to the Node.js server to execute a function
func (c *NodeClient) ExecuteFunction(ctx context.Context, code, inputJSON string) (string, error) {
	// Create the request payload
	type requestPayload struct {
		Code  string          `json:"code"`
		Input json.RawMessage `json:"input"`
	}

	// Parse the input JSON to ensure it's valid
	var inputRaw json.RawMessage
	if err := json.Unmarshal([]byte(inputJSON), &inputRaw); err != nil {
		return "", fmt.Errorf("invalid input JSON: %w", err)
	}

	payload := requestPayload{
		Code:  code,
		Input: inputRaw,
	}

	// Marshal the payload to JSON
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request payload: %w", err)
	}

	// Create the HTTP request
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		fmt.Sprintf("%s/execute", c.baseURL),
		bytes.NewReader(payloadBytes),
	)
	if err != nil {
		return "", fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	// Send the request
	c.logger.Debugf("Sending request to Node.js server: %s", c.baseURL)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send HTTP request: %w", err)
	}
	defer resp.Body.Close()

	// Read the response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	// Check the response status
	if resp.StatusCode != http.StatusOK {
		c.logger.Errorf("Node.js server returned non-OK status: %d, body: %s", resp.StatusCode, string(respBody))
		return "", fmt.Errorf("Node.js server returned status %d: %s", resp.StatusCode, string(respBody))
	}

	// Return the response body as a string
	return string(respBody), nil
}

// CheckHealth checks if the Node.js server is healthy
func (c *NodeClient) CheckHealth(ctx context.Context) error {
	// Create the HTTP request
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		fmt.Sprintf("%s/healthcheck", c.baseURL),
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Send the request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send HTTP request: %w", err)
	}
	defer resp.Body.Close()

	// Check the response status
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("healthcheck failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// CheckReady checks if the Node.js server is ready
func (c *NodeClient) CheckReady(ctx context.Context) error {
	// Create the HTTP request
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		fmt.Sprintf("%s/ready", c.baseURL),
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Send the request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send HTTP request: %w", err)
	}
	defer resp.Body.Close()

	// Check the response status
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("readiness check failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// WaitForReady waits for the Node.js server to be ready
func (c *NodeClient) WaitForReady(ctx context.Context, timeout, interval time.Duration) error {
	// Create a context with timeout
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Create a ticker for polling
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Poll the ready endpoint until it's ready or times out
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for Node.js server to be ready: %w", ctx.Err())
		case <-ticker.C:
			if err := c.CheckReady(ctx); err != nil {
				c.logger.Debugf("Node.js server not ready yet: %v", err)
			} else {
				c.logger.Info("Node.js server is ready")
				return nil
			}
		}
	}
}
