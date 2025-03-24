# NXpose Makefile
# Supports building for Linux, macOS, Windows, and creating APK and RPM packages

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod

# Binary names
SERVER_NAME=nxpose-server
CLIENT_NAME=nxpose

# Source directories
SERVER_SRC=./cmd/server
CLIENT_SRC=./cmd/client

# Output directories
BIN_DIR=./bin
DIST_DIR=./dist
PKG_DIR=./packages

# Version information
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
GIT_COMMIT ?= $(shell git rev-parse HEAD 2>/dev/null || echo "unknown")
BUILD_DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

# LDFLAGS
LDFLAGS=-ldflags "-X main.version=$(VERSION) -X main.gitCommit=$(GIT_COMMIT) -X main.buildDate=$(BUILD_DATE)"

# Supported platforms: GOOS_GOARCH
PLATFORMS=linux_amd64 linux_arm64 darwin_amd64 darwin_arm64 windows_amd64 windows_arm64

# Packaging tools
NFPM_CMD=$(shell command -v nfpm 2> /dev/null)
GORELEASER_CMD=$(shell command -v goreleaser 2> /dev/null)

# Check for Windows
ifeq ($(OS),Windows_NT)
	SHELL := powershell.exe
	.SHELLFLAGS := -NoProfile -Command
	RM_F_CMD = Remove-Item -Force -ErrorAction Ignore
	MKDIR_CMD = New-Item -ItemType Directory -Force
	RMDIR_CMD = Remove-Item -Recurse -Force -ErrorAction Ignore
	EXECUTABLE_EXTENSION = .exe
else
	RM_F_CMD = rm -f
	MKDIR_CMD = mkdir -p
	RMDIR_CMD = rm -rf
	EXECUTABLE_EXTENSION =
endif

.PHONY: all build test clean deps server client build-all packages apk rpm help

all: build

# Install dependencies
deps:
	$(GOMOD) download
	$(GOMOD) tidy
ifeq ($(NFPM_CMD),)
	@echo "nfpm is not installed. Installing..."
	$(GOCMD) install github.com/goreleaser/nfpm/v2/cmd/nfpm@latest
endif

# Build server
server:
	$(MKDIR_CMD) $(BIN_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BIN_DIR)/$(SERVER_NAME)$(EXECUTABLE_EXTENSION) $(SERVER_SRC)

# Build client
client:
	$(MKDIR_CMD) $(BIN_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BIN_DIR)/$(CLIENT_NAME)$(EXECUTABLE_EXTENSION) $(CLIENT_SRC)

# Build both server and client
build: server client

# Run tests
test:
	$(GOTEST) -v ./...

# Build for all platforms
build-all: clean
	@echo "Building for all platforms..."
	$(MKDIR_CMD) $(DIST_DIR)
ifeq ($(OS),Windows_NT)
	@powershell -Command "foreach ($$platform in '$(PLATFORMS)'.Split(' ')) { \
		$$parts = $$platform.Split('_'); \
		$$GOOS = $$parts[0]; \
		$$GOARCH = $$parts[1]; \
		$$DIST_PATH = \"$(DIST_DIR)/$$platform\"; \
		$$SERVER_OUTPUT = \"$$DIST_PATH/$(SERVER_NAME)\"; \
		$$CLIENT_OUTPUT = \"$$DIST_PATH/$(CLIENT_NAME)\"; \
		if ($$GOOS -eq \"windows\") { \
			$$SERVER_OUTPUT += \".exe\"; \
			$$CLIENT_OUTPUT += \".exe\"; \
		}; \
		Write-Host \"Building for $$GOOS/$$GOARCH...\"; \
		New-Item -ItemType Directory -Force -Path $$DIST_PATH | Out-Null; \
		$$env:GOOS = $$GOOS; \
		$$env:GOARCH = $$GOARCH; \
		$(GOBUILD) $(LDFLAGS) -o $$SERVER_OUTPUT $(SERVER_SRC); \
		$(GOBUILD) $(LDFLAGS) -o $$CLIENT_OUTPUT $(CLIENT_SRC); \
	}"
