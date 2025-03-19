package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/fabrique/crossplane-skyhook/pkg/config"
	"github.com/fabrique/crossplane-skyhook/pkg/grpc"
	"github.com/fabrique/crossplane-skyhook/pkg/node"
)

func main() {
	// Parse command line flags
	grpcAddr := flag.String("grpc-addr", ":50051", "gRPC server address")
	tempDir := flag.String("temp-dir", "", "Temporary directory for code files")
	gcInterval := flag.Duration("gc-interval", 5*time.Minute, "Garbage collection interval")
	idleTimeout := flag.Duration("idle-timeout", 30*time.Minute, "Idle process timeout")
	flag.Parse()

	// Create logger
	logger := log.New(os.Stdout, "[skyhook] ", log.LstdFlags)

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

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		logger.Fatalf("Invalid configuration: %v", err)
	}

	// Create process manager
	processManager, err := node.NewProcessManager(cfg.GCInterval, cfg.IdleTimeout, cfg.TempDir, logger)
	if err != nil {
		logger.Fatalf("Failed to create process manager: %v", err)
	}

	// Create gRPC server
	server := grpc.NewServer(processManager, logger)

	// Start gRPC server
	go func() {
		if err := server.Start(cfg.GRPCAddress); err != nil {
			logger.Fatalf("Failed to start gRPC server: %v", err)
		}
	}()

	logger.Printf("Server started on %s", cfg.GRPCAddress)

	// Wait for termination signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	// Stop the server
	logger.Println("Shutting down...")
	server.Stop()
}
