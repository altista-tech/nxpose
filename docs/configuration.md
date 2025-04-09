# Configuring NXpose

This guide explains how to configure both the NXpose client and server components to meet your specific requirements.

## Client Configuration

The NXpose client stores its configuration in the user's home directory:

- Linux/macOS: `~/.nxpose/config.yaml`
- Windows: `%USERPROFILE%\.nxpose\config.yaml`

### Basic Configuration

A basic client configuration file looks like this:

```yaml
# Server to connect to
server: "nxpose.example.com:8443"

# Default subdomain to use (optional)
default_subdomain: "myapp"

# Default protocol for tunnels (http, https, tcp)
default_protocol: "http"

# TLS verification settings
tls:
  # Verify server certificate (recommended for production)
  verify: true
  
  # Path to custom CA certificate (if using self-signed certificates)
  ca_cert: ""

# Auth token (generated when you register)
auth_token: ""

# Logging settings
logging:
  # Log level (debug, info, warn, error)
  level: "info"
```

### Environment Variables

You can also configure the client using environment variables, which take precedence over the configuration file:

| Environment Variable | Description | Default |
| --- | --- | --- |
| NXPOSE_SERVER | Server to connect to | nxpose.example.com:8443 |
| NXPOSE_SUBDOMAIN | Default subdomain to use | (random) |
| NXPOSE_PROTOCOL | Default protocol for tunnels | http |
| NXPOSE_TLS_VERIFY | Verify server certificate | true |
| NXPOSE_TLS_CA_CERT | Path to custom CA certificate | "" |
| NXPOSE_AUTH_TOKEN | Auth token | "" |
| NXPOSE_LOG_LEVEL | Log level | info |

### Client Registration

Before using NXpose, you need to register with a server to obtain an authentication token:

```bash
nxpose register --server nxpose.example.com:8443
```

This will:
1. Generate a client certificate signing request
2. Send it to the server
3. Receive a signed certificate
4. Store the certificate and token in your configuration directory

## Server Configuration

The NXpose server uses a YAML configuration file located at one of the following paths (in order of precedence):

1. Path specified with the `--config` flag
2. `/etc/nxpose/server-config.yaml` (Linux/macOS)
3. `C:\ProgramData\nxpose\server-config.yaml` (Windows)
4. `./server-config.yaml` (current directory)

### Server Settings

The server configuration file is divided into several sections:

#### Basic Server Settings

```yaml
# Server settings
server:
  # Address to bind the server to
  bind_address: "0.0.0.0"

  # Port for the HTTPS API server
  port: 8443

  # Base domain for tunnels
  domain: "nxpose.example.com"
```

#### TLS Settings

```yaml
# TLS settings
tls:
  # Path to TLS certificate file
  cert: "/etc/nxpose/certs/server.crt"

  # Path to TLS key file
  key: "/etc/nxpose/certs/server.key"

  # Minimum TLS version (optional)
  min_version: "1.2"
```

#### OAuth2 Configuration

```yaml
# OAuth2 configuration for user authentication
oauth2:
  # Enable OAuth2 authentication
  enabled: true

  # Redirect URL for OAuth callbacks
  redirect_url: "https://nxpose.example.com/auth/callback"

  # Session key for cookie encryption
  session_key: "your-random-secret-key"

  # Session store options (memory, mongo, redis)
  session_store: "memory"

  token_duration: "5m"    # JWT token duration
  cookie_duration: "24h"  # Cookie duration

  # OAuth providers
  providers:
    # GitHub OAuth configuration
    - name: "github"
      client_id: "github-client-id"
      client_secret: "github-client-secret"
      scopes:
        - "user:email"
        - "read:user"
```

#### MongoDB Configuration

```yaml
# MongoDB configuration for user storage
mongodb:
  # Enable MongoDB storage
  enabled: true

  # MongoDB connection URI
  uri: "mongodb://localhost:27017"

  # Database name
  database: "nxpose"

  # Connection timeout
  timeout: "10s"
```

#### Redis Configuration

```yaml
# Redis configuration for session storage and caching
redis:
  # Enable Redis storage
  enabled: false

  # Redis server host
  host: "localhost"

  # Redis server port
  port: 6379

  # Redis password (optional)
  password: ""

  # Redis database number
  db: 0

  # Key prefix for Redis keys
  key_prefix: "nxpose:"

  # Connection timeout
  timeout: "10s"
```

