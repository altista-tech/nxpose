package protocol

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
)

// Handler interface for different protocol handlers
type Handler interface {
	// Forward forwards traffic from the public endpoint to the local service
	Forward(conn net.Conn) error

	// Start starts the handler with appropriate listeners
	Start() error

	// Stop stops the handler
	Stop() error
}

// HTTPHandler implements Handler for HTTP protocol
type HTTPHandler struct {
	LocalPort  int
	RemoteAddr string
	proxy      *httputil.ReverseProxy
	server     *http.Server
}

// NewHTTPHandler creates a new HTTP protocol handler
func NewHTTPHandler(localPort int, remoteAddr string) *HTTPHandler {
	return &HTTPHandler{
		LocalPort:  localPort,
		RemoteAddr: remoteAddr,
	}
}

// Start starts the HTTP handler
func (h *HTTPHandler) Start() error {
	// Set up the reverse proxy to forward requests to the local service
	localURL, err := url.Parse(fmt.Sprintf("http://localhost:%d", h.LocalPort))
	if err != nil {
		return fmt.Errorf("failed to parse local URL: %w", err)
	}

	h.proxy = httputil.NewSingleHostReverseProxy(localURL)

	// In a real implementation, this would be connected to the tunnel
	// For now, we just create a stub that would be used if we were running
	// a real server
	h.server = &http.Server{
		Addr: h.RemoteAddr,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h.proxy.ServeHTTP(w, r)
		}),
	}

	// In a real implementation, this would start the server in a goroutine
	// But since this is just a stub, we'll simulate it
	fmt.Printf("HTTP handler started for local port %d (simulated)\n", h.LocalPort)

	return nil
}

// Forward forwards a connection (used for non-HTTP protocols)
func (h *HTTPHandler) Forward(conn net.Conn) error {
	// Not used for HTTP as we use a reverse proxy instead
	return fmt.Errorf("direct connection forwarding not implemented for HTTP")
}

// Stop stops the HTTP handler
func (h *HTTPHandler) Stop() error {
	// In a real implementation, this would stop the server
	fmt.Println("HTTP handler stopped (simulated)")
	return nil
}

// TCPHandler implements Handler for TCP protocol
type TCPHandler struct {
	LocalPort   int
	RemoteAddr  string
	listener    net.Listener
	connections []net.Conn
	done        chan struct{}
}

// NewTCPHandler creates a new TCP protocol handler
func NewTCPHandler(localPort int, remoteAddr string) *TCPHandler {
	return &TCPHandler{
		LocalPort:   localPort,
		RemoteAddr:  remoteAddr,
		connections: make([]net.Conn, 0),
		done:        make(chan struct{}),
	}
}

// Start starts the TCP handler
func (t *TCPHandler) Start() error {
	// In a real implementation, this would start a listener
	// and accept connections to forward
	fmt.Printf("TCP handler started for local port %d (simulated)\n", t.LocalPort)
	return nil
}

// Forward forwards a TCP connection
func (t *TCPHandler) Forward(conn net.Conn) error {
	// In a real implementation, this would:
	// 1. Open a connection to the local service
	// 2. Copy data in both directions
	// For now, we just simulate that

	fmt.Printf("Forwarding TCP connection to localhost:%d (simulated)\n", t.LocalPort)

	// Track this connection
	t.connections = append(t.connections, conn)

	return nil
}

// Stop stops the TCP handler
func (t *TCPHandler) Stop() error {
	// In a real implementation, this would close the listener
	// and all active connections

	// Close the done channel to signal shutdown
	close(t.done)

	// Close all tracked connections
	for _, conn := range t.connections {
		conn.Close()
	}

	fmt.Println("TCP handler stopped (simulated)")
	return nil
}

// GetHandler returns the appropriate handler for the given protocol
func GetHandler(protocol string, localPort int, remoteAddr string) (Handler, error) {
	switch protocol {
	case "http":
		return NewHTTPHandler(localPort, remoteAddr), nil
	case "https":
		// In a real implementation, we'd set up TLS
		return NewHTTPHandler(localPort, remoteAddr), nil
	case "tcp":
		return NewTCPHandler(localPort, remoteAddr), nil
	default:
		return nil, fmt.Errorf("unsupported protocol: %s", protocol)
	}
}