else
	@for platform in $(PLATFORMS); do \
		GOOS=$$(echo $$platform | cut -d_ -f1); \
		GOARCH=$$(echo $$platform | cut -d_ -f2); \
		DIST_PATH="$(DIST_DIR)/$$platform"; \
		SERVER_OUTPUT="$$DIST_PATH/$(SERVER_NAME)"; \
		CLIENT_OUTPUT="$$DIST_PATH/$(CLIENT_NAME)"; \
		if [ "$$GOOS" = "windows" ]; then \
			SERVER_OUTPUT="$$SERVER_OUTPUT.exe"; \
			CLIENT_OUTPUT="$$CLIENT_OUTPUT.exe"; \
		fi; \
		echo "Building for $$GOOS/$$GOARCH..."; \
		$(MKDIR_CMD) $$DIST_PATH; \
		GOOS=$$GOOS GOARCH=$$GOARCH $(GOBUILD) $(LDFLAGS) -o $$SERVER_OUTPUT $(SERVER_SRC); \
		GOOS=$$GOOS GOARCH=$$GOARCH $(GOBUILD) $(LDFLAGS) -o $$CLIENT_OUTPUT $(CLIENT_SRC); \
	done
endif

# Create packages using nfpm
packages: build-all
	@echo "Creating packages..."
	$(MKDIR_CMD) $(PKG_DIR)
ifeq ($(OS),Windows_NT)
	@powershell -Command "\
		$$config = @'\
name: nxpose\
version: ${VERSION}\
description: Secure tunneling service for exposing local services to the internet\
vendor: NXpose\
maintainer: Your Name <your.email@example.com>\
license: MIT\
homepage: https://github.com/yourusername/nxpose\
contents:\
  - src: dist/linux_amd64/nxpose-server\
    dst: /usr/bin/nxpose-server\
  - src: dist/linux_amd64/nxpose\
    dst: /usr/bin/nxpose\
  - src: server-config.example.yaml\
    dst: /etc/nxpose/server-config.example.yaml\
    type: config|noreplace\
overrides:\
  rpm:\
    scripts:\
      postinstall: |\
        mkdir -p /var/lib/nxpose\
  apk:\
    scripts:\
      postinstall: |\
        mkdir -p /var/lib/nxpose\
'@\
		Set-Content -Path nfpm.yaml -Value $$config\
	"
	@powershell -Command "\
		foreach ($$arch in @('amd64', 'arm64')) { \
			if ($$arch -eq 'amd64') { \
				$$rpm_arch = 'x86_64' \
			} else { \
				$$rpm_arch = 'aarch64' \
			} \
			$$env:ARCH = $$arch \
			nfpm package --target $(PKG_DIR)/nxpose_$(VERSION)_$$rpm_arch.rpm --packager rpm --config nfpm.yaml \
		} \
		foreach ($$arch in @('amd64', 'arm64')) { \
			if ($$arch -eq 'amd64') { \
				$$apk_arch = 'x86_64' \
			} else { \
				$$apk_arch = 'aarch64' \
			} \
			$$env:ARCH = $$arch \
			nfpm package --target $(PKG_DIR)/nxpose_$(VERSION)_$$apk_arch.apk --packager apk --config nfpm.yaml \
		}"
else
	# Generate nfpm config
	@cat > nfpm.yaml <<EOF
name: nxpose
version: ${VERSION}
description: Secure tunneling service for exposing local services to the internet
vendor: NXpose
maintainer: Your Name <your.email@example.com>
license: MIT
homepage: https://github.com/yourusername/nxpose
contents:
  - src: dist/linux_amd64/nxpose-server
    dst: /usr/bin/nxpose-server
  - src: dist/linux_amd64/nxpose
    dst: /usr/bin/nxpose
  - src: server-config.example.yaml
    dst: /etc/nxpose/server-config.example.yaml
    type: config|noreplace
