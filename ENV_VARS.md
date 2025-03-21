# Environment Variables Configuration

This document describes the environment variables that can be used to configure the Crossplane Skyhook server.

## Configuration Library

The server uses the [envconfig](https://github.com/kelseyhightower/envconfig) library to handle environment variable configuration. This library automatically maps environment variables to struct fields based on struct tags, providing a more robust and maintainable configuration system.

## Configuration Hierarchy

The configuration is loaded in the following order, with later sources taking precedence:

1. Default values (defined in struct tags)
2. Environment variables (processed by envconfig)
3. Command-line flags

## Available Environment Variables

| Environment Variable | Description | Default Value | Example |
|----------------------|-------------|---------------|---------|
| `SKYHOOK_GRPC_ADDRESS` | gRPC server address | `:9443` | `0.0.0.0:9443` |
| `SKYHOOK_TEMP_DIR` | Temporary directory for code files | `$TMPDIR/crossplane-skyhook` | `/tmp/skyhook` |
| `SKYHOOK_GC_INTERVAL` | Garbage collection interval | `5m` | `10m` |
| `SKYHOOK_IDLE_TIMEOUT` | Idle process timeout | `30m` | `1h` |
| `SKYHOOK_TLS_ENABLED` | Enable TLS | `false` | `true` |
| `SKYHOOK_TLS_CERT_FILE` | Path to TLS certificate file | `` | `/certs/tls.crt` |
| `SKYHOOK_TLS_KEY_FILE` | Path to TLS key file | `` | `/certs/tls.key` |
| `SKYHOOK_LOG_LEVEL` | Log level (debug, info, warn, error) | `info` | `debug` |
| `SKYHOOK_LOG_FORMAT` | Log format (auto, text, json) | `auto` | `json` |
| `SKYHOOK_NODE_SERVER_PORT` | Port for the Node.js HTTP server | `3000` | `3001` |
| `SKYHOOK_HEALTH_CHECK_WAIT` | Timeout for health check | `30s` | `1m` |
| `SKYHOOK_HEALTH_CHECK_INTERVAL` | Interval for health check polling | `500ms` | `1s` |
| `SKYHOOK_NODE_REQUEST_TIMEOUT` | Timeout for Node.js requests | `30s` | `1m` |

## Legacy Environment Variables

For backward compatibility, the following legacy environment variables are still supported:

| Legacy Environment Variable | Description | Equivalent New Variable |
|----------------------------|-------------|-------------------------|
| `TLS_SERVER_CERTS_DIR` | Directory containing TLS certificates | Sets `SKYHOOK_TLS_ENABLED=true` and configures cert/key paths |

## Duration Format

Duration values should be specified using Go's duration format:

- `s` - seconds
- `m` - minutes
- `h` - hours

Examples: `30s`, `5m`, `1h30m`

## Boolean Format

Boolean values can be specified as:

- `true` or `1` for true
- `false` or `0` for false

## Example Usage

```bash
# Basic configuration
export SKYHOOK_GRPC_ADDRESS=":8443"
export SKYHOOK_LOG_LEVEL="debug"

# TLS configuration
export SKYHOOK_TLS_ENABLED="true"
export SKYHOOK_TLS_CERT_FILE="/path/to/cert.pem"
export SKYHOOK_TLS_KEY_FILE="/path/to/key.pem"

# Node.js configuration
export SKYHOOK_NODE_SERVER_PORT="3001"
export SKYHOOK_NODE_REQUEST_TIMEOUT="1m"

# Start the server
./server
```
