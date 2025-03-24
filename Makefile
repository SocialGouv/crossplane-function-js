GO := go
PROJECT_NAME := crossplane-skyhook
VERSION := $(shell git describe --tags 2>/dev/null || echo "v0.1.0")
DOCKER_IMAGE := localhost:5001/$(PROJECT_NAME)

.PHONY: all build build-static clean test e2e-test e2e-test-simple setup-test-env clean-test-env

# Default target
all: build

# Refresh dependencies
deps:
	$(GO) mod tidy
	$(GO) mod vendor

# Update dependencies
update-deps:
	$(GO) get -u ./...

# Build the project
build:
	$(GO) build -o bin/skyhook-server cmd/server/main.go

# Build a static binary
build-static:
	CGO_ENABLED=0 $(GO) build -a -installsuffix cgo -o bin/skyhook-server cmd/server/main.go

# Clean build artifacts
clean:
	rm -f bin/skyhook-server
	$(GO) clean

# Run unit tests
test:
	$(GO) test -v ./...

# Set up test environment
setup-test-env:
	./tests/kind-with-registry.sh
	./tests/install-crossplane.sh

# Run e2e tests
e2e-test:
	./tests/e2e.sh

# Clean up test environment
clean-test-env:
	kind delete cluster || true
