<div align="center">
  <h1>NXpose</h1>
  <p>Secure Tunneling Service for Local Development</p>
</div>

NXpose is a powerful, secure tunneling service written in Go that exposes local services to the internet through encrypted tunnels. Perfect for webhook testing, sharing local environments, and remote access to development services.

- **Secure encrypted tunnels**: All traffic protected with TLS 1.2+ encryption
- **OAuth2 authentication**: Secure user authentication with GitHub and Google
- **Multi-protocol support**: HTTP, HTTPS, and TCP tunneling
- **Let's Encrypt integration**: Automatic SSL certificate management with wildcard support
- **Session management**: Redis-backed sessions for scalability
- **MongoDB storage**: Persistent user and tunnel metadata
- **Custom subdomains**: Automatic generation or user-specified subdomains
- **Tunnel limits**: Configurable per-user and per-client restrictions
- **Health monitoring**: Automatic cleanup of inactive tunnels
- **Cross-platform**: Native support for Linux, macOS, and Windows

---

<div align="center">

[![Build Status](https://github.com/yourusername/nxpose/workflows/CI/badge.svg)](https://github.com/yourusername/nxpose/actions)
[![Go Report Card](https://goreportcard.com/badge/github.com/yourusername/nxpose)](https://goreportcard.com/report/github.com/yourusername/nxpose)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

</div>

## Quick Start

```bash
# Install the client
go install github.com/yourusername/nxpose/cmd/client@latest

# Register with the server
nxpose register

# Expose a local HTTP service on port 3000
nxpose expose http 3000

# Get your public URL (e.g., https://abc123.nxpose.example.com)
```

## Table of Contents

- [Installation](#installation)
  - [From Prebuilt Binaries](#from-prebuilt-binaries)
  - [From Packages](#from-packages)
  - [From Source](#from-source)
  - [Using Go Install](#using-go-install)
- [Client Usage](#client-usage)
  - [Registration](#registration)
  - [Exposing Services](#exposing-services)
  - [TCP Tunnels](#tcp-tunnels)
- [Server Setup](#server-setup)
  - [Configuration](#configuration)
  - [TLS Certificates](#tls-certificates)
  - [OAuth2 Setup](#oauth2-setup)
  - [Database Setup](#database-setup)
- [Architecture](#architecture)
- [Use Cases](#use-cases)
- [Building from Source](#building-from-source)
- [Configuration Reference](#configuration-reference)
- [Security](#security)
- [Contributing](#contributing)
- [License](#license)

## Installation

### From Prebuilt Binaries

Download the latest release for your platform from the [releases page](https://github.com/yourusername/nxpose/releases).

```bash
# Linux AMD64
wget https://github.com/yourusername/nxpose/releases/download/v0.1.0/nxpose-linux-amd64.tar.gz
tar xzf nxpose-linux-amd64.tar.gz
sudo mv nxpose /usr/local/bin/

# macOS (Apple Silicon)
wget https://github.com/yourusername/nxpose/releases/download/v0.1.0/nxpose-darwin-arm64.tar.gz
tar xzf nxpose-darwin-arm64.tar.gz
sudo mv nxpose /usr/local/bin/

# Windows
# Download nxpose-windows-amd64.zip and extract to your PATH
```

### From Packages

For Linux systems, DEB packages are available for both AMD64 and ARM64 architectures:

```bash
# Debian/Ubuntu AMD64
wget https://github.com/yourusername/nxpose/releases/download/v0.1.0/nxpose_0.1.0_linux_amd64.deb
sudo dpkg -i nxpose_0.1.0_linux_amd64.deb

# Debian/Ubuntu ARM64
wget https://github.com/yourusername/nxpose/releases/download/v0.1.0/nxpose_0.1.0_linux_arm64.deb
sudo dpkg -i nxpose_0.1.0_linux_arm64.deb
```

### From Source

Requires Go 1.21 or later:

```bash
# Clone the repository
git clone https://github.com/yourusername/nxpose.git
cd nxpose

# Build both client and server
make build

# The binaries will be in ./bin directory
./bin/nxpose --help
./bin/nxpose-server --help

# Install to your system
sudo make install
```

### Using Go Install

```bash
# Install client
go install github.com/yourusername/nxpose/cmd/client@latest

# Install server
go install github.com/yourusername/nxpose/cmd/server@latest
```

## Client Usage

### Registration

Before creating tunnels, register with the NXpose server to obtain authentication certificates:

```bash
# Register using OAuth2 (recommended)
nxpose register

# This will:
# 1. Open your browser for OAuth authentication
# 2. Download your client certificates
# 3. Save credentials to ~/.nxpose/config.json
```

For non-interactive environments:

```bash
# Use a specific server
nxpose register --server https://nxpose.example.com

# Specify custom config directory
nxpose register --config-dir /path/to/config
```

### Exposing Services

#### HTTP Services

Expose a local HTTP service:

```bash
# Basic HTTP tunnel on port 3000
nxpose expose http 3000

# Output:
# Tunnel created successfully!
# Public URL: https://random-subdomain.nxpose.example.com
# Forwarding to: http://localhost:3000
```

#### HTTPS Services

Expose a local HTTPS service:

```bash
# HTTPS tunnel (nxpose handles TLS termination)
nxpose expose https 8443

# For self-signed certificates, use --insecure
nxpose expose https 8443 --insecure
```

#### Custom Subdomains

Request a specific subdomain (subject to availability):

```bash
nxpose expose http 3000 --subdomain myapp

# Your URL: https://myapp.nxpose.example.com
```

#### Skip Local Service Check

Get the public URL before starting your local service (useful for webhook configuration):

```bash
nxpose expose http 3000 --skip-local-check

# Configure webhooks with the returned URL
# Then start your local service on port 3000
```

### TCP Tunnels

Expose arbitrary TCP services:

```bash
# Expose SSH server
nxpose expose tcp 22

# Expose PostgreSQL
nxpose expose tcp 5432

# Expose custom TCP service
nxpose expose tcp 8080
```

TCP tunnels are assigned a random port on the server:

```
Tunnel created successfully!
Public Address: tcp://nxpose.example.com:15234
Forwarding to: tcp://localhost:22
```

### Additional Options

```bash
# Use custom server URL
nxpose expose http 3000 --server https://custom-server.com

# Specify connection timeout
nxpose expose http 3000 --timeout 30s

# Enable verbose logging
nxpose expose http 3000 --verbose

# Use custom config directory
nxpose expose http 3000 --config-dir /path/to/config
```

## Server Setup

### Configuration

The server can be configured using a YAML file or environment variables.

#### Using Configuration File

Create `server-config.yaml`:

```yaml
server:
  bind_address: "0.0.0.0"
  port: 8443
  domain: "nxpose.example.com"

tls:
  cert: "/etc/nxpose/certs/server.crt"
  key: "/etc/nxpose/certs/server.key"

oauth2:
  enabled: true
  redirect_url: "https://nxpose.example.com/auth/callback"
  session_key: "your-random-secret-key"
  session_store: "redis"
  providers:
    - name: "github"
      client_id: "${GITHUB_CLIENT_ID}"
      client_secret: "${GITHUB_CLIENT_SECRET}"

mongodb:
  enabled: true
  uri: "mongodb://localhost:27017"
  database: "nxpose"

redis:
  enabled: true
  host: "localhost"
  port: 6379
  key_prefix: "nxpose:"

letsencrypt:
  enabled: true
  email: "admin@example.com"
  environment: "production"
  storage_dir: "/etc/nxpose/certificates"

tunnels:
  max_per_user: 10
  max_per_client: 5
  expiration_hours: 24
  inactive_timeout_mins: 60

logging:
  level: "info"
  format: "json"

access_control:
  require_auth: true
  allow_registration: true
```

Run the server:

```bash
nxpose-server --config server-config.yaml
```

#### Using Environment Variables

```bash
# Basic configuration
export NXPOSE_SERVER_BIND_ADDRESS="0.0.0.0"
export NXPOSE_SERVER_PORT=8443
export NXPOSE_SERVER_DOMAIN="nxpose.example.com"

# TLS configuration
export NXPOSE_TLS_CERT="/etc/nxpose/certs/server.crt"
export NXPOSE_TLS_KEY="/etc/nxpose/certs/server.key"

# OAuth2
export NXPOSE_OAUTH2_ENABLED=true
export NXPOSE_OAUTH2_SESSION_KEY="your-random-secret-key"
export GITHUB_CLIENT_ID="your-github-client-id"
export GITHUB_CLIENT_SECRET="your-github-client-secret"

# MongoDB
export NXPOSE_MONGODB_ENABLED=true
export NXPOSE_MONGODB_URI="mongodb://localhost:27017"

# Redis
export NXPOSE_REDIS_ENABLED=true
export NXPOSE_REDIS_HOST="localhost"
export NXPOSE_REDIS_PORT=6379

# Let's Encrypt
export NXPOSE_LETSENCRYPT_ENABLED=true
export NXPOSE_LETSENCRYPT_EMAIL="admin@example.com"

# Start server
nxpose-server
```

### TLS Certificates

NXpose supports multiple TLS certificate options:

#### Option 1: Let's Encrypt (Recommended)

Automatic certificate management with wildcard support:

```yaml
letsencrypt:
  enabled: true
  email: "admin@example.com"
  environment: "production"  # or "staging" for testing
  dns_provider: "cloudflare"
  dns_credentials:
    api_token: "${CLOUDFLARE_API_TOKEN}"
```

Supported DNS providers:
- **Cloudflare**: Set `CLOUDFLARE_API_TOKEN`
- **DigitalOcean**: Set `DO_AUTH_TOKEN`

DNS-01 challenges enable:
- Wildcard certificates (`*.nxpose.example.com`)
- Certificate generation without port 80 access
- Support for servers behind firewalls

#### Option 2: Static Certificates

Use your own certificates:

```yaml
tls:
  cert: "/path/to/certificate.pem"
  key: "/path/to/private-key.pem"
```

#### Option 3: Self-Signed (Development Only)

The server automatically generates self-signed certificates if no other option is configured:

```bash
# Just start the server
nxpose-server --config server-config.yaml

# Clients must use --insecure flag
nxpose register --insecure
```

### OAuth2 Setup

#### GitHub OAuth

1. Create a new OAuth App at https://github.com/settings/developers
2. Set Authorization callback URL to `https://your-domain.com/auth/callback`
3. Copy Client ID and Client Secret to your configuration:

```yaml
oauth2:
  enabled: true
  redirect_url: "https://nxpose.example.com/auth/callback"
  session_key: "generate-a-random-secret-key"
  providers:
    - name: "github"
      client_id: "your-github-client-id"
      client_secret: "your-github-client-secret"
      scopes:
        - "user:email"
        - "read:user"
```

#### Google OAuth

1. Create a project at https://console.cloud.google.com/
2. Enable Google+ API
3. Create OAuth 2.0 credentials
4. Add authorized redirect URI: `https://your-domain.com/auth/callback`
5. Configure:

```yaml
oauth2:
  providers:
    - name: "google"
      client_id: "your-google-client-id.apps.googleusercontent.com"
      client_secret: "your-google-client-secret"
      scopes:
        - "https://www.googleapis.com/auth/userinfo.email"
        - "https://www.googleapis.com/auth/userinfo.profile"
```

### Database Setup

#### MongoDB

Required for user and tunnel metadata storage:

```bash
# Using Docker
docker run -d \
  --name nxpose-mongo \
  -p 27017:27017 \
  -v nxpose-data:/data/db \
  mongo:latest

# Configuration
mongodb:
  enabled: true
  uri: "mongodb://localhost:27017"
  database: "nxpose"
  timeout: "10s"
```

#### Redis (Optional but Recommended)

For session management and caching:

```bash
# Using Docker
docker run -d \
  --name nxpose-redis \
  -p 6379:6379 \
  redis:alpine

# Configuration
redis:
  enabled: true
  host: "localhost"
  port: 6379
  db: 0
  key_prefix: "nxpose:"
```

### DNS Configuration

Configure your DNS to point to your NXpose server:

```
; A record for main domain
nxpose.example.com.    A    1.2.3.4

; Wildcard for all subdomains
*.nxpose.example.com.  A    1.2.3.4
```

For testing, you can use hosts file:

```bash
# /etc/hosts
127.0.0.1  nxpose.localhost
127.0.0.1  test.nxpose.localhost
```

## Architecture

### Overview

NXpose uses a client-server architecture with WebSocket-based tunnels:

```
┌─────────────────┐              ┌─────────────────┐
│  External User  │              │  NXpose Server  │
│   (Internet)    │              │  (Public VPS)   │
└────────┬────────┘              └────────┬────────┘
         │                                │
         │ 1. HTTPS Request               │
         │ https://abc.nxpose.com/api     │
         ├───────────────────────────────>│
         │                                │ 2. Route to Tunnel
         │                                │    (WebSocket)
         │                                │         │
         │                                ├─────────┘
         │                                │
         │                                │ 3. Forward Request
         │                         ┌──────┴──────┐
         │                         │   Tunnel    │
         │                         │  (WSS)      │
         │                         └──────┬──────┘
         │                                │
         │                         ┌──────▼──────┐
         │                         │   Client    │
         │                         │ (Developer) │
         │                         └──────┬──────┘
         │                                │
         │                                │ 4. HTTP to localhost
         │                         ┌──────▼──────┐
         │                         │   Local     │
         │                         │  Service    │
         │                         │  :3000      │
         │                         └──────┬──────┘
         │                                │
         │                                │ 5. Response
         │ 6. HTTP Response               │
         │<───────────────────────────────┤
```

### Key Components

#### 1. Client (`cmd/client`)

The NXpose client runs on the developer's machine and:

- Connects to the local service (e.g., `localhost:3000`)
- Establishes secure WebSocket connection to server
- Forwards incoming requests to local service
- Returns responses back through the tunnel
- Handles automatic reconnection with exponential backoff
- Manages client certificates and authentication

#### 2. Server (`cmd/server`)

The NXpose server is a public-facing service that:

- Accepts incoming HTTP/HTTPS requests
- Routes requests to appropriate WebSocket tunnels
- Manages tunnel lifecycle and cleanup
- Handles user authentication (OAuth2)
- Issues and manages TLS certificates (Let's Encrypt)
- Stores user and tunnel data (MongoDB)
- Manages sessions (Redis)
- Monitors tunnel health and connectivity

#### 3. Internal Packages

| Package | Purpose |
|---------|---------|
| `internal/config` | Configuration management for client and server |
| `internal/server` | Core server logic, HTTP handlers, WebSocket management |
| `internal/tunnel` | Client-side tunnel management and connection handling |
| `internal/crypto` | TLS certificates, OAuth2, encryption utilities |
| `internal/protocol` | Protocol definitions and message handlers |
| `internal/logger` | Structured logging with logrus |

### Request Flow

1. **External Request**: User makes HTTP request to `https://abc123.nxpose.example.com/api/test`

2. **Server Routing**:
   - Server extracts subdomain `abc123`
   - Looks up associated tunnel in database
   - Validates tunnel is active and not expired

3. **Tunnel Forwarding**:
   - Server sends `HTTPRequest` message through WebSocket
   - Message includes headers, method, URL path, and body

4. **Client Processing**:
   - Client receives WebSocket message
   - Creates HTTP request to `http://localhost:3000/api/test`
   - Forwards all headers and request body

5. **Local Service**:
   - Developer's application processes the request
   - Returns HTTP response

6. **Response Tunneling**:
   - Client receives HTTP response
   - Sends `HTTPResponse` message through WebSocket
   - Includes status code, headers, and body

7. **External Response**:
   - Server receives WebSocket message
   - Returns HTTP response to original requester

### Security Model

- **Transport Security**: All tunnel connections use TLS 1.2+ with WebSocket Secure (WSS)
- **Authentication**: OAuth2 for user authentication, client certificates for tunnel authentication
- **Authorization**: Per-user tunnel limits, subdomain ownership validation
- **Data Isolation**: Each tunnel is isolated, no cross-tunnel communication
- **Certificate Management**: Automatic Let's Encrypt certificates with DNS-01 challenges
- **Session Security**: Secure cookie handling with Redis-backed sessions

## Use Cases

### Webhook Development

Test webhooks from external services without deploying:

```bash
# Start your local webhook handler
npm start  # Running on localhost:3000

# Create tunnel
nxpose expose http 3000

# Configure webhook in external service (GitHub, Stripe, etc.)
# Webhook URL: https://abc123.nxpose.example.com/webhook

# Receive webhooks directly in your local environment
```

### Team Collaboration

Share your local development environment:

```bash
# Developer starts local app
nxpose expose http 3000

# Share URL with team members
# URL: https://demo-feature.nxpose.example.com

# Team members can test the feature in real-time
```

### Mobile App Development

Test mobile apps against local backend:

```bash
# Backend running locally
nxpose expose http 8080

# Use public URL in mobile app
# API URL: https://myapi.nxpose.example.com
```

### IoT and Remote Access

Access devices behind NAT or firewalls:

```bash
# Expose device SSH access
nxpose expose tcp 22

# Access from anywhere
ssh user@nxpose.example.com -p 15234
```

### Client Demos

Show work to clients without deployment:

```bash
# Expose your local development version
nxpose expose http 3000 --subdomain client-demo

# Share: https://client-demo.nxpose.example.com
```

## Building from Source

### Prerequisites

- Go 1.21 or later
- Make (optional, for using Makefile)

### Build Commands

Using Make (Linux/macOS):

```bash
# Build both client and server
make build

# Build only client
make client

# Build only server
make server

# Build for all platforms
make build-all

# Create DEB packages
make packages

# Run tests
make test

# Clean build artifacts
make clean
```

Using Go commands:

```bash
# Build client
cd cmd/client && go build -o nxpose

# Build server
cd cmd/server && go build -o nxpose-server

# Install client
go install nxpose/cmd/client

# Install server
go install nxpose/cmd/server
```

For Windows users, a batch script is provided:

```cmd
# Build both
build.bat build

# Build client only
build.bat client

# Build server only
build.bat server

# Build all platforms
build.bat build-all

# Clean
build.bat clean
```

### Cross-Compilation

Build for specific platforms:

```bash
# Linux AMD64
GOOS=linux GOARCH=amd64 go build -o nxpose-linux-amd64 ./cmd/client

# Linux ARM64
GOOS=linux GOARCH=arm64 go build -o nxpose-linux-arm64 ./cmd/client

# macOS Intel
GOOS=darwin GOARCH=amd64 go build -o nxpose-darwin-amd64 ./cmd/client

# macOS Apple Silicon
GOOS=darwin GOARCH=arm64 go build -o nxpose-darwin-arm64 ./cmd/client

# Windows
GOOS=windows GOARCH=amd64 go build -o nxpose-windows-amd64.exe ./cmd/client
```

## Configuration Reference

### Environment Variables

#### Server Configuration

| Variable | Description | Default |
|----------|-------------|---------|
| `NXPOSE_SERVER_BIND_ADDRESS` | Address to bind server | `0.0.0.0` |
| `NXPOSE_SERVER_PORT` | Server port | `8443` |
| `NXPOSE_SERVER_DOMAIN` | Base domain for tunnels | `localhost` |
| `NXPOSE_TLS_CERT` | Path to TLS certificate | `""` |
| `NXPOSE_TLS_KEY` | Path to TLS key | `""` |
| `NXPOSE_VERBOSE` | Enable verbose logging | `false` |

#### OAuth2 Configuration

| Variable | Description | Default |
|----------|-------------|---------|
| `NXPOSE_OAUTH2_ENABLED` | Enable OAuth2 | `false` |
| `NXPOSE_OAUTH2_SESSION_KEY` | Session encryption key | Required if OAuth2 enabled |
| `NXPOSE_OAUTH2_SESSION_STORE` | Session store (memory/mongo/redis) | `memory` |
| `GITHUB_CLIENT_ID` | GitHub OAuth client ID | `""` |
| `GITHUB_CLIENT_SECRET` | GitHub OAuth client secret | `""` |
| `GOOGLE_CLIENT_ID` | Google OAuth client ID | `""` |
| `GOOGLE_CLIENT_SECRET` | Google OAuth client secret | `""` |

#### Database Configuration

| Variable | Description | Default |
|----------|-------------|---------|
| `NXPOSE_MONGODB_ENABLED` | Enable MongoDB | `false` |
| `NXPOSE_MONGODB_URI` | MongoDB connection URI | `mongodb://localhost:27017` |
| `NXPOSE_MONGODB_DATABASE` | Database name | `nxpose` |
| `NXPOSE_REDIS_ENABLED` | Enable Redis | `false` |
| `NXPOSE_REDIS_HOST` | Redis host | `localhost` |
| `NXPOSE_REDIS_PORT` | Redis port | `6379` |
| `NXPOSE_REDIS_DB` | Redis database number | `0` |

#### Let's Encrypt Configuration

| Variable | Description | Default |
|----------|-------------|---------|
| `NXPOSE_LETSENCRYPT_ENABLED` | Enable Let's Encrypt | `false` |
| `NXPOSE_LETSENCRYPT_EMAIL` | Email for Let's Encrypt | Required if enabled |
| `NXPOSE_LETSENCRYPT_ENVIRONMENT` | Environment (production/staging) | `production` |
| `CLOUDFLARE_API_TOKEN` | Cloudflare API token for DNS | `""` |
| `DO_AUTH_TOKEN` | DigitalOcean API token for DNS | `""` |

#### Tunnel Configuration

| Variable | Description | Default |
|----------|-------------|---------|
| `NXPOSE_TUNNELS_MAX_PER_USER` | Max tunnels per user | `10` |
| `NXPOSE_TUNNELS_MAX_PER_CLIENT` | Max tunnels per client | `5` |
| `NXPOSE_TUNNELS_EXPIRATION_HOURS` | Tunnel expiration (hours) | `24` |
| `NXPOSE_TUNNELS_MAX_CONNECTION` | Max connection duration | `""` (unlimited) |

### Client Configuration

Client configuration is stored in `~/.nxpose/config.json`:

```json
{
  "server_url": "https://nxpose.example.com",
  "token": "eyJhbGciOiJIUzI1NiIs...",
  "cert_path": "/home/user/.nxpose/client.crt",
  "key_path": "/home/user/.nxpose/client.key",
  "insecure_skip_verify": false
}
```

## Security

### Reporting Security Issues

If you discover a security vulnerability, please email security@example.com instead of using the issue tracker. Security issues will be addressed as a priority.

### Best Practices

#### For Server Operators

1. **Use Let's Encrypt**: Enable automatic certificate management
2. **Enable OAuth2**: Require user authentication for all tunnels
3. **Use Redis Sessions**: Store sessions in Redis, not in memory
4. **Set Tunnel Limits**: Prevent abuse with per-user limits
5. **Monitor Logs**: Enable JSON logging and monitor for suspicious activity
6. **Regular Updates**: Keep NXpose and dependencies up to date
7. **Firewall Rules**: Restrict access to MongoDB and Redis
8. **Strong Session Keys**: Use cryptographically random session keys

#### For Developers

1. **Protect Credentials**: Never commit client certificates or tokens
2. **Use HTTPS Locally**: Test with HTTPS to match production
3. **Limit Exposure Time**: Only expose tunnels when needed
4. **Monitor Active Tunnels**: Regularly check and close unused tunnels
5. **Verify Server**: Always verify the server's SSL certificate
6. **Use Authentication**: Register with OAuth2, don't use legacy registration

### Encryption

- **Transport**: TLS 1.2+ for all connections
- **WebSocket**: WSS (WebSocket Secure) for all tunnels
- **Certificates**: Support for Let's Encrypt, static, and self-signed certificates
- **Cipher Suites**: Configurable with secure defaults

## Dependencies

### Server Dependencies

- `github.com/gorilla/mux` - HTTP routing
- `github.com/gorilla/sessions` - Session management
- `github.com/gorilla/websocket` - WebSocket support
- `github.com/spf13/cobra` - CLI framework
- `github.com/spf13/viper` - Configuration management
- `github.com/sirupsen/logrus` - Structured logging
- `go.mongodb.org/mongo-driver` - MongoDB driver
- `github.com/go-redis/redis/v8` - Redis client
- `golang.org/x/oauth2` - OAuth2 authentication
- `github.com/caddyserver/certmagic` - Let's Encrypt integration

### Client Dependencies

- `github.com/spf13/cobra` - CLI framework
- `github.com/gorilla/websocket` - WebSocket client
- `github.com/sirupsen/logrus` - Logging
- `github.com/google/uuid` - UUID generation

## Contributing

Contributions are welcome! Here's how you can help:

### Reporting Bugs

1. Check if the bug is already reported in [Issues](https://github.com/yourusername/nxpose/issues)
2. Create a new issue with:
   - Clear description of the problem
   - Steps to reproduce
   - Expected vs actual behavior
   - Environment details (OS, Go version, NXpose version)
   - Relevant logs

### Suggesting Features

1. Open an issue tagged `enhancement`
2. Describe the feature and use case
3. Explain why it would be useful
4. Provide examples if possible

### Pull Requests

1. Fork the repository
2. Create a feature branch: `git checkout -b feature/amazing-feature`
3. Make your changes
4. Add tests for new functionality
5. Ensure all tests pass: `go test ./...`
6. Format code: `go fmt ./...`
7. Commit with clear messages
8. Push to your fork
9. Open a Pull Request

### Development Setup

```bash
# Clone your fork
git clone https://github.com/yourusername/nxpose.git
cd nxpose

# Install dependencies
go mod download

# Run tests
go test ./...

# Run linting
go vet ./...

# Build
make build

# Test client and server locally
./bin/nxpose-server --config server-config.example.yaml
./bin/nxpose register --server https://localhost:8443 --insecure
```

### Code Style

- Follow [Effective Go](https://golang.org/doc/effective_go.html) guidelines
- Use `gofmt` for formatting
- Write clear, descriptive comments
- Add tests for new features
- Keep functions focused and small
- Use meaningful variable names

## FAQ

### Can I self-host NXpose?

Yes! NXpose is open source and designed to be self-hosted. Follow the [Server Setup](#server-setup) section to deploy your own instance.

### Is NXpose production-ready?

NXpose is suitable for development and testing. For production use cases, ensure proper security configuration, monitoring, and resource limits.

### How many tunnels can I create?

By default, users can create up to 10 tunnels. Server operators can configure this limit via `tunnels.max_per_user`.

### Do tunnels expire?

Yes, tunnels expire after 24 hours by default. This can be configured via `tunnels.expiration_hours`.

### Can I use custom domains?

Currently, NXpose uses subdomains under the server's base domain. Custom domain support is planned for a future release.

### Is there a rate limit?

Rate limiting depends on server configuration. Contact your NXpose server operator for specific limits.

### How do I delete a tunnel?

Tunnels are automatically deleted when the client disconnects or when they expire. Manual deletion via API is planned for a future release.

### Can I use NXpose with Docker?

Yes! Both client and server can run in Docker containers. Docker images will be published soon.

## Roadmap

- [ ] Docker images for client and server
- [ ] Custom domain support
- [ ] Web dashboard for tunnel management
- [ ] Tunnel analytics and logging
- [ ] HTTP/2 and gRPC support
- [ ] WebSocket proxying
- [ ] Custom authentication providers
- [ ] Team/organization support
- [ ] API for tunnel management
- [ ] Kubernetes operator

## License

NXpose is released under the [MIT License](LICENSE).

## Acknowledgments

- Inspired by [ngrok](https://ngrok.com/), [localtunnel](https://localtunnel.me/), and [reproxy](https://reproxy.io)
- Built with [Go](https://golang.org/) and amazing open source libraries
- Documentation site built with [MkDocs Material](https://squidfunk.github.io/mkdocs-material/)

## Support

- **Issues**: [GitHub Issues](https://github.com/yourusername/nxpose/issues)
- **Discussions**: [GitHub Discussions](https://github.com/yourusername/nxpose/discussions)
- **Email**: support@example.com

---

<div align="center">
Made with ❤️ by the NXpose community
</div>
