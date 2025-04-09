# API Reference

The NXpose server provides a REST API that allows for programmatic interaction with the service. This document describes the available endpoints, authentication methods, and expected responses.

## Authentication

API requests must be authenticated using one of the following methods:

### Client Certificate Authentication

Most API endpoints require a valid client certificate that was issued by the NXpose server's certificate authority. This is the primary authentication method used by the client.

### Bearer Token Authentication

Some endpoints support Bearer token authentication, which is useful for integration with other services. Tokens are obtained through the OAuth2 flow.

Example:
```
Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...
```

## API Endpoints

### Client Registration API

#### Register a New Client

```
POST /api/v1/register
```

Registers a new client with the server and issues a client certificate.

**Request Body:**
```json
{
  "csr": "-----BEGIN CERTIFICATE REQUEST-----\n...\n-----END CERTIFICATE REQUEST-----",
  "name": "my-client",
  "subdomain": "my-app"
}
```

**Response:**
```json
{
  "certificate": "-----BEGIN CERTIFICATE-----\n...\n-----END CERTIFICATE-----",
  "token": "client-auth-token",
  "expires_at": "2023-12-31T23:59:59Z"
}
```

**Status Codes:**
- `201 Created`: Registration successful
- `400 Bad Request`: Invalid CSR or parameters
- `409 Conflict`: Subdomain already in use
- `429 Too Many Requests`: Rate limit exceeded

### Tunnel Management API

#### Create a Tunnel

```
POST /api/v1/tunnels
```

Creates a new tunnel.

**Request Body:**
```json
{
  "protocol": "http",
  "port": 3000,
  "subdomain": "myapp",
  "path": "/api",
  "options": {
    "auth": "username:password",
    "headers": {
      "X-Forwarded-From": "NXpose"
    },
    "allow_cidrs": ["192.168.1.0/24"]
  }
}
```

**Response:**
```json
{
  "id": "tunnel-id-123456",
  "url": "https://myapp.nxpose.example.com",
  "protocol": "http",
  "port": 3000,
  "status": "active",
  "created_at": "2023-01-01T12:00:00Z",
  "expires_at": "2023-01-02T12:00:00Z"
}
```

**Status Codes:**
- `201 Created`: Tunnel created successfully
- `400 Bad Request`: Invalid parameters
- `409 Conflict`: Subdomain already in use
- `429 Too Many Requests`: Maximum number of tunnels reached

#### List Tunnels

```
GET /api/v1/tunnels
```

Lists all tunnels owned by the authenticated client.

**Response:**
```json
{
  "tunnels": [
    {
      "id": "tunnel-id-123456",
      "url": "https://myapp.nxpose.example.com",
      "protocol": "http",
      "port": 3000,
      "status": "active",
      "created_at": "2023-01-01T12:00:00Z",
      "expires_at": "2023-01-02T12:00:00Z"
    },
    {
      "id": "tunnel-id-789012",
      "url": "https://another.nxpose.example.com",
      "protocol": "https",
      "port": 8443,
      "status": "active",
      "created_at": "2023-01-01T14:00:00Z",
      "expires_at": "2023-01-02T14:00:00Z"
    }
  ]
}
```

**Status Codes:**
- `200 OK`: Success
- `401 Unauthorized`: Invalid authentication

#### Get Tunnel Details

```
GET /api/v1/tunnels/{tunnel_id}
```

Gets details about a specific tunnel.

**Response:**
```json
{
  "id": "tunnel-id-123456",
  "url": "https://myapp.nxpose.example.com",
  "protocol": "http",
  "port": 3000,
  "status": "active",
  "created_at": "2023-01-01T12:00:00Z",
  "expires_at": "2023-01-02T12:00:00Z",
  "statistics": {
    "requests": 42,
    "bytes_in": 12345,
    "bytes_out": 54321
  }
}
```

**Status Codes:**
- `200 OK`: Success
- `401 Unauthorized`: Invalid authentication
- `404 Not Found`: Tunnel not found

#### Delete a Tunnel

```
DELETE /api/v1/tunnels/{tunnel_id}
```

