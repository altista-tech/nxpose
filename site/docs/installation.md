# Installation

## From Prebuilt Binaries

Download the latest release for your platform from the [releases page](https://github.com/yourusername/nxpose/releases).

### Linux

```bash
# AMD64
wget https://github.com/yourusername/nxpose/releases/download/v0.1.0/nxpose-linux-amd64.tar.gz
tar xzf nxpose-linux-amd64.tar.gz
sudo mv nxpose /usr/local/bin/

# ARM64
wget https://github.com/yourusername/nxpose/releases/download/v0.1.0/nxpose-linux-arm64.tar.gz
tar xzf nxpose-linux-arm64.tar.gz
sudo mv nxpose /usr/local/bin/
```

### macOS

```bash
# Intel
wget https://github.com/yourusername/nxpose/releases/download/v0.1.0/nxpose-darwin-amd64.tar.gz
tar xzf nxpose-darwin-amd64.tar.gz
sudo mv nxpose /usr/local/bin/

# Apple Silicon
wget https://github.com/yourusername/nxpose/releases/download/v0.1.0/nxpose-darwin-arm64.tar.gz
tar xzf nxpose-darwin-arm64.tar.gz
sudo mv nxpose /usr/local/bin/
```

### Windows

Download the appropriate ZIP file from the releases page and extract it to a directory in your PATH.

## From Packages

For Linux systems, DEB packages are available:

```bash
# Debian/Ubuntu AMD64
wget https://github.com/yourusername/nxpose/releases/download/v0.1.0/nxpose_0.1.0_linux_amd64.deb
sudo dpkg -i nxpose_0.1.0_linux_amd64.deb

# Debian/Ubuntu ARM64
wget https://github.com/yourusername/nxpose/releases/download/v0.1.0/nxpose_0.1.0_linux_arm64.deb
sudo dpkg -i nxpose_0.1.0_linux_arm64.deb
```

## From Source

Requires Go 1.21 or later:

```bash
# Clone the repository
git clone https://github.com/yourusername/nxpose.git
cd nxpose

# Build both client and server
make build

# The binaries will be in ./bin directory
./bin/nxpose --help
./bin/nxpose-server --help

# Install to your system
sudo make install
```

## Using Go Install

```bash
# Install client
go install github.com/yourusername/nxpose/cmd/client@latest

# Install server
go install github.com/yourusername/nxpose/cmd/server@latest
```

## Verify Installation

```bash
nxpose --version
```

## Next Steps

After installation, proceed to [Quick Start](quickstart.md) to create your first tunnel.
