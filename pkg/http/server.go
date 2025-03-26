package http

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/socialgouv/xfuncjs-server/pkg/logger"
	"github.com/socialgouv/xfuncjs-server/pkg/node"
)

// ErrServerClosed is returned by the Server's Start method after a call to Stop.
var ErrServerClosed = errors.New("http: server closed")

// Server is the HTTP server for health checks
type Server struct {
	server         *http.Server
	processManager *node.ProcessManager
	logger         logger.Logger
}

// NewServer creates a new HTTP server for health checks
func NewServer(processManager *node.ProcessManager, logger logger.Logger) *Server {
	return &Server{
		processManager: processManager,
		logger:         logger,
	}
}

// Start starts the HTTP server on the specified address
func (s *Server) Start(address string) error {
	mux := http.NewServeMux()

	// Register the /healthz endpoint for Kubernetes
	mux.HandleFunc("/healthz", s.healthzHandler)

	s.server = &http.Server{
		Addr:    address,
		Handler: mux,
	}

	s.logger.Infof("Starting HTTP health check server on %s", address)
	return s.server.ListenAndServe()
}

// Stop stops the HTTP server
func (s *Server) Stop(ctx context.Context) error {
	if s.server != nil {
		s.logger.Info("Stopping HTTP health check server")
		return s.server.Shutdown(ctx)
	}
	return nil
}

// healthzHandler handles the /healthz endpoint for Kubernetes
func (s *Server) healthzHandler(w http.ResponseWriter, r *http.Request) {
	// This endpoint is used by Kubernetes to check if the service is healthy
	// It should return a 200 OK response if the service is healthy
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"status":"ok","timestamp":"%s"}`, time.Now().Format(time.RFC3339))
}