#### Let's Encrypt Settings

```yaml
# Let's Encrypt settings for automatic TLS certificates
letsencrypt:
  # Enable Let's Encrypt integration
  enabled: false

  # Email address for Let's Encrypt registration
  email: "admin@example.com"
  
  # Environment to use: "production" or "staging"
  environment: "production"

  # Directory to store Let's Encrypt certificates
  storage_dir: "/etc/nxpose/certificates"
```

#### Tunnel Settings

```yaml
# Tunnel settings
tunnels:
  # Maximum number of tunnels per client
  max_per_client: 5

  # Maximum number of tunnels per user
  max_per_user: 10

  # Maximum connection time for tunnels
  max_connection: "24h"

  # Tunnel expiration time in hours
  expiration_hours: 24

  # Automatically clean up inactive tunnels after this many minutes
  inactive_timeout_mins: 60

  # TCP port range for TCP tunnels
  tcp_port_min: 10000
  tcp_port_max: 20000
```

#### Logging Settings

```yaml
# Logging settings
logging:
  # Log level (debug, info, warn, error)
  level: "info"

  # Log format (text, json)
  format: "text"

  # Log file (if not specified, logs to stdout)
  file: "/var/log/nxpose/server.log"
```

#### Access Control Settings

```yaml
# Access control settings
access_control:
  # Enable authentication for clients
  require_auth: true

  # Allow registration of new clients
  allow_registration: true

  # Allowed client sources (IP ranges in CIDR notation)
  allowed_sources:
    - "0.0.0.0/0"  # Allow all IPs
```

### Environment Variables

The server can also be configured using environment variables, which take precedence over the configuration file:

| Environment Variable | Description | Default |
| --- | --- | --- |
| NXPOSE_SERVER_BIND_ADDRESS | Address to bind the server to | 0.0.0.0 |
| NXPOSE_SERVER_PORT | Port to listen on | 8443 |
| NXPOSE_SERVER_DOMAIN | Base domain for tunnels | localhost |
| NXPOSE_TLS_CERT | Path to TLS certificate file | "" |
| NXPOSE_TLS_KEY | Path to TLS key file | "" |
| NXPOSE_OAUTH2_ENABLED | Enable OAuth2 authentication | false |
| NXPOSE_OAUTH2_REDIRECT_URL | Redirect URL for OAuth callbacks | "" |
| NXPOSE_MONGODB_ENABLED | Enable MongoDB | false |
| NXPOSE_MONGODB_URI | MongoDB connection URI | mongodb://localhost:27017 |
| NXPOSE_REDIS_ENABLED | Enable Redis | false |
| NXPOSE_REDIS_HOST | Redis server host | localhost |
| NXPOSE_LETSENCRYPT_ENABLED | Enable Let's Encrypt | false |
| NXPOSE_LETSENCRYPT_EMAIL | Email for Let's Encrypt | "" |
| NXPOSE_LOG_LEVEL | Log level | info |

## Example Configurations

### Minimal Client Configuration

```yaml
server: "nxpose.example.com:8443"
auth_token: "your-auth-token"
```

### Minimal Server Configuration

```yaml
server:
  bind_address: "0.0.0.0"
  port: 8443
  domain: "nxpose.example.com"

tls:
  cert: "/etc/nxpose/certs/server.crt"
  key: "/etc/nxpose/certs/server.key"
```

### Production Server Configuration

For a production environment, consider using this more comprehensive configuration:

```yaml
server:
  bind_address: "0.0.0.0"
  port: 8443
  domain: "nxpose.example.com"

tls:
  cert: "/etc/nxpose/certs/server.crt"
  key: "/etc/nxpose/certs/server.key"
  min_version: "1.2"

oauth2:
  enabled: true
  redirect_url: "https://nxpose.example.com/auth/callback"
  session_key: "your-random-secret-key"
  session_store: "redis"
  providers:
    - name: "github"
      client_id: "github-client-id"
      client_secret: "github-client-secret"
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

letsencrypt:
  enabled: true
  email: "admin@example.com"
  environment: "production"

tunnels:
  max_per_client: 5
  max_per_user: 10
  inactive_timeout_mins: 60

logging:
  level: "info"
  format: "json"
  file: "/var/log/nxpose/server.log"

access_control:
  require_auth: true
  allow_registration: true
  allowed_sources:
    - "0.0.0.0/0"
``` 