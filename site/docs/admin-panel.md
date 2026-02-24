# Admin Panel

NXpose includes a built-in admin panel for monitoring and managing your tunneling server. The panel is built with HTMX for real-time updates and uses a clean, modern interface.

## Enabling the Admin Panel

Add the following to your `server-config.yaml`:

```yaml
admin:
  enabled: true
  path_prefix: "/admin"
  auth_method: "basic"
  username: "admin"
  password: "your-secure-password"
```

Or use environment variables:

```bash
export NXPOSE_ADMIN_ENABLED=true
export NXPOSE_ADMIN_PATH_PREFIX=/admin
export NXPOSE_ADMIN_AUTH_METHOD=basic
export NXPOSE_ADMIN_USERNAME=admin
export NXPOSE_ADMIN_PASSWORD=your-secure-password
```

## Configuration Options

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | `false` | Enable the admin panel |
| `path_prefix` | string | `/admin` | URL path prefix for the admin panel |
| `auth_method` | string | `basic` | Authentication method: `basic` or `none` |
| `username` | string | `admin` | Username for basic authentication |
| `password` | string | (empty) | Password for basic authentication |

!!! warning "Security"
    Always set a strong password when exposing the admin panel. If no password is set, authentication is effectively disabled.

## Pages

### Dashboard

The dashboard provides a real-time overview of your server:

- **Active Tunnels** - Number of currently open tunnels
- **Connected Clients** - Number of clients with active connections
- **Total Connections** - Cumulative connection count since server start
- **Server Uptime** - How long the server has been running

Stats auto-refresh every 5 seconds via HTMX polling.

### Tunnel Management

The tunnels page shows all active tunnels with:

- Tunnel ID and protocol (HTTP/TCP)
- Assigned subdomain
- Target port
- Creation time and last activity
- Connection count
- Active/inactive status

**Actions:**

- **Kill Tunnel** - Immediately terminates a tunnel and disconnects the client

### Client List

View all connected clients with:

- Client ID
- Number of active tunnels per client
- List of associated tunnels
- Last active timestamp

### Server Settings

View the current server configuration and control operational state:

- **Maintenance Mode** - Toggle maintenance mode on/off. When enabled, the server rejects new tunnel creation requests while keeping existing tunnels active.

## Access

Navigate to the configured path in your browser:

```
https://your-server.com:8443/admin
```

If basic authentication is enabled, your browser will prompt for credentials.

## API Access

The admin panel endpoints also support JSON responses for programmatic access. Omit the `HX-Request` header to receive JSON instead of HTML fragments:

```bash
# Get server stats
curl -u admin:password https://your-server.com:8443/admin/api/stats

# List all tunnels
curl -u admin:password https://your-server.com:8443/admin/api/tunnels

# Kill a tunnel
curl -X POST -u admin:password https://your-server.com:8443/admin/api/tunnels/tun-abc123/kill

# List connected clients
curl -u admin:password https://your-server.com:8443/admin/api/clients

# Toggle maintenance mode
curl -X POST -u admin:password https://your-server.com:8443/admin/api/settings/maintenance
```

See the [API Reference](api-reference.md) for full endpoint documentation.
