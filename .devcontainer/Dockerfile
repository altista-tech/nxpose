FROM mcr.microsoft.com/devcontainers/go:1-1.24-bookworm

RUN apt-get update && apt-get install -y \
    ca-certificates \
    curl \
    && rm -rf /var/lib/apt/lists/*

USER vscode
WORKDIR /workspace
