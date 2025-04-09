# Using NXpose

This guide covers how to use NXpose for creating and managing secure tunnels. It includes common use cases and best practices.

## Client Commands

The NXpose client provides a command-line interface for creating and managing tunnels. Here are the main commands:

### Help Command

Get help on available commands and options:

```bash
nxpose help
```

### Version Command

Check the version of your NXpose client:

```bash
nxpose version
```

### Registration

Before using NXpose, you need to register with a server:

```bash
# Register with default server (from config)
nxpose register

# Register with specific server
nxpose register --server nxpose.example.com:8443

# Register with custom subdomain (if supported by server)
nxpose register --subdomain myapp
```

### Creating Tunnels

The main functionality of NXpose is creating tunnels to expose local services:

#### HTTP Tunnels

Expose a local HTTP service:

```bash
# Expose a local service running on port 3000
nxpose expose http 3000

# Expose with a custom subdomain
nxpose expose http 3000 --subdomain myapp

# Expose to a specific path on the server
nxpose expose http 3000 --path /api

# Expose with authentication (basic auth)
nxpose expose http 3000 --auth username:password
```

#### HTTPS Tunnels

Expose a local HTTPS service:

```bash
# Expose a local HTTPS service running on port 8443
nxpose expose https 8443

# Skip TLS verification for the local service
nxpose expose https 8443 --skip-verify
```

#### TCP Tunnels

Expose any TCP service:

```bash
# Expose a local TCP service running on port 5432 (e.g., PostgreSQL)
nxpose expose tcp 5432
```

#### Advanced Options

NXpose provides several advanced options for tunnel creation:

```bash
# Skip the local service availability check
# Useful when you want to obtain the URL before starting your local service
nxpose expose http 3000 --skip-local-check

# Set a custom hostname for the tunnel
nxpose expose http 3000 --hostname custom.example.com

# Limit tunnel access to specific IP addresses
nxpose expose http 3000 --allow-cidrs 192.168.1.0/24,10.0.0.0/8

# Add custom headers to forwarded requests
nxpose expose http 3000 --header "X-Forwarded-From: NXpose"

# Run in verbose mode for debugging
nxpose expose http 3000 --verbose
```

### Managing Tunnels

List and manage your active tunnels:

```bash
# List all active tunnels
nxpose list

# Get details about a specific tunnel
nxpose info myapp

# Close a specific tunnel
nxpose close myapp

# Close all active tunnels
nxpose close-all
```

### Tunnel Status

Check the status of your tunnels:

```bash
# Get status of all tunnels
nxpose status

# Get status of a specific tunnel
nxpose status myapp
```

## Server Management

If you're running your own NXpose server, these commands will help you manage it:

### Starting the Server

Start the NXpose server with a configuration file:

```bash
# Start with default configuration
nxpose-server

# Start with a specific configuration file
nxpose-server --config /path/to/server-config.yaml

# Start with verbose logging
nxpose-server --verbose
```

### Managing Server

Administrative commands for server management:

```bash
# List all active tunnels on the server
nxpose-server admin tunnels

# List all registered clients
nxpose-server admin clients

# Revoke a client's access
nxpose-server admin revoke client-id
```

### Monitoring the Server

Monitor the server's status and performance:

```bash
# Check server status
nxpose-server status

# Watch server metrics
nxpose-server metrics
```

## Common Use Cases

### Local Development with Webhooks

When developing an application that receives webhooks from external services:

1. Start your local application:
   ```bash
   # Example: Start a Node.js application
   npm run dev
   ```

2. Create a tunnel to your local service:
   ```bash
   nxpose expose http 3000 --subdomain webhook-dev
   ```

3. Configure the external service to send webhooks to your NXpose URL:
   ```
   https://webhook-dev.nxpose.example.com/webhook
   ```

4. Receive and process webhooks locally without deploying your application.

### Sharing Your Application with Team Members

Share your work-in-progress with team members or clients:

1. Start your local application
2. Create a tunnel:
   ```bash
   nxpose expose http 3000 --subdomain team-demo
   ```
3. Share the URL `https://team-demo.nxpose.example.com` with your team

### Exposing TCP Services

Access databases or other TCP services remotely:

1. Start your local database (e.g., PostgreSQL on port 5432)
2. Create a TCP tunnel:
   ```bash
   nxpose expose tcp 5432
   ```
3. Connect to the exposed TCP port using the assigned port on the NXpose server

## Troubleshooting

### Connection Issues

If you're having trouble connecting to the NXpose server:

1. Verify your server configuration:
   ```bash
   cat ~/.nxpose/config.yaml
   ```

2. Check if the server is accessible:
   ```bash
   curl -k https://nxpose.example.com:8443/ping
   ```

3. Try registering again:
   ```bash
   nxpose register --force
   ```

### Tunnel Problems

If your tunnels aren't working correctly:

1. Check tunnel status:
   ```bash
   nxpose status
   ```

2. Verify that your local service is running and accessible:
   ```bash
   curl http://localhost:3000
   ```

3. Use verbose mode for debugging:
   ```bash
   nxpose expose http 3000 --verbose
   ```

4. Check the server logs for errors:
   ```bash
   tail -f /var/log/nxpose/server.log
   ```

## Best Practices

For the best experience with NXpose, follow these practices:

1. **Secure your exposures**: Only expose services when needed, and consider using authentication
2. **Use custom subdomains**: Makes your services easier to remember and share
3. **Monitor your tunnels**: Regularly check active tunnels to close any that are no longer needed
4. **Update regularly**: Keep both client and server updated for the latest features and security fixes 