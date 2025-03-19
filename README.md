# Crossplane Skyhook

A gRPC server for Crossplane functions that executes inline JavaScript/TypeScript code from Crossplane compositions.

## Overview

Crossplane Skyhook is a gRPC server that allows Crossplane to execute JavaScript/TypeScript code as part of its composition process. It works by:

1. Receiving requests from Crossplane with inline JavaScript/TypeScript code
2. Creating a deterministic hash from the code
3. Creating a Node.js subprocess for the hash if it doesn't exist
4. Relaying the request to the Node.js subprocess
5. Returning the response from the Node.js subprocess back to Crossplane

## Requirements

- Go 1.22+
- Node.js 22+ (for TypeScript support)
- Yarn Berry (already configured in the project)
- protoc (Protocol Buffers compiler)

## Installation

```bash
# Clone the repository
git clone https://github.com/fabrique/crossplane-skyhook.git
cd crossplane-skyhook

# Install Go dependencies
go mod download

# Build the server
go build -o bin/skyhook-server cmd/server/main.go
```

## Usage

### Starting the Server

```bash
./bin/skyhook-server --grpc-addr=:50051 --temp-dir=/tmp/crossplane-skyhook
```

### Command Line Options

- `--grpc-addr`: gRPC server address (default: `:50051`)
- `--temp-dir`: Temporary directory for code files (default: OS temp dir + `/crossplane-skyhook`)
- `--gc-interval`: Garbage collection interval (default: `5m`)
- `--idle-timeout`: Idle process timeout (default: `30m`)

## Architecture

### Go gRPC Server

The Go server handles gRPC requests from Crossplane and manages Node.js subprocesses. It:

- Creates a deterministic hash for each piece of JavaScript/TypeScript code
- Manages a pool of Node.js subprocesses, one for each unique code hash
- Relays requests to the appropriate subprocess
- Collects and returns responses
- Implements garbage collection to terminate idle processes

### Node.js Runtime

The Node.js runtime executes the JavaScript/TypeScript code. It:

- Uses Node.js 22's experimental TypeScript support
- Executes the code in a controlled environment
- Returns structured results or errors

## Testing

### Running Tests

```bash
go test ./test/e2e
```

### End-to-End Testing with Crossplane

The `test/fixtures` directory contains sample Kubernetes manifests for testing with Crossplane:

- `crd.yaml`: A simple CRD that defines a `SimpleConfigMap` resource
- `composition.yaml`: A Composition that uses the Skyhook function to transform data
- `sample.yaml`: A sample `SimpleConfigMap` resource for testing

## License

MIT
