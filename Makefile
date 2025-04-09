# Makefile for nxpose
# Builds the nxpose server and creates packages for macOS and Linux (ARM and AMD64)
# Supports multiple package formats: .deb, .rpm for Linux and .pkg for macOS

# Variables
NAME := nxpose
VERSION := 1.0.0
# Default RPM release number
RPM_RELEASE ?= 1
GO := go
GOFMT := gofmt
GOBUILD := $(GO) build
GOTEST := $(GO) test
BINARY_NAME := nxpose-server

# Detect operating system and architecture
OS := $(shell uname -s)
HOST_ARCH := $(shell uname -m)

# Target architecture can be specified, default to host architecture
ARCH ?= $(HOST_ARCH)

# Normalize architecture names
ifeq ($(ARCH),x86_64)
  ARCH := amd64
  GOARCH := amd64
else ifeq ($(ARCH),amd64)
  GOARCH := amd64
else ifeq ($(ARCH),arm64)
  GOARCH := arm64
else ifeq ($(ARCH),aarch64)
  ARCH := arm64
  GOARCH := arm64
endif

# Linux package formats
LINUX_PACKAGE_FORMATS := deb rpm

# OS-specific settings
ifeq ($(OS),Darwin)
  # macOS settings
  PACKAGE_FORMAT ?= pkg
  PACKAGE_NAME := $(NAME)-$(VERSION)-$(ARCH).$(PACKAGE_FORMAT)
  PKG_IDENTIFIER := com.example.$(NAME)
  PKG_SCRIPTS := pkg_scripts
else
  # Linux settings
  PACKAGE_FORMAT ?= deb
  # Fix the naming convention for deb packages
  ifeq ($(PACKAGE_FORMAT),deb)
    PACKAGE_NAME := $(NAME)_$(VERSION)_$(ARCH).$(PACKAGE_FORMAT)
  else ifeq ($(PACKAGE_FORMAT),rpm)
    PACKAGE_NAME := $(NAME)-$(VERSION)-$(RPM_RELEASE).$(ARCH).$(PACKAGE_FORMAT)
  endif
endif

# Directories
BUILD_DIR := build/$(ARCH)
BIN_DIR := $(BUILD_DIR)/bin
ifeq ($(OS),Darwin)
  # macOS package structure
  INSTALL_DIR := $(BUILD_DIR)/payload/usr/local/bin
  CONFIG_DIR := $(BUILD_DIR)/payload/etc/$(NAME)
  LAUNCHD_DIR := $(BUILD_DIR)/payload/Library/LaunchDaemons
  SCRIPTS_DIR := $(BUILD_DIR)/scripts
else
  # Linux package structure
  DEBIAN_DIR := $(BUILD_DIR)/DEBIAN
  INSTALL_DIR := $(BUILD_DIR)/opt/$(NAME)
  CONFIG_DIR := $(BUILD_DIR)/etc/$(NAME)
  SYSTEMD_DIR := $(BUILD_DIR)/etc/systemd/system
  LOG_DIR := $(BUILD_DIR)/var/log/$(NAME)
  
  # Debian package files
  DEBIAN_CONTROL := $(DEBIAN_DIR)/control
  DEBIAN_POSTINST := $(DEBIAN_DIR)/postinst
  DEBIAN_PRERM := $(DEBIAN_DIR)/prerm
  SYSTEMD_SERVICE := $(SYSTEMD_DIR)/$(NAME).service
  
  # RPM package directories
  RPM_BUILD_DIR := $(BUILD_DIR)/rpmbuild
  SPEC_FILE := $(RPM_BUILD_DIR)/SPECS/$(NAME).spec
  RPM_SOURCES_DIR := $(RPM_BUILD_DIR)/SOURCES
  RPM_BUILDROOT := $(RPM_BUILD_DIR)/BUILDROOT/$(NAME)-$(VERSION)-1.$(ARCH)
endif

# Default target
all: clean build package

# Build for all architectures and package formats
all-arch:
	@$(MAKE) clean-all
ifeq ($(OS),Darwin)
	@$(MAKE) ARCH=amd64
	@$(MAKE) ARCH=arm64
else
	@echo "Building all packages for Linux..."
	@for arch in amd64 arm64; do \
		for fmt in $(LINUX_PACKAGE_FORMATS); do \
			echo "Building $$fmt package for $$arch..."; \
			$(MAKE) ARCH=$$arch PACKAGE_FORMAT=$$fmt; \
		done; \
	done
