FROM golang:1.21-alpine AS builder

WORKDIR /app

# Copy go.mod and go.sum first for caching
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source code
COPY . .

# Build the server binary
RUN CGO_ENABLED=0 GOOS=linux go build -o nxpose-server ./cmd/server

# Use a minimal alpine image for the final container
FROM alpine:3.18

WORKDIR /app

# Install necessary packages
RUN apk --no-cache add ca-certificates tzdata

# Copy the binary from the builder stage
COPY --from=builder /app/nxpose-server /app/

# Copy config files
COPY server-config.example.yaml /app/server-config.yaml

# Create directories for certificates
RUN mkdir -p /app/certs /app/certificates

# Expose ports
EXPOSE 80
EXPOSE 443

# Set environment variables
ENV NXPOSE_SERVER_PORT=443

# Run as non-root user
RUN addgroup -S nxpose && adduser -S nxpose -G nxpose
RUN chown -R nxpose:nxpose /app
USER nxpose

# Command to run when container starts
CMD ["/app/nxpose-server", "--config", "/app/server-config.yaml"]