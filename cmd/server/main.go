package main

import (
	"flag"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/socialgouv/crossplane-skyhook/pkg/config"
	"github.com/socialgouv/crossplane-skyhook/pkg/grpc"
	"github.com/socialgouv/crossplane-skyhook/pkg/logger"
	"github.com/socialgouv/crossplane-skyhook/pkg/node"
)

func main() {
	// Parse command line flags
	grpcAddr := flag.String("grpc-addr", ":9443", "gRPC server address")
	tempDir := flag.String("temp-dir", "", "Temporary directory for code files")
	gcInterval := flag.Duration("gc-interval", 5*time.Minute, "Garbage collection interval")
	idleTimeout := flag.Duration("idle-timeout", 30*time.Minute, "Idle process timeout")
	logLevel := flag.String("log-level", "info", "Log level (debug, info, warn, error)")
	logFormat := flag.String("log-format", "auto", "Log format (auto, text, json). Auto uses text for TTY, JSON otherwise")
	flag.Parse()

	// Create logger
	log := logger.NewLogrusLogger(*logLevel, *logFormat)

	// Create configuration
	cfg := config.DefaultConfig()
	if *grpcAddr != "" {
		cfg.GRPCAddress = *grpcAddr
	}
	if *tempDir != "" {
		cfg.TempDir = *tempDir
	}
	if *gcInterval > 0 {
		cfg.GCInterval = *gcInterval
	}
	if *idleTimeout > 0 {
		cfg.IdleTimeout = *idleTimeout
	}

	// Check for TLS configuration
	tlsCertsDir := os.Getenv("TLS_SERVER_CERTS_DIR")
	if tlsCertsDir != "" {
		log.Infof("TLS certificates directory found at %s", tlsCertsDir)
		cfg.TLSEnabled = true
		cfg.TLSCertFile = filepath.Join(tlsCertsDir, "tls.crt")
		cfg.TLSKeyFile = filepath.Join(tlsCertsDir, "tls.key")
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	// Create process manager
	processManager, err := node.NewProcessManager(cfg.GCInterval, cfg.IdleTimeout, cfg.TempDir, log)
	if err != nil {
		log.Fatalf("Failed to create process manager: %v", err)
	}

	// Create gRPC server
	server := grpc.NewServer(processManager, log)

	// Start gRPC server
	go func() {
		if err := server.Start(cfg.GRPCAddress, cfg.TLSEnabled, cfg.TLSCertFile, cfg.TLSKeyFile); err != nil {
			log.Fatalf("Failed to start gRPC server: %v", err)
		}
	}()

	log.Infof("Server started on %s", cfg.GRPCAddress)

	// Wait for termination signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	// Stop the server
	log.Info("Shutting down...")
	server.Stop()
}