endif

# Build the binary
build:
	@echo "Building $(BINARY_NAME) for $(ARCH)..."
	@mkdir -p $(BIN_DIR)
	GOOS=$(if $(filter Darwin,$(OS)),darwin,linux) GOARCH=$(GOARCH) $(GOBUILD) -o $(BIN_DIR)/$(BINARY_NAME) ./cmd/server

# Platform-specific package preparation
prepare-package: build
ifeq ($(OS),Darwin)
	@echo "Preparing macOS package for $(ARCH)..."
	@mkdir -p $(INSTALL_DIR)
	@mkdir -p $(CONFIG_DIR)
	@mkdir -p $(LAUNCHD_DIR)
	@mkdir -p $(SCRIPTS_DIR)
	@cp $(BIN_DIR)/$(BINARY_NAME) $(INSTALL_DIR)/
	@cp server-config.example.yaml $(CONFIG_DIR)/server-config.yaml
	@echo "<?xml version=\"1.0\" encoding=\"UTF-8\"?>" > $(LAUNCHD_DIR)/com.example.$(NAME).plist
	@echo "<!DOCTYPE plist PUBLIC \"-//Apple//DTD PLIST 1.0//EN\" \"http://www.apple.com/DTDs/PropertyList-1.0.dtd\">" >> $(LAUNCHD_DIR)/com.example.$(NAME).plist
	@echo "<plist version=\"1.0\">" >> $(LAUNCHD_DIR)/com.example.$(NAME).plist
	@echo "  <dict>" >> $(LAUNCHD_DIR)/com.example.$(NAME).plist
	@echo "    <key>Label</key>" >> $(LAUNCHD_DIR)/com.example.$(NAME).plist
	@echo "    <string>com.example.$(NAME)</string>" >> $(LAUNCHD_DIR)/com.example.$(NAME).plist
	@echo "    <key>ProgramArguments</key>" >> $(LAUNCHD_DIR)/com.example.$(NAME).plist
	@echo "    <array>" >> $(LAUNCHD_DIR)/com.example.$(NAME).plist
	@echo "      <string>/usr/local/bin/$(BINARY_NAME)</string>" >> $(LAUNCHD_DIR)/com.example.$(NAME).plist
	@echo "      <string>--config</string>" >> $(LAUNCHD_DIR)/com.example.$(NAME).plist
	@echo "      <string>/etc/$(NAME)/server-config.yaml</string>" >> $(LAUNCHD_DIR)/com.example.$(NAME).plist
	@echo "    </array>" >> $(LAUNCHD_DIR)/com.example.$(NAME).plist
	@echo "    <key>RunAtLoad</key>" >> $(LAUNCHD_DIR)/com.example.$(NAME).plist
	@echo "    <true/>" >> $(LAUNCHD_DIR)/com.example.$(NAME).plist
	@echo "    <key>KeepAlive</key>" >> $(LAUNCHD_DIR)/com.example.$(NAME).plist
	@echo "    <true/>" >> $(LAUNCHD_DIR)/com.example.$(NAME).plist
	@echo "    <key>StandardOutPath</key>" >> $(LAUNCHD_DIR)/com.example.$(NAME).plist
	@echo "    <string>/var/log/$(NAME).log</string>" >> $(LAUNCHD_DIR)/com.example.$(NAME).plist
	@echo "    <key>StandardErrorPath</key>" >> $(LAUNCHD_DIR)/com.example.$(NAME).plist
	@echo "    <string>/var/log/$(NAME).log</string>" >> $(LAUNCHD_DIR)/com.example.$(NAME).plist
	@echo "  </dict>" >> $(LAUNCHD_DIR)/com.example.$(NAME).plist
	@echo "</plist>" >> $(LAUNCHD_DIR)/com.example.$(NAME).plist
	@echo "#!/bin/sh" > $(SCRIPTS_DIR)/postinstall
	@echo "# Load the LaunchDaemon" >> $(SCRIPTS_DIR)/postinstall
	@echo "launchctl load /Library/LaunchDaemons/com.example.$(NAME).plist" >> $(SCRIPTS_DIR)/postinstall
	@chmod 755 $(SCRIPTS_DIR)/postinstall
	@echo "#!/bin/sh" > $(SCRIPTS_DIR)/preinstall
	@echo "# Unload the LaunchDaemon if it exists" >> $(SCRIPTS_DIR)/preinstall
	@echo "launchctl unload /Library/LaunchDaemons/com.example.$(NAME).plist 2>/dev/null || true" >> $(SCRIPTS_DIR)/preinstall
	@chmod 755 $(SCRIPTS_DIR)/preinstall
