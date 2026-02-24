# NXpose Open-Source Readiness Plan

## Overview
Prepare the nxpose tunneling service for open-source release by adding missing community files, enhancing the documentation website with a product landing page, building comprehensive unit tests, creating integration test infrastructure using dev containers, expanding CI/CD for cross-platform package builds (DEB, RPM, macOS PKG, client binaries), and adding a self-hosting admin panel using HTMX and shadcn/ui.

## Context
- Files involved: All Go packages in internal/, cmd/, site/, infra/, .github/workflows/, .devcontainer/, internal/admin/ (new)
- Related patterns: testify for assertions, gorilla/mux routing, MkDocs Material theming, Docker multi-stage builds
- Dependencies: testify (existing), testcontainers-go (new), Docker/dev containers, HTMX, shadcn/ui (via CDN or embedded assets)

## Development Approach
- **Testing approach**: TDD where practical, code-first for infrastructure/config tasks
- Complete each task fully before moving to the next
- **CRITICAL: every task MUST include new/updated tests**
- **CRITICAL: all tests must pass before starting next task**

## Implementation Steps

### Task 1: Add Missing Open-Source Community Files

**Files:**
- Create: `README.md` 
- Create: `.github/ISSUE_TEMPLATE/bug_report.md`
- Create: `.github/ISSUE_TEMPLATE/feature_request.md`
- Create: `.github/PULL_REQUEST_TEMPLATE.md`

- [x] Add GitHub issue and PR templates
- [x] Update README.md to fix broken contributing/license links and add CI badge
- [x] Verify all cross-references are valid

### Task 2: Unit Tests for internal/config Package

**Files:**
- Create: `internal/config/config_test.go`

- [ ] Test YAML config file loading (server and client configs)
- [ ] Test environment variable overrides with NXPOSE_ prefix
- [ ] Test default value population
- [ ] Test invalid/missing config file handling
- [ ] Test config validation (required fields, port ranges, URLs)
- [ ] Run project test suite - must pass before task 3

### Task 3: Unit Tests for internal/crypto Package

**Files:**
- Create: `internal/crypto/encryption_test.go`
- Create: `internal/crypto/server_tls_test.go`
- Create: `internal/crypto/oauth_test.go`
- Create: `internal/crypto/certificate_manager_test.go`
- Create: `internal/crypto/dns_test.go`

- [ ] Test self-signed certificate generation (valid cert, key pair, expiry)
- [ ] Test TLS configuration creation (min version, cipher suites)
- [ ] Test certificate file loading (valid, missing, invalid files)
- [ ] Test OAuth2 config construction for GitHub and Google providers
- [ ] Test OAuth2 state parameter generation and validation
- [ ] Test DNS provider configuration parsing
- [ ] Run project test suite - must pass before task 4

### Task 4: Unit Tests for internal/protocol Package

**Files:**
- Create: `internal/protocol/protocol_test.go`

- [ ] Test HTTP request/response message serialization and deserialization
- [ ] Test protocol message type handling (tunnel create, data, close)
- [ ] Test metrics collection (request count, byte tracking)
- [ ] Test error message formatting
- [ ] Run project test suite - must pass before task 5

### Task 5: Unit Tests for internal/tunnel Package

**Files:**
- Create: `internal/tunnel/tunnel_test.go`
- Create: `internal/tunnel/tcp_tunnel_test.go`

- [ ] Test tunnel manager creation and configuration
- [ ] Test tunnel registration and lookup
- [ ] Test tunnel expiration and cleanup logic
- [ ] Test TCP tunnel connection handling
- [ ] Test reconnection logic and backoff
- [ ] Test concurrent tunnel operations (race conditions)
- [ ] Run project test suite - must pass before task 6

### Task 6: Expand Server Unit Tests

**Files:**
- Modify: `internal/server/server_test.go`
- Create: `internal/server/handler_test.go`
- Create: `internal/server/websocket_test.go`

- [ ] Add tests for all HTTP handler endpoints (health, status, tunnel CRUD)
- [ ] Test subdomain routing and wildcard matching
- [ ] Test tunnel limit enforcement (per-user and per-client)
- [ ] Test session management (create, validate, expire)
- [ ] Test WebSocket upgrade and tunnel data flow with mock connections
- [ ] Test OAuth2 callback handling with mock provider responses
- [ ] Test MongoDB and Redis store interfaces with mock implementations
- [ ] Run project test suite - must pass before task 7

### Task 7: Dev Container and Integration Test Infrastructure

**Files:**
- Create: `.devcontainer/devcontainer.json`
- Create: `.devcontainer/docker-compose.yml`
- Create: `.devcontainer/Dockerfile`
- Create: `internal/integration/integration_test.go`
- Create: `internal/integration/helpers_test.go`
- Modify: `Makefile` (add integration test targets)

- [ ] Create devcontainer config with Go toolchain, Docker-in-Docker, MongoDB, Redis
- [ ] Create docker-compose for dev environment (server, MongoDB, Redis, test runner)
- [ ] Create test helper functions for container lifecycle management
- [ ] Write integration test: client registers, creates tunnel, sends HTTP through tunnel
- [ ] Write integration test: TCP tunnel creation and data forwarding
- [ ] Write integration test: tunnel expiration and cleanup under load
- [ ] Write integration test: multiple concurrent clients with tunnel isolation
- [ ] Add `make test-integration` and `make test-all` Makefile targets
- [ ] Add build tag `//go:build integration` to separate integration from unit tests
- [ ] Run full test suite (unit + integration) - must pass before task 8

### Task 8: CI/CD Pipeline for Tests

