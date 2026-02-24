# Quick Start

Get started with NXpose in less than 5 minutes.

## Prerequisites

- NXpose client installed (see [Installation](installation.md))
- Access to an NXpose server (public or self-hosted)
- A local service to expose (e.g., a web server on port 3000)

## Step 1: Register

Register with the NXpose server to obtain authentication credentials:

```bash
nxpose register
```

This will:

1. Open your browser for OAuth authentication
2. Download your client certificates
3. Save credentials to `~/.nxpose/config.json`

!!! info "Using a Custom Server"
    If you're using a self-hosted server:
    ```bash
    nxpose register --server https://nxpose.your-domain.com
    ```

## Step 2: Start Your Local Service

For this example, let's start a simple HTTP server:

```bash
# Using Python
python3 -m http.server 3000

# Using Node.js
npx http-server -p 3000

# Or use your own application
npm start  # Running on port 3000
```

## Step 3: Create a Tunnel

Expose your local service to the internet:

```bash
nxpose expose http 3000
```

You'll see output like:

```
Tunnel created successfully!
Public URL: https://abc123def456.nxpose.example.com
Forwarding to: http://localhost:3000
Status: Active

Press Ctrl+C to stop the tunnel
```

## Step 4: Test Your Tunnel

Open the public URL in your browser or test with curl:

```bash
curl https://abc123def456.nxpose.example.com
```

You should see the response from your local service!

## Common Use Cases

### Webhook Testing

```bash
# Start your webhook handler locally
nxpose expose http 3000

# Copy the public URL
# Configure it in your webhook provider (GitHub, Stripe, etc.)
# Receive webhooks directly in your local environment
```

### Custom Subdomain

```bash
nxpose expose http 3000 --subdomain myapp
# URL: https://myapp.nxpose.example.com
```

### Skip Service Check

Get the URL before starting your service:

```bash
nxpose expose http 3000 --skip-local-check
# Configure webhooks with the returned URL
# Then start your service
```

### TCP Tunnels

Expose TCP services like databases or SSH:

```bash
# Expose SSH
nxpose expose tcp 22

# Expose PostgreSQL
nxpose expose tcp 5432
```

## Next Steps

- [Client Configuration](client/configuration.md) - Learn about client configuration options
- [Server Setup](server/setup.md) - Set up your own NXpose server
- [Use Cases](use-cases.md) - Explore more ways to use NXpose
- [FAQ](faq.md) - Common questions and answers

## Troubleshooting

### Connection Refused

If you see "connection refused", make sure your local service is running:

```bash
# Check if the port is listening
lsof -i :3000  # Linux/macOS
netstat -an | findstr 3000  # Windows
```

### Authentication Failed

Re-register with the server:

```bash
nxpose register --force
```

### SSL Certificate Errors

For self-hosted servers with self-signed certificates:

```bash
nxpose register --insecure
nxpose expose http 3000 --insecure
```

!!! warning
    Only use `--insecure` for development environments. Always use proper SSL certificates in production.