else
	@echo "Preparing Linux $(PACKAGE_FORMAT) package for $(ARCH)..."
	@mkdir -p $(INSTALL_DIR)
	@mkdir -p $(CONFIG_DIR)
	@mkdir -p $(SYSTEMD_DIR)
	@mkdir -p $(LOG_DIR)
	@cp $(BIN_DIR)/$(BINARY_NAME) $(INSTALL_DIR)/
	@cp server-config.example.yaml $(CONFIG_DIR)/server-config.yaml
	@cp nxpose.service $(SYSTEMD_SERVICE)
	
	# Format-specific preparation
ifeq ($(PACKAGE_FORMAT),deb)
	@mkdir -p $(DEBIAN_DIR)
	@echo "Package: $(NAME)" > $(DEBIAN_CONTROL)
	@echo "Version: $(VERSION)" >> $(DEBIAN_CONTROL)
	@echo "Section: net" >> $(DEBIAN_CONTROL)
	@echo "Priority: optional" >> $(DEBIAN_CONTROL)
	@echo "Architecture: $(ARCH)" >> $(DEBIAN_CONTROL)
	@echo "Maintainer: Your Name <your.email@example.com>" >> $(DEBIAN_CONTROL)
	@echo "Description: nxpose tunneling server" >> $(DEBIAN_CONTROL)
	@echo " A secure tunneling service to expose local services to the internet." >> $(DEBIAN_CONTROL)
	@echo "#!/bin/sh" > $(DEBIAN_POSTINST)
	@echo "set -e" >> $(DEBIAN_POSTINST)
	@echo "# Create nxpose user and group if they don't exist" >> $(DEBIAN_POSTINST)
	@echo "if ! getent group nxpose >/dev/null; then" >> $(DEBIAN_POSTINST)
	@echo "  groupadd --system nxpose" >> $(DEBIAN_POSTINST)
	@echo "fi" >> $(DEBIAN_POSTINST)
	@echo "if ! getent passwd nxpose >/dev/null; then" >> $(DEBIAN_POSTINST)
	@echo "  useradd --system --gid nxpose --shell /sbin/nologin --home-dir /opt/nxpose nxpose" >> $(DEBIAN_POSTINST)
	@echo "fi" >> $(DEBIAN_POSTINST)
	@echo "# Set proper permissions" >> $(DEBIAN_POSTINST)
	@echo "chown -R nxpose:nxpose /opt/nxpose" >> $(DEBIAN_POSTINST)
	@echo "chown -R nxpose:nxpose /etc/nxpose" >> $(DEBIAN_POSTINST)
	@echo "chown -R nxpose:nxpose /var/log/nxpose" >> $(DEBIAN_POSTINST)
	@echo "# Enable and start the systemd service" >> $(DEBIAN_POSTINST)
	@echo "systemctl daemon-reload" >> $(DEBIAN_POSTINST)
	@echo "systemctl enable $(NAME).service" >> $(DEBIAN_POSTINST)
	@echo "systemctl start $(NAME).service || true" >> $(DEBIAN_POSTINST)
	@echo "exit 0" >> $(DEBIAN_POSTINST)
	@chmod 755 $(DEBIAN_POSTINST)
	@echo "#!/bin/sh" > $(DEBIAN_PRERM)
	@echo "set -e" >> $(DEBIAN_PRERM)
	@echo "# Stop and disable the systemd service" >> $(DEBIAN_PRERM)
	@echo "systemctl stop $(NAME).service || true" >> $(DEBIAN_PRERM)
	@echo "systemctl disable $(NAME).service || true" >> $(DEBIAN_PRERM)
	@echo "exit 0" >> $(DEBIAN_PRERM)
	@chmod 755 $(DEBIAN_PRERM)