**Files:**
- Create: `.github/workflows/test.yml`

- [ ] Create test workflow: runs unit tests on push/PR with go test ./...
- [ ] Add test coverage reporting with go tool cover
- [ ] Add go vet and staticcheck linting steps
- [ ] Add integration test job using Docker Compose services (MongoDB, Redis)
- [ ] Add coverage badge generation
- [ ] Run all workflows locally with act or verify YAML syntax
- [ ] Run project test suite - must pass before task 9

### Task 9: CI/CD for Cross-Platform Package Builds

**Files:**
- Modify: `Makefile` (add RPM support, client binary targets)
- Modify: `.github/workflows/build-packages.yml`

- [ ] Add RPM package format support in Makefile (spec file generation, rpmbuild)
- [ ] Add client binary build target in Makefile (cross-compile for linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64)
- [ ] Expand CI build matrix to include RPM format alongside DEB
- [ ] Add macOS PKG build job in CI (using macos-latest runner)
- [ ] Add client binary build job that produces standalone binaries for all platforms
- [ ] Add GitHub Release creation on tag push with all artifacts (DEB, RPM, PKG, client binaries)
- [ ] Gate build-packages workflow on test workflow passing
- [ ] Test that RPM installs server as a systemd service (same as DEB)
- [ ] Run project test suite - must pass before task 10

### Task 10: Admin Panel for Self-Hosting

**Files:**
- Create: `internal/admin/admin.go` (admin HTTP handler, routes)
- Create: `internal/admin/templates/` (HTMX templates - layout, dashboard, tunnels, settings)
- Create: `internal/admin/static/` (CSS with shadcn/ui styling, JS)
- Create: `internal/admin/admin_test.go`
- Modify: `internal/server/server.go` (mount admin routes)
- Modify: `internal/config/config.go` (admin panel config fields)

- [ ] Design admin panel pages: dashboard (active tunnels, connections, bandwidth), tunnel list with controls (kill/inspect), server settings view, client list
- [ ] Create admin Go handler package with HTMX-compatible endpoints returning HTML fragments
- [ ] Build base layout template with shadcn/ui-styled components (sidebar nav, cards, tables, badges)
- [ ] Implement dashboard page: live tunnel count, active connections, bytes transferred, uptime
- [ ] Implement tunnel management page: list tunnels, kill tunnel, view tunnel details
- [ ] Implement client list page: connected clients, their tunnels, last active time
- [ ] Implement server settings page: view current config, toggle maintenance mode
- [ ] Add admin authentication (reuse existing session/OAuth or add basic auth option)
- [ ] Add real-time updates via HTMX polling or SSE for dashboard stats
- [ ] Mount admin routes on configurable path (default /admin) in server.go
- [ ] Add admin panel config fields (enabled, path prefix, auth method) to server config
- [ ] Write unit tests for admin handler endpoints
- [ ] Run project test suite - must pass before task 11

### Task 11: Enhance MkDocs Site with Product Landing Page

**Files:**
- Modify: `site/docs/stylesheets/extra.css`
- Create: `site/overrides/home.html`
- Modify: `site/mkdocs.yml`
- Modify: `site/docs/index.md` (or create custom landing)
- Create: `site/docs/self-hosting.md`
- Create: `site/docs/api-reference.md`
- Create: `site/docs/admin-panel.md`

- [ ] Create custom home page template with hero section, feature cards, and CTA
- [ ] Add product-style CSS (gradient hero, feature grid, responsive layout)
- [ ] Add self-hosting guide (Docker Compose deployment, bare metal, configuration)
- [ ] Add API reference documentation for server endpoints
- [ ] Add admin panel documentation (setup, features, screenshots)
- [ ] Add "Getting Started" prominent quick-start section on landing page
- [ ] Configure MkDocs navigation to use custom home + existing docs structure
- [ ] Add social cards and OpenGraph metadata for link previews
- [ ] Test site builds correctly with `make site` and docker-compose
- [ ] Verify responsive design on mobile/tablet/desktop viewports
- [ ] Run project test suite - must pass before task 12

### Task 12: Verify Acceptance Criteria

- [ ] All community files present and properly linked (LICENSE, CONTRIBUTING, CODE_OF_CONDUCT, SECURITY)
- [ ] Unit tests exist for all internal packages (config, crypto, protocol, tunnel, server, admin)
- [ ] Integration tests run in containers with real MongoDB and Redis
- [ ] Dev container config allows one-click development setup
- [ ] CI pipeline runs unit tests, integration tests, and linting on every PR
- [ ] CI builds DEB, RPM, and macOS PKG packages for server (amd64 + arm64)
- [ ] CI builds client binaries for Linux, macOS, and Windows (amd64 + arm64)
- [ ] GitHub Releases created with all artifacts on tag push
- [ ] Admin panel accessible at /admin with dashboard, tunnel management, and client list
- [ ] Documentation site has a polished product landing page
- [ ] Self-hosting and admin panel documentation is comprehensive
- [ ] Run full test suite: `go test ./...`
- [ ] Run linter: `go vet ./...`
- [ ] Verify test coverage meets 80%+
- [ ] Manual test: open documentation site and verify landing page renders correctly
- [ ] Manual test: open project in VS Code with dev container and verify it works
- [ ] Manual test: access admin panel and verify dashboard shows live tunnel stats

### Task 13: Update Documentation

- [ ] Update README.md with test instructions, dev container usage, and admin panel info
- [ ] Update CLAUDE.md with integration test commands and new file patterns
- [ ] Move this plan to `docs/plans/completed/`
