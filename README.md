# NXpose

NXpose is a secure tunneling service written in Go that allows you to expose local services to the internet through encrypted tunnels.

## Features

- **Secure encrypted tunnels**: All traffic is protected using TLS encryption
- **Instant exposure**: Quickly expose any local service to the internet
- **Webhook testing**: Perfect for testing webhook integrations with your local development environment
- **Multi-protocol support**: Works with HTTP, HTTPS, TCP and more
- **Custom subdomains**: Generate random or specify your own subdomains for easy access

## Installation

### From Source

```bash
# Clone the repository
git clone https://github.com/yourusername/nxpose.git
cd nxpose

# Build the client
cd cmd/client
go build -o nxpose

# Build the server (if you want to run your own server)
cd ../server
go build -o nxpose-server
```

### Using Go Install

```bash
go install github.com/yourusername/nxpose/cmd/client@latest
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

### Server Configuration

To run your own NXpose server, create a `server-config.yaml` file (see example below) and run:

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