else ifeq ($(PACKAGE_FORMAT),rpm)
	@mkdir -p $(RPM_BUILD_DIR)/SPECS $(RPM_BUILD_DIR)/SOURCES $(RPM_BUILD_DIR)/RPMS $(RPM_BUILD_DIR)/SRPMS $(RPM_BUILD_DIR)/BUILD
	@echo "Name: $(NAME)" > $(SPEC_FILE)
	@echo "Version: $(VERSION)" >> $(SPEC_FILE)
	@echo "Release: $(RPM_RELEASE)" >> $(SPEC_FILE)
	@echo "Summary: nxpose tunneling server" >> $(SPEC_FILE)
	@echo "Group: Applications/Internet" >> $(SPEC_FILE)
	@echo "License: MIT" >> $(SPEC_FILE)
	@echo "BuildArch: $(ARCH)" >> $(SPEC_FILE)
	@echo "AutoReqProv: no" >> $(SPEC_FILE)
	@echo "" >> $(SPEC_FILE)
	@echo "%description" >> $(SPEC_FILE)
	@echo "A secure tunneling service to expose local services to the internet." >> $(SPEC_FILE)
	@echo "" >> $(SPEC_FILE)
	@echo "%pre" >> $(SPEC_FILE)
	@echo "getent group nxpose >/dev/null || groupadd -r nxpose" >> $(SPEC_FILE)
	@echo "getent passwd nxpose >/dev/null || useradd -r -g nxpose -s /sbin/nologin -d /opt/nxpose nxpose" >> $(SPEC_FILE)
	@echo "" >> $(SPEC_FILE)
	@echo "%files" >> $(SPEC_FILE)
	@echo "/opt/$(NAME)/" >> $(SPEC_FILE)
	@echo "/etc/$(NAME)/" >> $(SPEC_FILE)
	@echo "/etc/systemd/system/$(NAME).service" >> $(SPEC_FILE)
	@echo "/var/log/$(NAME)/" >> $(SPEC_FILE)
	@echo "" >> $(SPEC_FILE)
	@echo "%post" >> $(SPEC_FILE)
	@echo "chown -R nxpose:nxpose /opt/nxpose" >> $(SPEC_FILE)
	@echo "chown -R nxpose:nxpose /etc/nxpose" >> $(SPEC_FILE)
	@echo "chown -R nxpose:nxpose /var/log/nxpose" >> $(SPEC_FILE)
	@echo "systemctl daemon-reload" >> $(SPEC_FILE)
	@echo "systemctl enable $(NAME).service" >> $(SPEC_FILE)
	@echo "systemctl start $(NAME).service || true" >> $(SPEC_FILE)
	@echo "" >> $(SPEC_FILE)
	@echo "%preun" >> $(SPEC_FILE)
	@echo "systemctl stop $(NAME).service || true" >> $(SPEC_FILE)
	@echo "systemctl disable $(NAME).service || true" >> $(SPEC_FILE)
	@echo "systemctl daemon-reload" >> $(SPEC_FILE)
endif
endif

# Create the package
package: prepare-package
ifeq ($(OS),Darwin)
	@echo "Creating macOS package for $(ARCH)..."
	@pkgbuild --root $(BUILD_DIR)/payload \
		--scripts $(SCRIPTS_DIR) \
		--identifier $(PKG_IDENTIFIER) \
		--version $(VERSION) \
		--install-location / \
		$(PACKAGE_NAME)
else
	@echo "Creating Linux $(PACKAGE_FORMAT) package for $(ARCH)..."
ifeq ($(PACKAGE_FORMAT),deb)
	# Ensure proper Debian package file permissions
	@chmod 755 $(DEBIAN_DIR)
	@chmod 644 $(DEBIAN_CONTROL)
	@find $(BUILD_DIR) -type d -exec chmod 755 {} \;
	@find $(BUILD_DIR)/opt -type f -exec chmod 755 {} \;
	@find $(BUILD_DIR)/etc -type f -exec chmod 644 {} \;
	@chmod 644 $(SYSTEMD_SERVICE)
	@chmod 755 $(DEBIAN_POSTINST) $(DEBIAN_PRERM)
	
	# Calculate the installed size in KB
	@du -sk --exclude=DEBIAN $(BUILD_DIR) > $(BUILD_DIR)/.size.tmp
	@echo "Installed-Size: $$(cat $(BUILD_DIR)/.size.tmp | cut -f1)" >> $(DEBIAN_CONTROL)
	@rm -f $(BUILD_DIR)/.size.tmp
	
	# Create debian-binary file (required for .deb format)
	@echo "2.0" > $(BUILD_DIR)/debian-binary
	
	# Use dpkg-deb with proper settings
	@dpkg-deb --build --root-owner-group $(BUILD_DIR) ./$(PACKAGE_NAME)
	@echo "Created package: $(PACKAGE_NAME)"
