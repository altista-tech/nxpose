# NXpose

NXpose is a secure tunneling service written in Go that allows you to expose local services to the internet through encrypted tunnels.

## Features

- **Secure encrypted tunnels**: All traffic is protected using TLS encryption
- **Instant exposure**: Quickly expose any local service to the internet
- **Webhook testing**: Perfect for testing webhook integrations with your local development environment
- **Multi-protocol support**: Works with HTTP, HTTPS, TCP and more
- **Custom subdomains**: Generate random or specify your own subdomains for easy access

## Installation

### From Prebuilt Binaries

Pre-built binaries for Linux, macOS, and Windows are available on the [releases page](https://github.com/yourusername/nxpose/releases).

### From Packages

For Linux, APK and RPM packages are available for both AMD64 and ARM64 architectures:

```bash
# Install from RPM (Fedora, CentOS, RHEL)
sudo rpm -i nxpose_version_x86_64.rpm

# Install from APK (Alpine Linux)
sudo apk add --allow-untrusted nxpose_version_x86_64.apk
```

### From Source

```bash
# Clone the repository
git clone https://github.com/yourusername/nxpose.git
cd nxpose

# Build the client and server using Make
make build

# The binaries will be available in the './bin' directory
```

### Using Go Install

```bash
go install github.com/yourusername/nxpose/cmd/client@latest
```

## Using the Makefile

The repository includes a comprehensive Makefile for building binaries for multiple platforms and creating package formats.

### Building Binaries

```bash
# Build both client and server for your current platform
make build

# Build only the server
make server

# Build only the client
make client

# Build for all supported platforms (Linux, macOS, Windows - AMD64 and ARM64)
make build-all
```

### Creating Packages

```bash
# Create APK and RPM packages for both AMD64 and ARM64
make packages

# Create only RPM packages
make rpm

# Create only APK packages
make apk
```

### Other Makefile Commands

```bash
# Install dependencies
make deps

# Run tests
make test

# Clean build artifacts
make clean

# Show available commands
make help
```

### Windows Build Script

For Windows users, a batch script is provided to simplify building without requiring GNU Make:

```cmd
# Build both client and server
build.bat build

# Build only the server
build.bat server

# Build only the client
build.bat client

# Build for all supported platforms
build.bat build-all

# Clean build artifacts
build.bat clean

# Show available commands
build.bat help
```

## Usage

### Client Commands

#### Register with a Server

Before creating tunnels, you need to register with the NXpose server to obtain certificates:

```bash
nxpose register
```

#### Expose a Local Service

To expose a local service running on port 3000 via HTTP:

```bash
nxpose expose http 3000
```

This will create a secure tunnel and provide you with a public URL that forwards to your local service.

#### Skip Local Service Check

When testing webhooks or scenarios where you need to get the URL before starting your local service:

```bash
nxpose expose http 3000 --skip-local-check
```

This will create a tunnel without checking if the local service is running, which is useful when you need the public URL to configure your local application.

### Server Configuration

To run your own NXpose server, you can use either a configuration file or environment variables.

#### Using a Configuration File

Create a `server-config.yaml` file (see example below) and run:

```bash
nxpose-server --config server-config.yaml
```

Example server configuration:

```yaml
# Server settings
server:
  bind_address: "0.0.0.0"
  port: 8443
  domain: "nxpose.example.com"

# TLS settings
tls:
  cert: "/etc/nxpose/certs/server.crt"
  key: "/etc/nxpose/certs/server.key"

# Logging settings
logging:
  level: "info"
  format: "text"

# Enable verbose output for debugging
verbose: false
```

#### Using Environment Variables

You can also configure the server using environment variables:

```bash
# Basic configuration
export NXPOSE_SERVER_BIND_ADDRESS="0.0.0.0"
export NXPOSE_SERVER_PORT=8443
export NXPOSE_SERVER_DOMAIN="nxpose.example.com"
export NXPOSE_TLS_CERT="/path/to/cert.pem"
export NXPOSE_TLS_KEY="/path/to/key.pem"
export NXPOSE_VERBOSE=true

# Start the server (without a config file)
nxpose-server
```

Available environment variables include:

| Environment Variable | Description | Default |
| --- | --- | --- |
| NXPOSE_SERVER_BIND_ADDRESS | Address to bind the server to | 0.0.0.0 |
| NXPOSE_SERVER_PORT | Port to listen on | 8443 |
| NXPOSE_SERVER_DOMAIN | Base domain for tunnels | localhost |
| NXPOSE_TLS_CERT | Path to TLS certificate file | "" |
| NXPOSE_TLS_KEY | Path to TLS key file | "" |
| NXPOSE_VERBOSE | Enable verbose logging | false |
| NXPOSE_LETSENCRYPT_ENABLED | Enable Let's Encrypt | false |
| NXPOSE_LETSENCRYPT_EMAIL | Email for Let's Encrypt | "" |
| NXPOSE_MONGODB_ENABLED | Enable MongoDB | false |
| NXPOSE_MONGODB_URI | MongoDB connection URI | mongodb://localhost:27017 |

## Architecture

NXpose consists of two main components:

1. **Client**: Runs locally on your machine, connecting to your local service and the NXpose server.
2. **Server**: Public-facing component that accepts secure connections from clients and routes external traffic through the established tunnels.

The architecture follows these principles:
- Secure communication using TLS
- Efficient data transfer with minimal overhead
- Reliable connections with automatic reconnection
- Clean separation of concerns between components

## Use Cases

- Develop and test webhook integrations locally
- Share your local development environment with team members or clients
- Expose IoT devices in restricted networks
- Create demos without deploying to production servers
- Test your applications from different locations

## Dependencies

The project uses the following external libraries:
- github.com/spf13/cobra - Command-line interface
- github.com/sirupsen/logrus - Structured logging
- github.com/stretchr/testify - Testing framework
- golang.org/x/net/websocket - WebSocket support
- github.com/google/uuid - UUID generation

## License

[MIT License]

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## Security

If you discover any security related issues, please email security@example.com instead of using the issue tracker.