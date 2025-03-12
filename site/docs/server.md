# Server

The NXpose server (`nxpose-server`) is the public-facing component that accepts client connections, manages tunnels, and proxies traffic. It handles TLS termination, OAuth2 authentication, subdomain routing for HTTP tunnels, and TCP port allocation for TCP tunnels.

**Prerequisites:**

- A server with a public IP address
- A domain name with wildcard DNS configured
- MongoDB for user and session storage
- Optionally: Redis for session caching

## DNS Setup

NXpose routes HTTP tunnels via subdomains. Each tunnel gets a unique subdomain like `abc123.tunnel.example.com`. Configure two DNS A records pointing to your server:

```
A    tunnel.example.com      →  YOUR_SERVER_IP
A    *.tunnel.example.com    →  YOUR_SERVER_IP
```

The wildcard record is required for HTTP tunnel subdomain routing to work.

## Install

### Linux packages

DEB and RPM packages are available from the [releases page](https://github.com/altista-tech/nxpose/releases). Packages install the server binary to `/opt/nxpose/`, configuration to `/etc/nxpose/`, and set up a systemd service.

```bash
# Debian / Ubuntu (AMD64)
curl -LO https://github.com/altista-tech/nxpose/releases/latest/download/nxpose_1.0.0_amd64.deb
sudo dpkg -i nxpose_1.0.0_amd64.deb

# Debian / Ubuntu (ARM64)
curl -LO https://github.com/altista-tech/nxpose/releases/latest/download/nxpose_1.0.0_arm64.deb
sudo dpkg -i nxpose_1.0.0_arm64.deb

# RHEL / Fedora / CentOS (AMD64)
curl -LO https://github.com/altista-tech/nxpose/releases/latest/download/nxpose-1.0.0-1.amd64.rpm
sudo rpm -i nxpose-1.0.0-1.amd64.rpm

# RHEL / Fedora / CentOS (ARM64)
curl -LO https://github.com/altista-tech/nxpose/releases/latest/download/nxpose-1.0.0-1.arm64.rpm
sudo rpm -i nxpose-1.0.0-1.arm64.rpm
```

### macOS package

```bash
# Apple Silicon
curl -LO https://github.com/altista-tech/nxpose/releases/latest/download/nxpose-1.0.0-arm64.pkg
sudo installer -pkg nxpose-1.0.0-arm64.pkg -target /

# Intel
curl -LO https://github.com/altista-tech/nxpose/releases/latest/download/nxpose-1.0.0-amd64.pkg
sudo installer -pkg nxpose-1.0.0-amd64.pkg -target /
```

### From source

Requires Go 1.24+:

```bash
git clone https://github.com/altista-tech/nxpose.git
cd nxpose
cd cmd/server && go build -o nxpose-server
sudo mv nxpose-server /usr/local/bin/
```

### Docker

```bash
docker pull ghcr.io/altista-tech/nxpose-server:latest
```

## Configuration

The server is configured via a YAML file. Copy the example and edit it:

```bash
sudo mkdir -p /etc/nxpose
sudo cp server-config.example.yaml /etc/nxpose/server-config.yaml
```

### Minimal configuration

The simplest setup with manual TLS certificates:

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

Automatic TLS certificate management. Requires port 80 to be accessible for ACME HTTP-01 challenges:

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

tunnels:
  max_per_user: 10
```

Set `environment` to `"staging"` during testing to avoid Let's Encrypt rate limits.

### Full production configuration

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

oauth2:
  enabled: true
  redirect_url: "https://tunnel.example.com/auth/callback"
  session_key: "generate-a-random-secret-here"
  session_store: "mongo"
  token_duration: "5m"
  cookie_duration: "24h"
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
  timeout: "10s"

redis:
  enabled: false
  host: "localhost"
  port: 6379
  db: 0
  key_prefix: "nxpose:"
  timeout: "10s"

tunnels:
  max_per_user: 10
  expiration_hours: 24
  inactive_timeout_mins: 60
  tcp_port_min: 10000
  tcp_port_max: 20000

logging:
  level: "info"
  format: "text"

admin:
  enabled: true
  path_prefix: "/admin"
  auth_method: "basic"
  username: "admin"
  password: "change-me"

access_control:
  require_auth: true
  allow_registration: true
  allowed_sources:
    - "0.0.0.0/0"
```

## SSL / Let's Encrypt

When `letsencrypt.enabled` is `true`, NXpose automatically obtains and renews TLS certificates from Let's Encrypt for the server domain and all tunnel subdomains. The ACME HTTP-01 challenge requires port 80 to be publicly accessible.

```yaml
letsencrypt:
  enabled: true
  email: "admin@example.com"
  environment: "production"    # "staging" for testing
  storage_dir: "/etc/nxpose/certificates"
```

If you prefer to manage certificates manually, disable Let's Encrypt and provide certificate paths:

```yaml
tls:
  cert: "/etc/nxpose/certs/server.crt"
  key: "/etc/nxpose/certs/server.key"
```

## OAuth2

NXpose authenticates clients via OAuth2. Currently supported providers: GitHub and Google. When a client runs `nxpose register`, the server opens a browser-based OAuth flow and issues client certificates upon successful authentication.

```yaml
oauth2:
  enabled: true
  redirect_url: "https://tunnel.example.com/auth/callback"
  session_key: "your-random-secret-key"
  session_store: "mongo"    # "memory", "mongo", or "redis"
  token_duration: "5m"
  cookie_duration: "24h"
  providers:
    - name: "github"
      client_id: "your-github-client-id"
      client_secret: "your-github-client-secret"
      scopes:
        - "user:email"
        - "read:user"
    # - name: "google"
    #   client_id: "your-google-client-id"
    #   client_secret: "your-google-client-secret"
    #   scopes:
    #     - "https://www.googleapis.com/auth/userinfo.email"
    #     - "https://www.googleapis.com/auth/userinfo.profile"
```

To create a GitHub OAuth app: go to GitHub Settings → Developer settings → OAuth Apps → New OAuth App. Set the callback URL to `https://tunnel.example.com/auth/callback`.

Validate your GitHub credentials with:

```bash
nxpose-server validate-github --config /etc/nxpose/server-config.yaml
```

## MongoDB and Redis

**MongoDB** stores user accounts, client registrations, and sessions. Required for production use:

```yaml
mongodb:
  enabled: true
  uri: "mongodb://localhost:27017"
  database: "nxpose"
  timeout: "10s"
```

**Redis** provides optional session caching and faster session lookups:

```yaml
redis:
  enabled: true
  host: "localhost"
  port: 6379
  db: 0
  key_prefix: "nxpose:"
  timeout: "10s"
```

Session store is selected via `oauth2.session_store`: `"memory"` (default, not persistent), `"mongo"`, or `"redis"`.

## Admin Panel

NXpose includes a built-in admin panel for monitoring tunnels, managing clients, and viewing server status. Accessible via basic HTTP authentication:

```yaml
admin:
  enabled: true
  path_prefix: "/admin"
  auth_method: "basic"
  username: "admin"
  password: "your-secure-password"
```

Access at `https://tunnel.example.com/admin`.

## Tunnel Settings

```yaml
tunnels:
  max_per_user: 10              # max tunnels per user across all clients
  max_connection: ""            # max tunnel duration (e.g., "24h"), empty = no limit
  expiration_hours: 24          # tunnel expiration time in hours
  inactive_timeout_mins: 60     # auto-cleanup after inactivity (minutes)
  tcp_port_min: 10000           # TCP tunnel port range start
  tcp_port_max: 20000           # TCP tunnel port range end
```

## Logging

```yaml
logging:
  level: "info"       # debug, info, warn, error
  format: "text"      # text or json
  # file: "/var/log/nxpose/server.log"   # log to file (default: stdout)
```

## Access Control

```yaml
access_control:
  require_auth: true            # require client authentication
  allow_registration: true      # allow new client registration
  allowed_sources:              # IP ranges in CIDR notation
    - "0.0.0.0/0"              # allow all (default)
    # - "192.168.1.0/24"       # restrict to local network
```

## Systemd Service

If you installed from DEB/RPM packages, the systemd service is already configured. For manual installations, create the service file:

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

```bash
sudo useradd --system --shell /sbin/nologin nxpose
sudo systemctl daemon-reload
sudo systemctl enable nxpose
sudo systemctl start nxpose
sudo systemctl status nxpose
```

## Docker Compose

Full stack with MongoDB and Redis:

```yaml
services:
  nxpose:
    image: ghcr.io/altista-tech/nxpose-server:latest
    ports:
      - "8443:8443"
      - "80:80"
      - "10000-10100:10000-10100"
    volumes:
      - ./server-config.yaml:/etc/nxpose/server-config.yaml
      - nxpose-certs:/etc/nxpose/certificates
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

```bash
docker compose up -d
```

## Firewall

Open the required ports on your server:

```bash
# HTTPS API and WebSocket tunnel endpoint
sudo ufw allow 8443/tcp

# HTTP for Let's Encrypt ACME challenges
sudo ufw allow 80/tcp

# TCP tunnel port range
sudo ufw allow 10000:20000/tcp
```

## Environment Variables

All configuration values can be overridden with `NXPOSE_` prefixed environment variables:

| Variable | Description | Default |
|----------|-------------|---------|
| `NXPOSE_SERVER_BIND_ADDRESS` | Server bind address | `0.0.0.0` |
| `NXPOSE_SERVER_PORT` | Server port | `8443` |
| `NXPOSE_SERVER_DOMAIN` | Base domain for tunnels | `localhost` |
| `NXPOSE_TLS_CERT` | Path to TLS certificate | |
| `NXPOSE_TLS_KEY` | Path to TLS key | |
| `NXPOSE_MONGODB_URI` | MongoDB connection URI | `mongodb://localhost:27017` |
| `NXPOSE_MONGODB_DATABASE` | MongoDB database name | `nxpose` |
| `NXPOSE_REDIS_HOST` | Redis host | `localhost` |
| `NXPOSE_REDIS_PORT` | Redis port | `6379` |
| `NXPOSE_ADMIN_ENABLED` | Enable admin panel | `false` |
| `NXPOSE_ADMIN_USERNAME` | Admin panel username | `admin` |
| `NXPOSE_ADMIN_PASSWORD` | Admin panel password | |

## Verify

```bash
# Check server is running
curl -k https://localhost:8443/api/status

# Register a client
nxpose register --server tunnel.example.com --port 8443

# Create a tunnel
nxpose expose http 3000
```

## All Server Options

```
nxpose-server runs the public-facing server component of the nxpose
secure tunneling service.

Usage:
  nxpose-server [flags]
  nxpose-server [command]

Available Commands:
  check-yaml      Check YAML config file parsing
  completion      Generate the autocompletion script for the specified shell
  fix-yaml        Fix YAML config file structure
  help            Help about any command
  validate-github Validate GitHub OAuth credentials in config

Flags:
  -b, --bind string       Address to bind the server to (default "0.0.0.0")
      --config string     config file (default is $HOME/.nxpose/server-config.yaml)
      --domain string     Base domain for tunnels (default "localhost")
  -h, --help              help for nxpose-server
  -p, --port int          Port to listen on (default 8443)
      --tls-cert string   Path to TLS certificate file
      --tls-key string    Path to TLS key file
  -v, --verbose           Enable verbose output
```
