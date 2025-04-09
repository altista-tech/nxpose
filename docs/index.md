# NXpose Documentation

Welcome to the NXpose documentation. This guide provides comprehensive information about installing, configuring, and using NXpose - a secure tunneling service written in Go that allows you to expose local services to the internet through encrypted tunnels.

## Documentation Sections

- [Introduction](introduction.md) - Learn about NXpose, its features, and use cases
- [Architecture](architecture.md) - Understand the system components and how they interact
- [Installation](installation.md) - Instructions for installing NXpose on different platforms
- [Configuration](configuration.md) - Configure NXpose client and server
- [Usage](usage.md) - Learn how to use NXpose for different scenarios
- [Development](development.md) - Information for developers and contributors
- [API Reference](api-reference.md) - API documentation for advanced integration

## Quick Start

```bash
# Install the client
go install github.com/yourusername/nxpose/cmd/client@latest

# Register with a server
nxpose register

# Expose a local HTTP service running on port 3000
nxpose expose http 3000
```

## Support

If you encounter any issues or have questions about using NXpose, please open an issue on the [GitHub repository](https://github.com/yourusername/nxpose/issues). 