else ifeq ($(PACKAGE_FORMAT),rpm)
	@mkdir -p $(RPM_BUILDROOT)
	@cp -r $(INSTALL_DIR) $(RPM_BUILDROOT)/opt/
	@cp -r $(CONFIG_DIR) $(RPM_BUILDROOT)/etc/
	@mkdir -p $(RPM_BUILDROOT)/etc/systemd/system
	@cp $(SYSTEMD_SERVICE) $(RPM_BUILDROOT)/etc/systemd/system/
	@mkdir -p $(RPM_BUILDROOT)/var/log/$(NAME)
	@rpmbuild --define "_topdir $(RPM_BUILD_DIR)" \
		--define "_build_arch $(ARCH)" \
		--define "_target_cpu $(if $(filter arm64,$(ARCH)),aarch64,$(ARCH))" \
		--define "_target_os linux" \
		-bb $(SPEC_FILE)
	@mkdir -p $(dir $(PACKAGE_NAME))
	@cp $(RPM_BUILD_DIR)/RPMS/$(if $(filter arm64,$(ARCH)),aarch64,$(ARCH))/$(NAME)-$(VERSION)-$(RPM_RELEASE).$(if $(filter arm64,$(ARCH)),aarch64,$(ARCH)).rpm ./$(PACKAGE_NAME)
endif
endif

# Install the package
install: package
ifeq ($(OS),Darwin)
	@echo "Installing macOS package for $(ARCH)..."
	@sudo installer -pkg $(PACKAGE_NAME) -target /
else
	@echo "Installing Linux $(PACKAGE_FORMAT) package for $(ARCH)..."
ifeq ($(PACKAGE_FORMAT),deb)
	@sudo dpkg -i $(PACKAGE_NAME)
else ifeq ($(PACKAGE_FORMAT),rpm)
	@sudo rpm -i $(PACKAGE_NAME)
endif
endif

# Uninstall the package
uninstall:
ifeq ($(OS),Darwin)
	@echo "Uninstalling from macOS..."
	@sudo launchctl unload /Library/LaunchDaemons/com.example.$(NAME).plist || true
	@sudo rm -f /Library/LaunchDaemons/com.example.$(NAME).plist
	@sudo rm -f /usr/local/bin/$(BINARY_NAME)
	@sudo rm -rf /etc/$(NAME)
else
	@echo "Uninstalling from Linux..."
ifeq ($(PACKAGE_FORMAT),deb)
	@sudo dpkg -r $(NAME)
else ifeq ($(PACKAGE_FORMAT),rpm)
	@sudo rpm -e $(NAME)
endif
endif

# Clean build directory for current architecture
clean:
	@echo "Cleaning build directory for $(ARCH)..."
	@rm -rf $(BUILD_DIR)
	@rm -f $(NAME)*$(ARCH).pkg $(NAME)*$(ARCH).deb $(NAME)*$(ARCH).rpm

# Clean all build directories
clean-all:
	@echo "Cleaning all build directories..."
	@rm -rf build
	@rm -f $(NAME)*.pkg $(NAME)*.deb $(NAME)*.rpm

# Test
test:
	@echo "Running tests..."
	$(GOTEST) -v ./...

# Format code
fmt:
	@echo "Formatting code..."
	$(GOFMT) -s -w .

# Help
help:
	@echo "Available targets:"
	@echo "  all        - Clean, build and package for current architecture ($(ARCH))"
	@echo "  all-arch   - Build and package for all architectures (amd64, arm64) and formats (deb,rpm)"
	@echo "  build      - Build the binary for current architecture"
	@echo "  prepare-package - Prepare the package structure"
	@echo "  package    - Create the package"
	@echo "  install    - Install the package"
	@echo "  uninstall  - Uninstall the package"
	@echo "  clean      - Clean build directory for current architecture"
	@echo "  clean-all  - Clean build directory for all architectures"
	@echo "  test       - Run tests"
	@echo "  fmt        - Format code"
	@echo "  help       - Show this help"
	@echo ""
	@echo "Architecture can be specified with ARCH=<arch>"
	@echo "  Example: make ARCH=arm64"
	@echo "  Supported architectures: amd64, arm64"
	@echo ""
	@echo "Package format can be specified with PACKAGE_FORMAT=<format>"
	@echo "  Example: make PACKAGE_FORMAT=rpm"
	@echo "  Supported formats on Linux: deb, rpm"

.PHONY: all all-arch build prepare-package package install uninstall clean clean-all test fmt help