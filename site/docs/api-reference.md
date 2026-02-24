# API Reference

NXpose server exposes a REST API over HTTPS for client registration, tunnel management, and status queries.

## Base URL

All API endpoints are served on the configured server port (default: `8443`).

```
https://your-server.com:8443/api/
```

## Authentication

Most endpoints require a valid client certificate obtained during registration. The certificate is sent as mutual TLS during the connection.

When OAuth2 is enabled, the registration endpoint redirects to the configured OAuth provider.

## Endpoints

### Client Registration

#### `POST /api/register`

Register a new client with the server. Returns a client ID and certificate for future connections.

**Request Body:**

```json
{
  "client_name": "my-laptop",
  "client_region": "us-east"
}
```

**Response (200 OK):**

```json
{
  "success": true,
  "message": "Client registered successfully",
  "client_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
  "certificate": "-----BEGIN CERTIFICATE-----\n...\n-----END CERTIFICATE-----",
  "expires_at": "2025-03-01T00:00:00Z"
}
```

**Error Response (400/401/500):**

```json
{
  "success": false,
  "message": "registration failed: invalid credentials"
}
```

---

### Tunnel Management

#### `POST /api/tunnel`

Create a new tunnel to expose a local service.

**Request Body:**

```json
{
  "client_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
  "protocol": "http",
  "port": 3000,
  "certificate": "-----BEGIN CERTIFICATE-----\n...\n-----END CERTIFICATE-----"
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `client_id` | string | Yes | Client ID from registration |
| `protocol` | string | Yes | `http` or `tcp` |
| `port` | integer | Yes | Local port to forward traffic to |
| `certificate` | string | Yes | Client certificate for authentication |

**Response (200 OK):**

```json
{
  "success": true,
  "message": "Tunnel created successfully",
  "tunnel_id": "tun-abc123",
  "public_url": "https://abc123.your-server.com"
}
```

For TCP tunnels, the `public_url` will include a port number:

```json
{
  "success": true,
  "message": "TCP tunnel created successfully",
  "tunnel_id": "tun-def456",
  "public_url": "tcp://your-server.com:10042"
}
```

---

### WebSocket Tunnel

#### `GET /api/ws` (WebSocket Upgrade)

Establishes a WebSocket connection for tunnel data transport. The client upgrades this connection to send and receive tunneled traffic.

**Query Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `tunnel_id` | string | Yes | The tunnel ID to connect to |
| `client_id` | string | Yes | The client ID |

**WebSocket Messages:**

Messages are exchanged as binary frames using the NXpose protocol format:

| Message Type | Description |
|-------------|-------------|
| `TUNNEL_DATA` | HTTP request/response data or raw TCP bytes |
| `TUNNEL_CLOSE` | Signal to close the tunnel |
| `TUNNEL_PING` | Keep-alive ping |
| `TUNNEL_PONG` | Keep-alive response |

---

### Server Status

#### `GET /api/status`

Returns the current server status. Does not require authentication.

**Response (200 OK):**

```json
{
  "status": "ok",
  "active_tunnels": 12,
  "connected_clients": 8,
  "uptime": "3d 14h 22m",
  "version": "1.0.0"
}
```

---

### Tunnel Request Handling

#### `GET /*` (Catch-all)

All other requests are routed to the appropriate tunnel based on the subdomain. The server extracts the subdomain from the `Host` header and forwards the request through the corresponding tunnel's WebSocket connection.

**Headers:**

| Header | Description |
|--------|-------------|
| `Host` | Used for subdomain matching (e.g., `abc123.your-server.com`) |
| `X-Forwarded-For` | Set by the server to the client's real IP |
| `X-Forwarded-Proto` | Set to the original protocol |

---

## Admin API

When the admin panel is enabled, additional endpoints are available under the configured path prefix (default: `/admin`).

### Dashboard Stats

#### `GET /admin/api/stats`

Returns server statistics.

**Response (JSON):**

```json
{
  "active_tunnels": 5,
  "connected_clients": 3,
  "total_connections": 1247,
  "uptime": 86400000000000,
  "uptime_str": "1d 0h 0m",
  "maintenance_mode": false
}
```

### Tunnel List

#### `GET /admin/api/tunnels`

Returns all active tunnels.

**Response (JSON):**

```json
[
  {
    "id": "tun-abc123",
    "client_id": "client-xyz",
    "protocol": "http",
    "subdomain": "abc123",
    "target_port": 3000,
    "create_time": "2025-01-15T10:30:00Z",
    "last_active": "2025-01-15T12:45:00Z",
    "expires_at": "2025-01-16T10:30:00Z",
    "connections": 42,
    "connected": true
  }
]
```

### Kill Tunnel

#### `POST /admin/api/tunnels/{id}/kill`

Terminates a specific tunnel.

**Response (JSON):**

```json
{
  "status": "killed"
}
```

### Client List

#### `GET /admin/api/clients`

Returns all connected clients.

**Response (JSON):**

```json
[
  {
    "id": "client-xyz",
    "tunnel_count": 2,
    "tunnels": [...],
    "last_active": "2025-01-15T12:45:00Z"
  }
]
```

### Toggle Maintenance Mode

#### `POST /admin/api/settings/maintenance`

Toggles server maintenance mode.

**Response (JSON):**

```json
{
  "maintenance_mode": true
}
```

## Error Codes

| HTTP Status | Description |
|------------|-------------|
| 200 | Success |
| 400 | Bad request (missing or invalid parameters) |
| 401 | Unauthorized (invalid certificate or credentials) |
| 404 | Not found (tunnel or client does not exist) |
| 429 | Too many tunnels (limit exceeded) |
| 500 | Internal server error |
| 503 | Service unavailable (maintenance mode) |
