package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Config holds the configuration for the server
type Config struct {
	// Server configuration
	GRPCAddress string        // Address for the gRPC server
	TempDir     string        // Directory for temporary files
	GCInterval  time.Duration // Interval for garbage collection
	IdleTimeout time.Duration // Timeout for idle processes

	// TLS configuration
	TLSEnabled  bool   // Whether TLS is enabled
	TLSCertFile string // Path to the TLS certificate file
	TLSKeyFile  string // Path to the TLS key file

	// Logging configuration
	LogLevel  string // Log level (debug, info, warn, error)
	LogFormat string // Log format (auto, text, json)

	// Node.js server configuration
	NodeServerPort      int           // Port for the Node.js HTTP server
	HealthCheckWait     time.Duration // Timeout for health check
	HealthCheckInterval time.Duration // Interval for health check polling
	NodeRequestTimeout  time.Duration // Timeout for Node.js requests
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	return &Config{
		// Server defaults
		GRPCAddress: ":9443",
		TempDir:     filepath.Join(os.TempDir(), "crossplane-skyhook"),
		GCInterval:  5 * time.Minute,
		IdleTimeout: 30 * time.Minute,

		// TLS defaults (disabled by default)
		TLSEnabled: false,

		// Logging defaults
		LogLevel:  "info",
		LogFormat: "auto",

		// Node.js server defaults
		NodeServerPort:      3000,
		HealthCheckWait:     30 * time.Second,
		HealthCheckInterval: 500 * time.Millisecond,
		NodeRequestTimeout:  30 * time.Second,
	}
}

// LoadFromEnv loads configuration from environment variables
func (c *Config) LoadFromEnv() {
	// Server configuration
	if val := os.Getenv("SKYHOOK_GRPC_ADDRESS"); val != "" {
		c.GRPCAddress = val
	}
	if val := os.Getenv("SKYHOOK_TEMP_DIR"); val != "" {
		c.TempDir = val
	}
	if val := os.Getenv("SKYHOOK_GC_INTERVAL"); val != "" {
		if duration, err := time.ParseDuration(val); err == nil {
			c.GCInterval = duration
		}
	}
	if val := os.Getenv("SKYHOOK_IDLE_TIMEOUT"); val != "" {
		if duration, err := time.ParseDuration(val); err == nil {
			c.IdleTimeout = duration
		}
	}

	// TLS configuration
	if val := os.Getenv("SKYHOOK_TLS_ENABLED"); val != "" {
		c.TLSEnabled = strings.ToLower(val) == "true" || val == "1"
	}
	// For backward compatibility
	if tlsCertsDir := os.Getenv("TLS_SERVER_CERTS_DIR"); tlsCertsDir != "" {
		c.TLSEnabled = true
		c.TLSCertFile = filepath.Join(tlsCertsDir, "tls.crt")
		c.TLSKeyFile = filepath.Join(tlsCertsDir, "tls.key")
	}
	if val := os.Getenv("SKYHOOK_TLS_CERT_FILE"); val != "" {
		c.TLSCertFile = val
	}
	if val := os.Getenv("SKYHOOK_TLS_KEY_FILE"); val != "" {
		c.TLSKeyFile = val
	}

	// Logging configuration
	if val := os.Getenv("SKYHOOK_LOG_LEVEL"); val != "" {
		c.LogLevel = val
	}
	if val := os.Getenv("SKYHOOK_LOG_FORMAT"); val != "" {
		c.LogFormat = val
	}

	// Node.js server configuration
	if val := os.Getenv("SKYHOOK_NODE_SERVER_PORT"); val != "" {
		if port, err := strconv.Atoi(val); err == nil {
			c.NodeServerPort = port
		}
	}
	if val := os.Getenv("SKYHOOK_HEALTH_CHECK_WAIT"); val != "" {
		if duration, err := time.ParseDuration(val); err == nil {
			c.HealthCheckWait = duration
		}
	}
	if val := os.Getenv("SKYHOOK_HEALTH_CHECK_INTERVAL"); val != "" {
		if duration, err := time.ParseDuration(val); err == nil {
			c.HealthCheckInterval = duration
		}
	}
	if val := os.Getenv("SKYHOOK_NODE_REQUEST_TIMEOUT"); val != "" {
		if duration, err := time.ParseDuration(val); err == nil {
			c.NodeRequestTimeout = duration
		}
	}
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.GRPCAddress == "" {
		return fmt.Errorf("GRPC address is required")
	}
	if c.TempDir == "" {
		return fmt.Errorf("temp directory is required")
	}
	if c.GCInterval <= 0 {
		return fmt.Errorf("GC interval must be positive")
	}
	if c.IdleTimeout <= 0 {
		return fmt.Errorf("idle timeout must be positive")
	}
	if c.TLSEnabled {
		if c.TLSCertFile == "" {
			return fmt.Errorf("TLS certificate file is required when TLS is enabled")
		}
		if c.TLSKeyFile == "" {
			return fmt.Errorf("TLS key file is required when TLS is enabled")
		}
	}
	if c.NodeServerPort <= 0 {
		return fmt.Errorf("Node server port must be positive")
	}
	if c.HealthCheckWait <= 0 {
		return fmt.Errorf("health check wait must be positive")
	}
	if c.HealthCheckInterval <= 0 {
		return fmt.Errorf("health check interval must be positive")
	}
	if c.NodeRequestTimeout <= 0 {
		return fmt.Errorf("node request timeout must be positive")
	}
	return nil
}
