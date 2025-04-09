# NXpose Architecture

This document describes the architecture of NXpose, explaining how the different components interact and how data flows through the system.

## System Components

NXpose consists of several core components that work together to create secure tunnels:

### 1. Client Components

- **Client CLI**: Command-line interface for user interaction
- **Tunnel Client**: Manages the local end of the tunnel connections
- **Certificate Manager**: Handles client TLS certificates
- **Local Service Connector**: Connects to local services that need to be exposed

### 2. Server Components

- **API Server**: Handles client registration and tunnel management requests
- **Tunnel Manager**: Manages active tunnels and their lifecycle
- **Authentication Server**: Handles user authentication via OAuth2
- **Certificate Authority**: Issues client certificates
- **Storage Adapters**: Connect to MongoDB and Redis for persistent storage

## Data Flow Diagram

```
┌─────────────────┐                  ┌───────────────────────────────────┐
│                 │                  │                                   │
│  Local Service  │                  │            Internet               │
│                 │                  │                                   │
└────────┬────────┘                  └───────────────┬───────────────────┘
         │                                           │
         │                                           │
         ▼                                           ▼
┌─────────────────┐    Encrypted     ┌───────────────────────────────────┐
│                 │    Tunnel        │                                   │
│  NXpose Client  ├────Connection────►       NXpose Server               │
│                 │                  │                                   │
└─────────────────┘                  └───────────────┬───────────────────┘
                                                     │
                                                     │
                                     ┌───────────────┴───────────────────┐
                                     │                                   │
                                     │        Storage (Optional)         │
                                     │     (MongoDB, Redis, Files)       │
                                     │                                   │
                                     └───────────────────────────────────┘
```

## Communication Protocols

NXpose utilizes several protocols for different aspects of communication:

1. **HTTPS/REST**: Used for the API server and client registration
2. **WebSockets**: Used for maintaining persistent tunnel connections
3. **TCP**: Used for raw TCP tunneling
4. **TLS**: Used to encrypt all communication between client and server

## Request Flow

Here's how a typical request flows through the NXpose system:

1. An external client makes a request to a subdomain on the NXpose server (e.g., `myapp.nxpose.example.com`)
2. The NXpose server identifies the tunnel based on the subdomain
3. The server sends the request data through the encrypted WebSocket connection to the NXpose client
4. The NXpose client forwards the request to the locally running service
5. The local service processes the request and sends the response back to the NXpose client
6. The NXpose client sends the response back through the WebSocket connection to the server
7. The server relays the response back to the original requester

## Security Architecture

Security is a core aspect of NXpose. Here's how different security measures are implemented:

### Authentication and Authorization

- **Client Authentication**: Clients authenticate using client certificates issued by the server's certificate authority
- **User Authentication**: Users can authenticate via OAuth2 providers (GitHub, Google, etc.)
- **Authorization**: Access controls determine which users can create tunnels and access resources

### Encryption

- **Transport Layer Security**: All communication between client and server is encrypted using TLS
- **Certificate Validation**: Certificates are validated to prevent man-in-the-middle attacks
- **Automatic Certificate Management**: Let's Encrypt integration for automatic TLS certificate generation and renewal

### Data Protection

- **Secure Storage**: User data and credentials can be stored in MongoDB with encryption options
- **Session Management**: Sessions can be managed in Redis with proper encryption and TTL settings
- **Minimal Data Collection**: Only essential data is collected and stored

## Scalability Considerations

The NXpose architecture is designed with scalability in mind:

- **Stateless API Server**: Can be scaled horizontally behind a load balancer
- **Independent Tunnel Connections**: Each tunnel operates independently
- **External Storage**: MongoDB and Redis can be scaled separately based on needs
- **Tunnel Limits**: Configurable limits on tunnels per user and client
- **Connection Timeouts**: Automatic cleanup of inactive tunnels 