# nxpose

[![Build](https://github.com/altista-tech/nxpose/actions/workflows/build-packages.yml/badge.svg)](https://github.com/altista-tech/nxpose/actions/workflows/build-packages.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/altista-tech/nxpose)](https://goreportcard.com/report/github.com/altista-tech/nxpose)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

A secure, self-hosted tunneling service to expose local services to the internet. Think ngrok, but you own the infrastructure.

## Features

- **HTTP and TCP tunneling** - Expose local web servers or TCP services through public URLs
- **Automatic TLS** - Let's Encrypt integration for production certificates, or self-signed for development
- **OAuth2 authentication** - Register clients via GitHub or Google
- **Client certificates** - Mutual TLS for secure tunnel connections
- **Subdomain routing** - Each tunnel gets a unique subdomain
- **Tunnel management** - Configurable per-user and per-client limits, automatic expiration
- **MongoDB and Redis backends** - Persistent storage for users, tunnels, and sessions

## Quick Start

### Install the Client

```bash
# Build from source
go install nxpose/cmd/client@latest

# Or clone and build
git clone https://github.com/altista-tech/nxpose.git
cd nxpose
cd cmd/client && go build -o nxpose
```

### Expose a Local Service

```bash
# Register with a server
nxpose register --server your-server.com:8443

# Expose a local HTTP service
nxpose expose http 8080

# Expose a local TCP service
nxpose expose tcp 5432
```

### Run Your Own Server

```bash
# Build the server
cd cmd/server && go build -o nxpose-server

# Copy and edit the example config
cp server-config.example.yaml server-config.yaml

# Start the server
./nxpose-server --config server-config.yaml
```

## Building

```bash
# Build both client and server
make build

# Build client binaries for all platforms
make build-clients

# Build packages (DEB/RPM for Linux, PKG for macOS)
make package

# Build for all architectures
make all-arch
```

## Testing

```bash
# Run unit tests
go test ./...

# Run integration tests (requires Docker with MongoDB and Redis)
make test-integration

# Run all tests (unit + integration)
make test-all

# Linting
go vet ./...

# Format code
make fmt
```

Integration tests use the `integration` build tag and require Docker to run MongoDB and Redis containers. They test end-to-end tunnel creation, data forwarding, concurrent clients, and cleanup under load.

## Dev Container

The project includes a VS Code dev container for one-click development setup:

1. Open the project in VS Code
2. When prompted, click "Reopen in Container" (or run the "Dev Containers: Reopen in Container" command)
3. The container includes Go 1.24, Docker-in-Docker, MongoDB, and Redis
4. Ports 8443 (server), 27017 (MongoDB), and 6379 (Redis) are forwarded automatically

Alternatively, use the Docker Compose setup directly:

```bash
docker compose -f .devcontainer/docker-compose.yml up
```

## Admin Panel

nxpose includes a built-in admin panel for self-hosted instances. Enable it in your server config:

```yaml
admin:
  enabled: true
  path_prefix: /admin
  auth_method: basic  # "basic", "oauth", or "none"
  username: admin
  password: changeme
```

The admin panel provides:
- **Dashboard** - Live tunnel count, active connections, bytes transferred, uptime
- **Tunnel management** - List, inspect, and kill active tunnels
- **Client list** - Connected clients, their tunnels, and last active time
- **Server settings** - View current configuration and toggle maintenance mode

Dashboard stats update in real time via HTMX polling.

## Configuration

The server is configured via YAML file. See [`server-config.example.yaml`](server-config.example.yaml) for all options.

Key settings:
- **Bind address and port** - Where the server listens (default: `0.0.0.0:8443`)
- **Base domain** - Domain for tunnel subdomains
- **TLS** - Self-signed or Let's Encrypt certificates
- **OAuth2 providers** - GitHub and Google client credentials
- **MongoDB** - Connection URI for persistent storage
- **Redis** - Optional session and cache backend
- **Tunnel limits** - Max tunnels per client (default: 5) and per user (default: 10)
- **Admin panel** - Enable/disable, path prefix, authentication method

Environment variables with the `NXPOSE_` prefix override config file values.

## Project Structure

```
cmd/
  client/         # CLI client for creating tunnels
  server/         # Tunnel server
internal/
  admin/          # Admin panel (HTMX handlers, templates, static assets)
  config/         # Configuration loading and validation
  crypto/         # TLS, certificates, OAuth2, DNS providers
  integration/    # Integration tests (Docker-based)
  logger/         # Structured logging setup
  protocol/       # Tunnel protocol messages and serialization
  server/         # HTTP handlers, WebSocket tunneling, session management
  tunnel/         # Tunnel lifecycle, TCP forwarding, cleanup
site/             # MkDocs documentation site
.devcontainer/    # VS Code dev container configuration
.github/workflows # CI/CD pipelines (tests, package builds)
```

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for development setup, code style, and pull request guidelines.

## License

[MIT](LICENSE)