overrides:
  rpm:
    scripts:
      postinstall: |
        mkdir -p /var/lib/nxpose
  apk:
    scripts:
      postinstall: |
        mkdir -p /var/lib/nxpose
EOF
	# Create RPM packages
	@for arch in amd64 arm64; do \
		if [ "$$arch" = "amd64" ]; then \
			rpm_arch="x86_64"; \
		else \
			rpm_arch="aarch64"; \
		fi; \
		ARCH=$$arch nfpm package --target $(PKG_DIR)/nxpose_$(VERSION)_$$rpm_arch.rpm --packager rpm --config nfpm.yaml; \
	done
	# Create APK packages
	@for arch in amd64 arm64; do \
		if [ "$$arch" = "amd64" ]; then \
			apk_arch="x86_64"; \
		else \
			apk_arch="aarch64"; \
		fi; \
		ARCH=$$arch nfpm package --target $(PKG_DIR)/nxpose_$(VERSION)_$$apk_arch.apk --packager apk --config nfpm.yaml; \
	done
endif

# Create RPM packages
rpm: build-all
	@echo "Creating RPM packages..."
	$(MKDIR_CMD) $(PKG_DIR)
ifeq ($(OS),Windows_NT)
	@powershell -Command "\
		foreach ($$arch in @('amd64', 'arm64')) { \
			if ($$arch -eq 'amd64') { \
				$$rpm_arch = 'x86_64' \
			} else { \
				$$rpm_arch = 'aarch64' \
			} \
			$$env:ARCH = $$arch \
			nfpm package --target $(PKG_DIR)/nxpose_$(VERSION)_$$rpm_arch.rpm --packager rpm --config nfpm.yaml \
		}"
else
	@for arch in amd64 arm64; do \
		if [ "$$arch" = "amd64" ]; then \
			rpm_arch="x86_64"; \
		else \
			rpm_arch="aarch64"; \
		fi; \
		ARCH=$$arch nfpm package --target $(PKG_DIR)/nxpose_$(VERSION)_$$rpm_arch.rpm --packager rpm --config nfpm.yaml; \
	done
endif

# Create APK packages
apk: build-all
	@echo "Creating APK packages..."
	$(MKDIR_CMD) $(PKG_DIR)
ifeq ($(OS),Windows_NT)
	@powershell -Command "\
		foreach ($$arch in @('amd64', 'arm64')) { \
			if ($$arch -eq 'amd64') { \
				$$apk_arch = 'x86_64' \
			} else { \
				$$apk_arch = 'aarch64' \
			} \
			$$env:ARCH = $$arch \
			nfpm package --target $(PKG_DIR)/nxpose_$(VERSION)_$$apk_arch.apk --packager apk --config nfpm.yaml \
		}"
else
	@for arch in amd64 arm64; do \
		if [ "$$arch" = "amd64" ]; then \
			apk_arch="x86_64"; \
		else \
			apk_arch="aarch64"; \
		fi; \
		ARCH=$$arch nfpm package --target $(PKG_DIR)/nxpose_$(VERSION)_$$apk_arch.apk --packager apk --config nfpm.yaml; \
	done
endif

# Clean
clean:
	$(RMDIR_CMD) $(BIN_DIR)
	$(RMDIR_CMD) $(DIST_DIR)
	$(RMDIR_CMD) $(PKG_DIR)
	$(RM_F_CMD) nfpm.yaml

# Help
help:
	@echo "Available commands:"
	@echo "  make deps         - Install dependencies"
	@echo "  make server       - Build server binary"
	@echo "  make client       - Build client binary"
	@echo "  make build        - Build both server and client"
	@echo "  make test         - Run tests"
	@echo "  make build-all    - Build for all supported platforms"
	@echo "  make packages     - Create all packages (APK and RPM)"
	@echo "  make apk          - Create APK packages for Linux"
	@echo "  make rpm          - Create RPM packages for Linux"
	@echo "  make clean        - Clean up build artifacts"
	@echo "  make help         - Show this help message"
	@echo ""
	@echo "Supported platforms: $(PLATFORMS)" 