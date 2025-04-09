# Installing NXpose

This guide covers different methods for installing NXpose on your system. You can install both the client and server components depending on your needs.

## Prerequisites

Before installing NXpose, ensure you have the following:

- For running prebuilt binaries:
  - A supported operating system (Linux, macOS, or Windows)
  
- For building from source:
  - Go 1.20 or later
  - Git
  - Make (for Linux/macOS) or Windows with PowerShell

- For server operation:
  - A domain name with DNS configured to point to your server
  - (Optional) MongoDB for user storage
  - (Optional) Redis for session management
  - (Optional) Let's Encrypt for automatic TLS certificates

## Installation Methods

### From Prebuilt Binaries

The easiest way to install NXpose is to download the prebuilt binaries from the releases page.

1. Visit the [NXpose releases page](https://github.com/yourusername/nxpose/releases)
2. Download the appropriate binary for your platform:
   - `nxpose` (client) and `nxpose-server` for Linux/macOS
   - `nxpose.exe` and `nxpose-server.exe` for Windows
3. Make the files executable (Linux/macOS only):
   ```bash
   chmod +x nxpose nxpose-server
   ```
4. Move the binaries to a directory in your PATH:
   ```bash
   # Linux/macOS
   sudo mv nxpose nxpose-server /usr/local/bin/
   
   # Windows (PowerShell, run as Administrator)
   Move-Item nxpose.exe, nxpose-server.exe C:\Windows\System32\
   ```

### From Packages

For Linux systems, you can install NXpose using package managers.

#### RPM-based distributions (Fedora, CentOS, RHEL)

```bash
# AMD64 architecture
sudo rpm -i nxpose_version_x86_64.rpm

# ARM64 architecture
sudo rpm -i nxpose_version_aarch64.rpm
```

#### Alpine Linux (APK)

```bash
# AMD64 architecture
sudo apk add --allow-untrusted nxpose_version_x86_64.apk

# ARM64 architecture
sudo apk add --allow-untrusted nxpose_version_aarch64.apk
```

### From Source

Building from source gives you the latest features and allows you to customize the build:

```bash
# Clone the repository
git clone https://github.com/yourusername/nxpose.git
cd nxpose

# Build the client and server using Make
make build

# The binaries will be available in the './bin' directory
```

For Windows users without Make, use the included batch script:

```cmd
# Clone the repository
git clone https://github.com/yourusername/nxpose.git
cd nxpose

# Build with the batch script
build.bat build
```

### Using Go Install

If you have Go installed, you can install NXpose directly using `go install`:

```bash
# Install the client
go install github.com/yourusername/nxpose/cmd/client@latest

# Install the server
go install github.com/yourusername/nxpose/cmd/server@latest
```

## Environment Setup

### Client Setup

The NXpose client stores its configuration and certificates in the user's home directory:

- Linux/macOS: `~/.nxpose/`
- Windows: `%USERPROFILE%\.nxpose\`

This directory will be created automatically when you first use the client.

### Server Setup

For the server, you should create a configuration directory:

```bash
# Linux/macOS
sudo mkdir -p /etc/nxpose
sudo cp server-config.example.yaml /etc/nxpose/server-config.yaml

# Windows (PowerShell, run as Administrator)
New-Item -ItemType Directory -Force -Path C:\ProgramData\nxpose
Copy-Item server-config.example.yaml C:\ProgramData\nxpose\server-config.yaml
```

Then edit the configuration file to match your requirements before starting the server.

## Verifying Installation

After installation, verify that NXpose is installed correctly:

```bash
# Check client version
nxpose version

# Check server version
nxpose-server version
```

## Next Steps

After installation, proceed to the [Configuration](configuration.md) page to set up NXpose for your environment. 