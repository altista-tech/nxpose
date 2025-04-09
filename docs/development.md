# Development Guide

This guide provides information for developers who want to contribute to NXpose or build it from source with customizations.

## Project Structure

The NXpose codebase is organized as follows:

```
nxpose/
├── cmd/                     # Command-line applications
│   ├── client/              # Client executable
│   └── server/              # Server executable
├── internal/                # Private application code
│   ├── config/              # Configuration handling
│   ├── crypto/              # Cryptography utilities
│   ├── logger/              # Logging utilities
│   ├── protocol/            # Protocol implementations
│   ├── server/              # Server implementation
│   ├── template/            # Template utilities
│   └── tunnel/              # Tunnel implementation
├── templates/               # HTML/template files
├── yaml_utils/              # YAML utilities
├── infra/                   # Infrastructure as code
├── vendor/                  # Vendored dependencies
├── .github/                 # GitHub workflows and templates
├── go.mod                   # Go module definition
├── go.sum                   # Go module checksums
├── Makefile                 # Build automation
└── server-config.example.yaml # Example server configuration
```

## Setting Up the Development Environment

### Prerequisites

- Go 1.20 or later
- Git
- Make (for Linux/macOS) or PowerShell (for Windows)

### Clone the Repository

```bash
git clone https://github.com/yourusername/nxpose.git
cd nxpose
```

### Install Dependencies

```bash
make deps
```

## Building from Source

### Building the Client

```bash
make client
```

The client binary will be in `./bin/nxpose` (or `./bin/nxpose.exe` on Windows).

### Building the Server

```bash
make server
```

The server binary will be in `./bin/nxpose-server` (or `./bin/nxpose-server.exe` on Windows).

### Building Both

```bash
make build
```

### Building for Multiple Platforms

```bash
make build-all
```

This will build binaries for Linux, macOS, and Windows (both AMD64 and ARM64 architectures) in the `./dist` directory.

## Running Tests

### Running All Tests

```bash
make test
```

### Running Specific Tests

```bash
go test ./internal/tunnel/...
```

## Creating Packages

NXpose can be packaged as RPM and APK packages for Linux distributions:

```bash
# Create all packages
make packages

# Create only RPM packages
make rpm

# Create only APK packages
make apk
```

## Development Workflow

1. **Create a feature branch**: Always create a new branch for your changes
   ```bash
   git checkout -b feature/my-new-feature
   ```

2. **Make your changes**: Implement your feature or fix

3. **Test your changes**: Add tests for your changes and run the test suite
   ```bash
   make test
   ```

4. **Build and verify**: Build the binaries and verify that they work
   ```bash
   make build
   ./bin/nxpose version
   ```

5. **Commit your changes**: Use descriptive commit messages
   ```bash
   git commit -m "Add feature: my new feature"
   ```

6. **Push your branch**: Push your branch to your fork
   ```bash
   git push origin feature/my-new-feature
   ```

7. **Create a pull request**: Open a PR against the main repository

## Code Style and Guidelines

### Go Code Style

- Follow the [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- Use `gofmt` to format your code
- Add comments to exported functions, types, and constants
- Use meaningful variable names

### Error Handling

- Always check errors and return them to the caller
- Use package `errors` for wrapping errors
- Log errors at the appropriate level

### Testing

- Write unit tests for all new functionality
- Aim for high test coverage
- Use table-driven tests where appropriate
- Mock external dependencies for tests

## Makefile Commands

The Makefile includes various commands to help with development:

| Command | Description |
| --- | --- |
| `make build` | Build both client and server for the current platform |
| `make client` | Build only the client |
| `make server` | Build only the server |
| `make test` | Run all tests |
| `make deps` | Install dependencies |
| `make clean` | Clean build artifacts |
| `make build-all` | Build for all platforms |
| `make packages` | Create RPM and APK packages |

## Debugging

### Client Debugging

Enable verbose output for the client:

```bash
nxpose expose http 3000 --verbose
```

### Server Debugging

Enable verbose output for the server:

```bash
nxpose-server --verbose
```

Set the log level to debug in the configuration:

```yaml
logging:
  level: "debug"
```

## Contributing Guidelines

We welcome contributions to NXpose! Here's how to contribute:

1. **Find an issue**: Look for open issues or create a new one
2. **Discuss**: Discuss your approach on the issue before starting work
3. **Fork and clone**: Fork the repository and clone it locally
4. **Create a branch**: Create a branch for your changes
5. **Make your changes**: Implement your feature or fix
6. **Test your changes**: Add tests and ensure all tests pass
7. **Submit a pull request**: Submit a PR with a clear description
8. **Code review**: Address any feedback from the code review
9. **Merge**: Once approved, your PR will be merged

### Pull Request Checklist

Before submitting a pull request, make sure you have:

- [ ] Written tests for your changes
- [ ] Updated documentation if necessary
- [ ] Run `go fmt` on your code
- [ ] Verified that all tests pass
- [ ] Checked that the build works
- [ ] Added a description of your changes to the PR 