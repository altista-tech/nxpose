# Self-Hosting Guide

Deploy NXpose on your own infrastructure for full control over your tunneling service.

## Prerequisites

- A server with a public IP address
- A domain name pointing to your server (with wildcard DNS for subdomains)
- Go 1.21+ (for building from source) or Docker

## DNS Configuration

NXpose uses subdomain routing for HTTP tunnels. Configure your DNS with:

```
A     your-domain.com        → your-server-ip
A     *.your-domain.com      → your-server-ip
```

For example, if your domain is `tunnel.example.com`:

```
A     tunnel.example.com     → 203.0.113.10
A     *.tunnel.example.com   → 203.0.113.10
```

## Deployment Options

### Docker Compose (Recommended)

Create a `docker-compose.yml`:

```yaml
version: "3.8"

services:
  nxpose:
    image: ghcr.io/nxrvl/nxpose-server:latest
    ports:
      - "8443:8443"
      - "80:80"          # For Let's Encrypt HTTP challenge
      - "10000-10100:10000-10100"  # TCP tunnel port range
    volumes:
      - ./server-config.yaml:/etc/nxpose/server-config.yaml
      - nxpose-certs:/etc/nxpose/certificates
    environment:
      - NXPOSE_SERVER_DOMAIN=tunnel.example.com
    depends_on:
      - mongodb
      - redis
    restart: unless-stopped

  mongodb:
    image: mongo:7
    volumes:
      - mongo-data:/data/db
    restart: unless-stopped

  redis:
    image: redis:7-alpine
    restart: unless-stopped

volumes:
  mongo-data:
  nxpose-certs:
```

Start the stack:

```bash
docker compose up -d
```

### From Packages

#### Debian / Ubuntu

```bash
wget https://github.com/nxrvl/nxpose/releases/latest/download/nxpose_linux_amd64.deb
sudo dpkg -i nxpose_linux_amd64.deb
```

The DEB package installs the server binary and sets up a systemd service.

#### RHEL / Fedora / CentOS

```bash
wget https://github.com/nxrvl/nxpose/releases/latest/download/nxpose_linux_amd64.rpm
sudo rpm -i nxpose_linux_amd64.rpm
```

#### macOS

```bash
wget https://github.com/nxrvl/nxpose/releases/latest/download/nxpose_darwin_arm64.pkg
sudo installer -pkg nxpose_darwin_arm64.pkg -target /
```

### From Source

```bash
git clone https://github.com/nxrvl/nxpose.git
cd nxpose
make build
sudo cp bin/nxpose-server /usr/local/bin/
```

## Configuration

Copy the example configuration and edit it:

```bash
sudo mkdir -p /etc/nxpose
sudo cp server-config.example.yaml /etc/nxpose/server-config.yaml
sudo nano /etc/nxpose/server-config.yaml
```

### Minimal Configuration

```yaml
server:
  bind_address: "0.0.0.0"
  port: 8443
  domain: "tunnel.example.com"

tls:
  cert: "/etc/nxpose/certs/server.crt"
  key: "/etc/nxpose/certs/server.key"

tunnels:
  max_per_user: 10
```

### With Let's Encrypt

```yaml
server:
  bind_address: "0.0.0.0"
  port: 8443
  domain: "tunnel.example.com"

letsencrypt:
  enabled: true
  email: "admin@example.com"
  environment: "production"
  storage_dir: "/etc/nxpose/certificates"
```

### With OAuth2 and MongoDB

```yaml
server:
  bind_address: "0.0.0.0"
  port: 8443
  domain: "tunnel.example.com"

oauth2:
  enabled: true
  redirect_url: "https://tunnel.example.com/auth/callback"
  session_key: "your-random-secret-key"
  session_store: "mongo"
  providers:
    - name: "github"
      client_id: "your-github-client-id"
      client_secret: "your-github-client-secret"
      scopes:
        - "user:email"
        - "read:user"

mongodb:
  enabled: true
  uri: "mongodb://localhost:27017"
  database: "nxpose"

redis:
  enabled: true
  host: "localhost"
  port: 6379
```

### Environment Variable Overrides

All configuration values can be overridden with environment variables using the `NXPOSE_` prefix:

| Variable | Description |
|----------|-------------|
| `NXPOSE_SERVER_BIND_ADDRESS` | Server bind address |
| `NXPOSE_SERVER_PORT` | Server port |
| `NXPOSE_SERVER_DOMAIN` | Base domain for tunnels |
| `NXPOSE_TLS_CERT` | Path to TLS certificate |
| `NXPOSE_TLS_KEY` | Path to TLS key |
| `NXPOSE_MONGODB_URI` | MongoDB connection URI |
| `NXPOSE_REDIS_HOST` | Redis host |
| `NXPOSE_ADMIN_ENABLED` | Enable admin panel |
| `NXPOSE_ADMIN_PASSWORD` | Admin panel password |

## Enabling the Admin Panel

Add the admin section to your configuration:

```yaml
admin:
  enabled: true
  path_prefix: "/admin"
  auth_method: "basic"
  username: "admin"
  password: "your-secure-password"
```

Access the admin panel at `https://tunnel.example.com/admin`.

See the [Admin Panel Guide](admin-panel.md) for details.

## Running as a Systemd Service

If you installed from source, create a service file:

```ini
# /etc/systemd/system/nxpose.service
[Unit]
Description=NXpose Tunnel Server
After=network.target mongodb.service redis.service

[Service]
Type=simple
User=nxpose
ExecStart=/usr/local/bin/nxpose-server --config /etc/nxpose/server-config.yaml
Restart=on-failure
RestartSec=5
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
```

Enable and start:

```bash
sudo systemctl daemon-reload
sudo systemctl enable nxpose
sudo systemctl start nxpose
```

## Firewall Rules

Open the required ports:

```bash
# HTTPS API and tunnel endpoint
sudo ufw allow 8443/tcp

# HTTP for Let's Encrypt challenges (if using Let's Encrypt)
sudo ufw allow 80/tcp

# TCP tunnel port range
sudo ufw allow 10000:20000/tcp
```

## Verification

Check the server is running:

```bash
curl -k https://localhost:8443/api/status
```

Register a client:

```bash
nxpose register --server your-domain.com:8443
nxpose expose http 3000
```

## Upgrading

### Docker

```bash
docker compose pull
docker compose up -d
```

### Packages

Download and install the new package version. The systemd service will restart automatically.

### From Source

```bash
git pull
make build
sudo cp bin/nxpose-server /usr/local/bin/
sudo systemctl restart nxpose
```
