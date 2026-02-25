# Contributing to NXpose

Thank you for your interest in contributing to NXpose! This guide will help you get started.

## Development Setup

### Prerequisites

- Go 1.24 or later
- Make (optional, for using Makefile)
- Docker and Docker Compose (for integration tests)

### Getting Started

```bash
# Fork and clone the repository
git clone https://github.com/<your-username>/nxpose.git
cd nxpose

# Install dependencies
go mod download

# Build both client and server
make build

# Run tests
go test ./...

# Run integration tests (requires Docker)
make test-integration

# Run linting
go vet ./...
staticcheck ./...
```

### Using the Dev Container

The project includes a VS Code dev container with all dependencies pre-configured:

1. Open the project in VS Code
2. Click "Reopen in Container" when prompted
3. The container includes Go 1.24, Docker-in-Docker, MongoDB, and Redis

### Building

```bash
# Build client
cd cmd/client && go build -o nxpose

# Build server
cd cmd/server && go build -o nxpose-server

# Or use Make
make build
```

## How to Contribute

### Reporting Bugs

1. Check [existing issues](https://github.com/altista-tech/nxpose/issues) to avoid duplicates.
2. Use the [bug report template](https://github.com/altista-tech/nxpose/issues/new?template=bug_report.md).
3. Include:
   - Clear description of the problem
   - Steps to reproduce
   - Expected vs actual behavior
   - OS, Go version, and NXpose version
   - Relevant logs

### Suggesting Features

1. Use the [feature request template](https://github.com/altista-tech/nxpose/issues/new?template=feature_request.md).
2. Describe the feature, its use case, and why it would be useful.

### Pull Request Workflow

1. Fork the repository and create a feature branch:
   ```bash
   git checkout -b feature/your-feature-name
   ```
2. Make your changes, following the code style guide below.
3. Add tests for new functionality.
4. Ensure all tests and linting pass:
   ```bash
   go test ./...
   go vet ./...
   staticcheck ./...
   ```
5. Format your code:
   ```bash
   go fmt ./...
   ```
6. Commit with clear, descriptive messages.
7. Push to your fork and open a Pull Request against `main`.

## Code Style Guide

- Follow [Effective Go](https://golang.org/doc/effective_go.html) guidelines.
- Use `gofmt` for formatting.
- **Imports**: Standard library first, third-party second, local packages last.
- **Naming**: `CamelCase` for exported identifiers, `camelCase` for unexported.
- **Error handling**: Always check errors; use `context` for cancellation.
- **Logging**: Use `logrus` for structured logging.
- **Testing**: Use `testify` for assertions.
- Document all exported functions and types.

## Testing

- Write tests for all new functionality.
- Use `testify` for assertions:
  ```go
  import "github.com/stretchr/testify/assert"

  func TestExample(t *testing.T) {
      result := DoSomething()
      assert.Equal(t, expected, result)
  }
  ```
- Run the full test suite before submitting a PR:
  ```bash
  go test ./...
  ```
- Run integration tests (requires Docker):
  ```bash
  make test-integration
  ```
- Run all tests (unit + integration):
  ```bash
  make test-all
  ```

## Code of Conduct

Please be respectful and constructive in all interactions. We are committed to providing a welcoming and inclusive experience for everyone.

## License

By contributing to NXpose, you agree that your contributions will be licensed under the [MIT License](LICENSE).
