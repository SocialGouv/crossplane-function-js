package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/kelseyhightower/envconfig"
)

// Config holds the configuration for the server
type Config struct {
	// Server configuration
	GRPCAddress string        `envconfig:"GRPC_ADDRESS" default:":9443" description:"gRPC server address"`
	TempDir     string        `envconfig:"TEMP_DIR" description:"Temporary directory for code files"`
	GCInterval  time.Duration `envconfig:"GC_INTERVAL" default:"5m" description:"Garbage collection interval"`
	IdleTimeout time.Duration `envconfig:"IDLE_TIMEOUT" default:"30m" description:"Idle process timeout"`

	// TLS configuration
	TLSEnabled  bool   `envconfig:"TLS_ENABLED" default:"false" description:"Enable TLS"`
	TLSCertFile string `envconfig:"TLS_CERT_FILE" description:"Path to TLS certificate file"`
	TLSKeyFile  string `envconfig:"TLS_KEY_FILE" description:"Path to TLS key file"`

	// Logging configuration
	LogLevel  string `envconfig:"LOG_LEVEL" default:"info" description:"Log level (debug, info, warn, error)"`
	LogFormat string `envconfig:"LOG_FORMAT" default:"auto" description:"Log format (auto, text, json)"`

	// Node.js server configuration
	NodeServerPort      int           `envconfig:"NODE_SERVER_PORT" default:"3000" description:"Port for the Node.js HTTP server"`
	HealthCheckWait     time.Duration `envconfig:"HEALTH_CHECK_WAIT" default:"30s" description:"Timeout for health check"`
	HealthCheckInterval time.Duration `envconfig:"HEALTH_CHECK_INTERVAL" default:"500ms" description:"Interval for health check polling"`
	NodeRequestTimeout  time.Duration `envconfig:"NODE_REQUEST_TIMEOUT" default:"30s" description:"Timeout for Node.js requests"`
}

// LoadConfig loads configuration from environment variables and returns a Config
func LoadConfig() (*Config, error) {
	config := &Config{}

	// Process environment variables
	if err := envconfig.Process("SKYHOOK", config); err != nil {
		return nil, fmt.Errorf("failed to process environment variables: %w", err)
	}

	// Set default temp dir if not specified
	if config.TempDir == "" {
		config.TempDir = filepath.Join(os.TempDir(), "crossplane-skyhook")
	}

	// Handle legacy env var for backward compatibility
	if tlsCertsDir := os.Getenv("TLS_SERVER_CERTS_DIR"); tlsCertsDir != "" {
		config.TLSEnabled = true
		config.TLSCertFile = filepath.Join(tlsCertsDir, "tls.crt")
		config.TLSKeyFile = filepath.Join(tlsCertsDir, "tls.key")
	}

	// Validate configuration
	if err := config.Validate(); err != nil {
		return nil, err
	}

	return config, nil
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
