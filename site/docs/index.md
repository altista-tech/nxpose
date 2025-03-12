# NXpose

**NXpose is a self-hosted secure tunneling service** that exposes local services to the internet through encrypted tunnels. Written in Go, distributed as a single binary and packages for Linux, macOS, and Windows.

- Automatic SSL termination with Let's Encrypt
- Mutual TLS client certificates
- HTTP tunnels with automatic subdomain routing
- TCP tunnels for databases, SSH, and any TCP service
- OAuth2 authentication (GitHub, Google)
- Built-in admin panel with live tunnel monitoring
- Docker, DEB, RPM, and macOS PKG packages
- Session storage via MongoDB or Redis
- Single binary, low memory footprint, cross-platform

NXpose consists of two components: the **server** (`nxpose-server`) runs on a public host and accepts tunnel connections, and the **client** (`nxpose`) runs on your machine and creates tunnels to expose local ports.

## Install

NXpose client is distributed as a self-contained binary for linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, and windows/amd64.

### Binary download

Download the latest binary from the [releases page](https://github.com/altista-tech/nxpose/releases):

```bash
# Linux AMD64
curl -LO https://github.com/altista-tech/nxpose/releases/latest/download/nxpose-linux-amd64
chmod +x nxpose-linux-amd64
sudo mv nxpose-linux-amd64 /usr/local/bin/nxpose

# Linux ARM64
curl -LO https://github.com/altista-tech/nxpose/releases/latest/download/nxpose-linux-arm64
chmod +x nxpose-linux-arm64
sudo mv nxpose-linux-arm64 /usr/local/bin/nxpose

# macOS Apple Silicon
curl -LO https://github.com/altista-tech/nxpose/releases/latest/download/nxpose-darwin-arm64
chmod +x nxpose-darwin-arm64
sudo mv nxpose-darwin-arm64 /usr/local/bin/nxpose

# macOS Intel
curl -LO https://github.com/altista-tech/nxpose/releases/latest/download/nxpose-darwin-amd64
chmod +x nxpose-darwin-amd64
sudo mv nxpose-darwin-amd64 /usr/local/bin/nxpose
```

For Windows, download `nxpose-windows-amd64.exe` from the [releases page](https://github.com/altista-tech/nxpose/releases) and add it to your PATH.

### Homebrew (macOS)

```bash
brew install nxrvl/tap/nxpose
```

### Go install

```bash
go install github.com/altista-tech/nxpose/cmd/client@latest
```

### Verify installation

```bash
nxpose --help
```

## Quick Start

### 1. Register with a server

Register with an NXpose server to obtain client certificates. This opens your browser for OAuth authentication:

```bash
nxpose register
```

Credentials are saved to `~/.nxpose/config.yaml`. For a custom server:

```bash
nxpose register --server your-server.com --port 8443
```

### 2. Expose a local service

Start a tunnel to your local HTTP service:

```bash
nxpose expose http 3000
```

Output:

```
Tunnel created successfully!
Public URL: https://abc123def456.nxpose.example.com
Forwarding to: http://localhost:3000
Status: Active

Press Ctrl+C to stop the tunnel
```

Your local port 3000 is now publicly accessible at the given URL.

## HTTP Tunnels

Expose any local HTTP service to the internet. NXpose creates a unique subdomain on the server's domain and proxies all incoming requests to your local port.

```bash
# Basic usage
nxpose expose http 8080

# Skip checking if the local port is open (useful for pre-configuring webhooks)
nxpose expose http 3000 --skip-local-check

# Keep the tunnel alive with automatic reconnection
nxpose expose http 3000 --keep-alive
```

The `--skip-local-check` flag is useful when you need the public URL before starting your service — for example, to configure a webhook endpoint in GitHub or Stripe, then start your handler.

## TCP Tunnels

Forward raw TCP connections for databases, SSH, or any TCP-based service. The server assigns a public port from its configured TCP range (default 10000–20000).

```bash
# Expose SSH
nxpose expose tcp 22

# Expose PostgreSQL
nxpose expose tcp 5432

# Expose Redis
nxpose expose tcp 6379

# Expose MySQL
nxpose expose tcp 3306
```

## Server Status

Check the server's status and certificate information:

```bash
nxpose status
```

## Client Configuration

The client config file is created automatically by `nxpose register` at `~/.nxpose/config.yaml`. All settings can be overridden with command-line flags:

| Flag | Description | Default |
|------|-------------|---------|
| `-s, --server` | Server hostname or IP address | `nxpose.naxrevlis.com` |
| `-p, --port` | Server port | `443` |
| `--config` | Config file path | `$HOME/.nxpose/config.yaml` |
| `--tls-cert` | Path to TLS certificate file | from config |
| `--tls-key` | Path to TLS key file | from config |
| `-v, --verbose` | Enable verbose output | `false` |

### Register flags

| Flag | Description | Default |
|------|-------------|---------|
| `--force-new` | Force new certificate registration even if one exists | `false` |
| `--save-cert` | Save certificate and key to disk | `true` |
| `--save-config` | Save registration info to config file | `true` |
| `--skip-oauth` | Skip OAuth authentication (not recommended) | `false` |

### Expose flags

| Flag | Description | Default |
|------|-------------|---------|
| `--keep-alive` | Keep the tunnel running until interrupted | `false` |
| `--skip-local-check` | Skip checking if the local service is available | `false` |

## Troubleshooting

### Connection refused

Your local service is not running or not listening on the expected port:

```bash
# Check if the port is listening
lsof -i :3000          # Linux / macOS
ss -tlnp | grep 3000   # Linux
netstat -an | grep 3000 # Windows
```

### Authentication failed

Re-register with the server to obtain new certificates:

```bash
nxpose register --force-new
```

### SSL certificate errors

For self-hosted servers with self-signed certificates during development:

```bash
nxpose register --server your-server.com --port 8443
```

If the server uses a self-signed certificate, you may need to provide the CA certificate or use the `--tls-cert` flag.

## All Client Options

```
nxpose is a Go-based secure tunneling service that allows exposing
local services to the internet through secure tunnels.

Usage:
  nxpose [command]

Available Commands:
  completion  Generate the autocompletion script for the specified shell
  expose      Expose a local service to the internet
  help        Help about any command
  register    Register with the nxpose server and obtain certificates
  status      Check the status of the nxpose server

Flags:
      --config string     config file (default is $HOME/.nxpose/config.yaml)
  -h, --help              help for nxpose
  -p, --port int          Server port (default 443)
  -s, --server string     Server hostname or IP address (default "nxpose.naxrevlis.com")
      --tls-cert string   Path to TLS certificate file
      --tls-key string    Path to TLS key file
  -v, --verbose           Enable verbose output
```

### nxpose expose

```
Expose a local service running on a specified port to the internet
through a secure tunnel.

Usage:
  nxpose expose [protocol] [port] [flags]

Flags:
  -h, --help               help for expose
      --keep-alive         Keep the tunnel running until interrupted
      --skip-local-check   Skip checking if the local service is available
                           before creating the tunnel
```

### nxpose register

```
Connect to the nxpose server to register and obtain the necessary
certificates for secure tunneling.

By default, OAuth2 authentication will be used if the server supports it.

Usage:
  nxpose register [flags]

Flags:
      --force-new     Force registration of a new certificate even if one exists
  -h, --help          help for register
      --save-cert     Save certificate and key to disk (default true)
      --save-config   Save registration information to config file (default true)
      --skip-oauth    Skip OAuth authentication (not recommended)
```

### nxpose status

```
Connect to the nxpose server to check its status, including
certificate information.

Usage:
  nxpose status [flags]
```
