package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Config holds the configuration for the server
type Config struct {
	// GRPCAddress is the address for the gRPC server
	GRPCAddress string

	// TempDir is the directory for temporary files
	TempDir string

	// GCInterval is the interval for garbage collection
	GCInterval time.Duration

	// IdleTimeout is the timeout for idle processes
	IdleTimeout time.Duration

	// TLSEnabled indicates whether TLS is enabled
	TLSEnabled bool

	// TLSCertFile is the path to the TLS certificate file
	TLSCertFile string

	// TLSKeyFile is the path to the TLS key file
	TLSKeyFile string
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	return &Config{
		GRPCAddress: ":9443",
		TempDir:     filepath.Join(os.TempDir(), "crossplane-skyhook"),
		GCInterval:  5 * time.Minute,
		IdleTimeout: 30 * time.Minute,
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
	return nil
}