Deletes a tunnel.

**Response:**
```json
{
  "message": "Tunnel deleted successfully"
}
```

**Status Codes:**
- `200 OK`: Success
- `401 Unauthorized`: Invalid authentication
- `404 Not Found`: Tunnel not found

### Admin API

These endpoints are only accessible to users with administrative privileges.

#### List All Tunnels

```
GET /api/v1/admin/tunnels
```

Lists all tunnels on the server.

**Response:**
```json
{
  "tunnels": [
    {
      "id": "tunnel-id-123456",
      "url": "https://myapp.nxpose.example.com",
      "client_id": "client-id-1",
      "user_id": "user-id-1",
      "protocol": "http",
      "port": 3000,
      "status": "active",
      "created_at": "2023-01-01T12:00:00Z",
      "expires_at": "2023-01-02T12:00:00Z"
    },
    // More tunnels...
  ]
}
```

**Status Codes:**
- `200 OK`: Success
- `401 Unauthorized`: Invalid authentication
- `403 Forbidden`: Not an administrator

#### List All Clients

```
GET /api/v1/admin/clients
```

Lists all registered clients.

**Response:**
```json
{
  "clients": [
    {
      "id": "client-id-1",
      "name": "my-client",
      "user_id": "user-id-1",
      "last_seen": "2023-01-01T14:30:00Z",
      "created_at": "2023-01-01T12:00:00Z",
      "active_tunnels": 2
    },
    // More clients...
  ]
}
```

**Status Codes:**
- `200 OK`: Success
- `401 Unauthorized`: Invalid authentication
- `403 Forbidden`: Not an administrator

#### Revoke a Client

```
DELETE /api/v1/admin/clients/{client_id}
```

Revokes a client's access and terminates all its active tunnels.

**Response:**
```json
{
  "message": "Client revoked successfully"
}
```

**Status Codes:**
- `200 OK`: Success
- `401 Unauthorized`: Invalid authentication
- `403 Forbidden`: Not an administrator
- `404 Not Found`: Client not found

### WebSocket API

The WebSocket API is used for maintaining tunnel connections.

#### Tunnel WebSocket

```
WS /api/v1/ws/tunnel/{tunnel_id}
```

Establishes a WebSocket connection for an existing tunnel.

**Query Parameters:**
- `token`: A short-lived tunnel token obtained during tunnel creation

**WebSocket Messages:**

The WebSocket connection uses a binary protocol for efficient data transfer. Each message is prefixed with a header that indicates the message type and payload length.

Message types:
- `0x01`: HTTP Request
- `0x02`: HTTP Response
- `0x03`: TCP Data
- `0x04`: Ping
- `0x05`: Pong
- `0x06`: Error
- `0x07`: Close

## Error Responses

All API errors follow a consistent format:

```json
{
  "error": {
    "code": "invalid_request",
    "message": "The request was invalid",
    "details": {
      "field": "subdomain",
      "reason": "already_exists"
    }
  }
}
```

Common error codes:
- `unauthorized`: Authentication is required
- `forbidden`: Insufficient permissions
- `not_found`: Resource not found
- `invalid_request`: The request was invalid
- `rate_limited`: Rate limit exceeded
- `server_error`: Internal server error

## Pagination

APIs that return collections support pagination through the following query parameters:

- `limit`: Maximum number of items to return (default: 20, max: 100)
- `offset`: Number of items to skip (default: 0)

Example:
```
GET /api/v1/tunnels?limit=10&offset=20
```

Response:
```json
{
  "tunnels": [...],
  "pagination": {
    "total": 42,
    "limit": 10,
    "offset": 20,
    "has_more": true
  }
}
```

## Rate Limiting

The API employs rate limiting to prevent abuse. Limits vary by endpoint but are generally specified in the following headers:

- `X-RateLimit-Limit`: The maximum number of requests allowed in the current time window
- `X-RateLimit-Remaining`: The number of requests remaining in the current time window
- `X-RateLimit-Reset`: The time at which the current rate limit window resets (Unix timestamp)

When rate limited, the API will respond with a `429 Too Many Requests` status code. 