package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/socialgouv/xfuncjs-server/pkg/config"
	pkgerrors "github.com/socialgouv/xfuncjs-server/pkg/errors"
	"github.com/socialgouv/xfuncjs-server/pkg/grpc"
	"github.com/socialgouv/xfuncjs-server/pkg/http"
	"github.com/socialgouv/xfuncjs-server/pkg/logger"
	"github.com/socialgouv/xfuncjs-server/pkg/node"
)

func main() {
	// Load configuration from environment variables
	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Define command line flags with current config values as defaults
	grpcAddr := flag.String("grpc-addr", cfg.GRPCAddress, "gRPC server address")
	httpAddr := flag.String("http-addr", ":8080", "HTTP server address for health checks")
	tempDir := flag.String("temp-dir", cfg.TempDir, "Temporary directory for code files")
	gcInterval := flag.Duration("gc-interval", cfg.GCInterval, "Garbage collection interval")
	idleTimeout := flag.Duration("idle-timeout", cfg.IdleTimeout, "Idle process timeout")
	logLevel := flag.String("log-level", cfg.LogLevel, "Log level (debug, info, warn, error)")
	logFormat := flag.String("log-format", cfg.LogFormat, "Log format (auto, text, json). Auto uses text for TTY, JSON otherwise")
	nodeServerPort := flag.Int("node-server-port", cfg.NodeServerPort, "Port for the Node.js HTTP server")
	healthCheckWait := flag.Duration("health-check-wait", cfg.HealthCheckWait, "Timeout for health check")
	healthCheckInterval := flag.Duration("health-check-interval", cfg.HealthCheckInterval, "Interval for health check polling")
	requestTimeout := flag.Duration("request-timeout", cfg.NodeRequestTimeout, "Timeout for requests")
	tlsEnabled := flag.Bool("tls-enabled", cfg.TLSEnabled, "Enable TLS")
	tlsCertFile := flag.String("tls-cert-file", cfg.TLSCertFile, "Path to TLS certificate file")
	tlsKeyFile := flag.String("tls-key-file", cfg.TLSKeyFile, "Path to TLS key file")
	flag.Parse()

	// Override config with command line flags (highest priority)
	cfg.GRPCAddress = *grpcAddr
	if *tempDir != "" {
		cfg.TempDir = *tempDir
	}
	cfg.GCInterval = *gcInterval
	cfg.IdleTimeout = *idleTimeout
	cfg.LogLevel = *logLevel
	cfg.LogFormat = *logFormat
	cfg.NodeServerPort = *nodeServerPort
	cfg.HealthCheckWait = *healthCheckWait
	cfg.HealthCheckInterval = *healthCheckInterval
	cfg.NodeRequestTimeout = *requestTimeout
	cfg.TLSEnabled = *tlsEnabled
	if *tlsCertFile != "" {
		cfg.TLSCertFile = *tlsCertFile
	}
	if *tlsKeyFile != "" {
		cfg.TLSKeyFile = *tlsKeyFile
	}

	// Create logger
	log := logger.NewLogrusLogger(cfg.LogLevel, cfg.LogFormat)

	// Add component information to the logger
	log = logger.WithComponent(log, "main")

	// Log configuration source information
	log.Info("Configuration loaded from environment variables and command line flags")

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		err = pkgerrors.WrapWithCode(err, pkgerrors.ErrorCodeInvalidInput, "configuration validation failed")
		log.WithFields(pkgerrors.GetFields(err)).Fatal("Invalid configuration")
	}

	// Create process manager with all configuration options
	processManager, err := node.NewProcessManager(
		cfg.GCInterval,
		cfg.IdleTimeout,
		cfg.TempDir,
		log,
		node.WithNodeServerPort(cfg.NodeServerPort),
		node.WithHealthCheckWait(cfg.HealthCheckWait),
		node.WithHealthCheckInterval(cfg.HealthCheckInterval),
		node.WithRequestTimeout(cfg.NodeRequestTimeout),
		node.WithYarnQueue(cfg.MaxConcurrentYarnInstalls),
	)
	if err != nil {
		err = pkgerrors.WrapWithCode(err, pkgerrors.ErrorCodeInternalError, "failed to create process manager")
		log.WithFields(pkgerrors.GetFields(err)).Fatal("Failed to create process manager")
	}

	// Create gRPC server
	grpcServer := grpc.NewServer(processManager, log)

	// Create HTTP server for health checks
	httpServer := http.NewServer(processManager, log)

	// Start gRPC server
	go func() {
		log.WithFields(map[string]interface{}{
			"address":     cfg.GRPCAddress,
			"tls_enabled": cfg.TLSEnabled,
		}).Info("Starting gRPC server")

		if err := grpcServer.Start(cfg.GRPCAddress, cfg.TLSEnabled, cfg.TLSCertFile, cfg.TLSKeyFile); err != nil {
			err = pkgerrors.WrapWithCode(err, pkgerrors.ErrorCodeInternalError, "failed to start gRPC server")
			log.WithFields(pkgerrors.GetFields(err)).Fatal("Failed to start gRPC server")
		}
	}()

	// Start HTTP server for health checks
	go func() {
		log.WithField("address", *httpAddr).Info("Starting HTTP server for health checks")

		if err := httpServer.Start(*httpAddr); err != nil && err != http.ErrServerClosed {
			err = pkgerrors.WrapWithCode(err, pkgerrors.ErrorCodeInternalError, "failed to start HTTP server")
			log.WithFields(pkgerrors.GetFields(err)).Fatal("Failed to start HTTP server")
		}
	}()

	log.WithField("address", cfg.GRPCAddress).Info("gRPC server started")
	log.WithField("address", *httpAddr).Info("HTTP server started")

	// Wait for termination signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh

	log.WithField("signal", sig.String()).Info("Received termination signal")

	// Stop the servers
	log.Info("Shutting down servers...")

	// Create a context with timeout for graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Stop the HTTP server
	if err := httpServer.Stop(ctx); err != nil {
		err = pkgerrors.Wrap(err, "error stopping HTTP server")
		log.WithFields(pkgerrors.GetFields(err)).Error("Failed to stop HTTP server")
	} else {
		log.Info("HTTP server stopped successfully")
	}

	// Stop the gRPC server
	grpcServer.Stop()
	log.Info("gRPC server stopped successfully")

	log.Info("Shutdown complete")
}
