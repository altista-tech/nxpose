package protocol

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// HandlerConfig holds configuration for protocol handlers
type HandlerConfig struct {
	// Local port to forward to
	LocalPort int
	// Remote address to listen on
	RemoteAddr string
	// TLS configuration for secure protocols
	TLSConfig *tls.Config
	// Logger for recording events
	Logger *logrus.Logger
	// Timeout for connections
	Timeout time.Duration
	// Context for cancellation
	Context context.Context
	// Optional: If the local service is actually HTTPS (not typical)
	LocalHTTPS bool
	// Optional: Custom headers to add to forwarded requests
	CustomHeaders map[string]string
}

// Handler interface for different protocol handlers
type Handler interface {
	// Forward forwards traffic from the public endpoint to the local service
	Forward(conn net.Conn) error
	// Start starts the handler with appropriate listeners
	Start() error
	// Stop stops the handler and releases resources
	Stop() error
	// GetMetrics returns handler metrics
	GetMetrics() map[string]interface{}
}

// BaseHandler contains common fields and methods for all handlers
type BaseHandler struct {
	config     HandlerConfig
	cancelFunc context.CancelFunc
	ctx        context.Context
	metrics    struct {
		requestsForwarded int64
		bytesForwarded    int64
		errorCount        int64
		lastRequestTime   time.Time
		activeConnections int
		mu                sync.RWMutex
	}
}

// newBaseHandler creates a new base handler
func newBaseHandler(config HandlerConfig) BaseHandler {
	// Set defaults for missing config values
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}
	if config.Logger == nil {
		config.Logger = logrus.New()
	}
	if config.Context == nil {
		config.Context = context.Background()
	}

	ctx, cancel := context.WithCancel(config.Context)

	return BaseHandler{
		config:     config,
		ctx:        ctx,
		cancelFunc: cancel,
	}
}

// updateMetrics updates handler metrics
func (h *BaseHandler) updateMetrics(requestsIncrement int64, bytesIncrement int64, errorIncrement int64, connectionDelta int) {
	h.metrics.mu.Lock()
	defer h.metrics.mu.Unlock()

	h.metrics.requestsForwarded += requestsIncrement
	h.metrics.bytesForwarded += bytesIncrement
	h.metrics.errorCount += errorIncrement
	h.metrics.activeConnections += connectionDelta
	h.metrics.lastRequestTime = time.Now()
}

// GetMetrics returns handler metrics
func (h *BaseHandler) GetMetrics() map[string]interface{} {
	h.metrics.mu.RLock()
	defer h.metrics.mu.RUnlock()

	return map[string]interface{}{
		"requests_forwarded": h.metrics.requestsForwarded,
		"bytes_forwarded":    h.metrics.bytesForwarded,
		"error_count":        h.metrics.errorCount,
		"active_connections": h.metrics.activeConnections,
		"last_request_time":  h.metrics.lastRequestTime,
		"uptime":             time.Since(h.metrics.lastRequestTime).String(),
	}
}

// HTTPHandler implements Handler for HTTP protocol
type HTTPHandler struct {
	BaseHandler
	proxy      *httputil.ReverseProxy
	server     *http.Server
	serverAddr string
}

// NewHTTPHandler creates a new HTTP protocol handler
func NewHTTPHandler(config HandlerConfig) *HTTPHandler {
	return &HTTPHandler{
		BaseHandler: newBaseHandler(config),
		serverAddr:  config.RemoteAddr,
	}
}

// setupProxy configures the reverse proxy for HTTP handler
func (h *HTTPHandler) setupProxy() error {
	// Determine if local service is HTTP or HTTPS
	scheme := "http"
	if h.config.LocalHTTPS {
		scheme = "https"
	}

	// Parse the URL for the local service
	localURL, err := url.Parse(fmt.Sprintf("%s://localhost:%d", scheme, h.config.LocalPort))
	if err != nil {
		return fmt.Errorf("failed to parse local URL: %w", err)
	}

	// Create a reverse proxy to the local service
	proxy := httputil.NewSingleHostReverseProxy(localURL)

	// Customize the director function to modify requests before forwarding
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		// Call the original director first
		originalDirector(req)

		// Add X-Forwarded headers
		req.Header.Set("X-Forwarded-Proto", "http")
		req.Header.Set("X-Forwarded-Host", req.Host)
		req.Header.Set("X-Forwarded-For", getClientIP(req))

		// Add user-specified custom headers
		for key, value := range h.config.CustomHeaders {
			req.Header.Set(key, value)
		}

		// Log the request if in debug mode
		if h.config.Logger.Level >= logrus.DebugLevel {
			h.config.Logger.WithFields(logrus.Fields{
				"method": req.Method,
				"path":   req.URL.Path,
				"host":   req.Host,
				"remote": getClientIP(req),
			}).Debug("Forwarding HTTP request")
		}
	}

	// Set up error handler
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		h.updateMetrics(0, 0, 1, 0)
		h.config.Logger.WithError(err).WithFields(logrus.Fields{
			"method": r.Method,
			"path":   r.URL.Path,
		}).Error("Error forwarding HTTP request")

		// Respond with an appropriate error
		w.WriteHeader(http.StatusBadGateway)
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("Error connecting to the local service"))
	}

	// Configure transport with timeouts and TLS settings
	proxy.Transport = &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   h.config.Timeout,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: h.config.LocalHTTPS, // Skip verification for local service
		},
	}

	h.proxy = proxy
	return nil
}

// wrapHandlerWithMetrics wraps an HTTP handler to record metrics
func (h *HTTPHandler) wrapHandlerWithMetrics(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startTime := time.Now()

		// Create a response wrapper to capture the response size
		rw := &responseWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK, // Default status code
		}

		// Record the active connection
		h.updateMetrics(0, 0, 0, 1)

		// Call the actual handler
		handler.ServeHTTP(rw, r)

		// Record metrics
		h.updateMetrics(1, int64(rw.size), 0, -1)

		// Log the request
		latency := time.Since(startTime)
		h.config.Logger.WithFields(logrus.Fields{
			"method":      r.Method,
			"path":        r.URL.Path,
			"status_code": rw.statusCode,
			"size":        rw.size,
			"latency_ms":  latency.Milliseconds(),
			"remote":      getClientIP(r),
		}).Info("HTTP request processed")
	})
}

// Start starts the HTTP handler
func (h *HTTPHandler) Start() error {
	// Set up the reverse proxy
	if err := h.setupProxy(); err != nil {
		return err
	}

	// Create wrapped handler that captures metrics
	handler := h.wrapHandlerWithMetrics(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.proxy.ServeHTTP(w, r)
	}))

	// Create HTTP server
	h.server = &http.Server{
		Addr:    h.serverAddr,
		Handler: handler,
		// Configure reasonable timeouts
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Start the server in a goroutine
	go func() {
		h.config.Logger.WithFields(logrus.Fields{
			"addr":     h.serverAddr,
			"protocol": "HTTP",
		}).Info("HTTP handler started")

		if err := h.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			h.config.Logger.WithError(err).Error("HTTP server failed")
		}
	}()

	return nil
}

// Forward forwards a connection directly
func (h *HTTPHandler) Forward(conn net.Conn) error {
	// Not used for HTTP as we use a reverse proxy instead
	return fmt.Errorf("direct connection forwarding not implemented for HTTP")
}

// Stop stops the HTTP handler
func (h *HTTPHandler) Stop() error {
	// Signal context cancellation
	h.cancelFunc()

	// Create shutdown context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Gracefully shut down the server
	if h.server != nil {
		if err := h.server.Shutdown(ctx); err != nil {
			h.config.Logger.WithError(err).Error("Error shutting down HTTP server")
			return err
		}
	}

	h.config.Logger.Info("HTTP handler stopped")
	return nil
}

// HTTPSHandler implements Handler for HTTPS protocol
type HTTPSHandler struct {
	HTTPHandler // Embed HTTP handler for reuse
}

// NewHTTPSHandler creates a new HTTPS protocol handler
func NewHTTPSHandler(config HandlerConfig) *HTTPSHandler {
	return &HTTPSHandler{
		HTTPHandler: HTTPHandler{
			BaseHandler: newBaseHandler(config),
			serverAddr:  config.RemoteAddr,
		},
	}
}

// setupProxy configures the reverse proxy for HTTPS handler
func (h *HTTPSHandler) setupProxy() error {
	// Determine if local service is HTTP or HTTPS (default to HTTP)
	scheme := "http"
	if h.config.LocalHTTPS {
		scheme = "https"
	}

	// Parse the URL for the local service
	localURL, err := url.Parse(fmt.Sprintf("%s://localhost:%d", scheme, h.config.LocalPort))
	if err != nil {
		return fmt.Errorf("failed to parse local URL: %w", err)
	}

	// Create a reverse proxy to the local service
	proxy := httputil.NewSingleHostReverseProxy(localURL)

	// Customize the director function to modify requests before forwarding
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		// Call the original director first
		originalDirector(req)

		// Add headers to indicate HTTPS
		req.Header.Set("X-Forwarded-Proto", "https")
		req.Header.Set("X-Forwarded-Host", req.Host)
		req.Header.Set("X-Forwarded-For", getClientIP(req))
		req.Header.Set("X-Forwarded-Port", "443")
		req.Header.Set("X-Forwarded-Ssl", "on")

		// Add user-specified custom headers
		for key, value := range h.config.CustomHeaders {
			req.Header.Set(key, value)
		}

		// Log the request if in debug mode
		if h.config.Logger.Level >= logrus.DebugLevel {
			h.config.Logger.WithFields(logrus.Fields{
				"method": req.Method,
				"path":   req.URL.Path,
				"host":   req.Host,
				"remote": getClientIP(req),
			}).Debug("Forwarding HTTPS request")
		}
	}

	// Set up error handler
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		h.updateMetrics(0, 0, 1, 0)
		h.config.Logger.WithError(err).WithFields(logrus.Fields{
			"method": r.Method,
			"path":   r.URL.Path,
		}).Error("Error forwarding HTTPS request")

		// Respond with an appropriate error
		w.WriteHeader(http.StatusBadGateway)
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("Error connecting to the local service"))
	}

	// Configure transport with timeouts and TLS settings
	proxy.Transport = &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   h.config.Timeout,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: h.config.LocalHTTPS, // Skip verification for local service
		},
	}

	h.proxy = proxy
	return nil
}

// Start starts the HTTPS handler
func (h *HTTPSHandler) Start() error {
	// Make sure we have a TLS config
	if h.config.TLSConfig == nil {
		return fmt.Errorf("TLS configuration is required for HTTPS handler")
	}

	// Set up the reverse proxy
	if err := h.setupProxy(); err != nil {
		return err
	}

	// Create wrapped handler that captures metrics
	handler := h.wrapHandlerWithMetrics(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.proxy.ServeHTTP(w, r)
	}))

	// Create HTTPS server with TLS config
	h.server = &http.Server{
		Addr:      h.serverAddr,
		Handler:   handler,
		TLSConfig: h.config.TLSConfig,
		// Configure reasonable timeouts
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Start the server in a goroutine
	go func() {
		h.config.Logger.WithFields(logrus.Fields{
			"addr":     h.serverAddr,
			"protocol": "HTTPS",
		}).Info("HTTPS handler started")

		// Start server with TLS
		if err := h.server.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
			h.config.Logger.WithError(err).Error("HTTPS server failed")
		}
	}()

	return nil
}

// TCPHandler implements Handler for TCP protocol
type TCPHandler struct {
	BaseHandler
	listener      net.Listener
	connections   map[string]net.Conn
	connectionsMu sync.RWMutex
	shutdownChan  chan struct{}
}

// NewTCPHandler creates a new TCP protocol handler
func NewTCPHandler(config HandlerConfig) *TCPHandler {
	return &TCPHandler{
		BaseHandler:  newBaseHandler(config),
		connections:  make(map[string]net.Conn),
		shutdownChan: make(chan struct{}),
	}
}

// Start starts the TCP handler
func (h *TCPHandler) Start() error {
	// Create a TCP listener
	var err error
	h.listener, err = net.Listen("tcp", h.config.RemoteAddr)
	if err != nil {
		return fmt.Errorf("failed to create TCP listener: %w", err)
	}

	// Handle incoming connections in a goroutine
	go h.acceptConnections()

	h.config.Logger.WithFields(logrus.Fields{
		"addr":     h.config.RemoteAddr,
		"protocol": "TCP",
	}).Info("TCP handler started")

	return nil
}

// acceptConnections accepts and handles incoming TCP connections
func (h *TCPHandler) acceptConnections() {
	for {
		// Check if we need to stop
		select {
		case <-h.ctx.Done():
			return
		case <-h.shutdownChan:
			return
		default:
			// Accept a connection with timeout
			h.listener.(*net.TCPListener).SetDeadline(time.Now().Add(1 * time.Second))
			conn, err := h.listener.Accept()
			if err != nil {
				// Check if this is a timeout (expected during shutdown)
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue
				}

				// Check if listener was closed (expected during shutdown)
				if strings.Contains(err.Error(), "use of closed network connection") {
					return
				}

				h.config.Logger.WithError(err).Error("Error accepting TCP connection")
				h.updateMetrics(0, 0, 1, 0)
				continue
			}

			// Handle the connection in a goroutine
			go h.handleConnection(conn)
		}
	}
}

// handleConnection handles a single TCP connection
func (h *TCPHandler) handleConnection(clientConn net.Conn) {
	// Generate a connection ID
	connID := fmt.Sprintf("%s-%d", clientConn.RemoteAddr().String(), time.Now().UnixNano())

	// Store the connection
	h.connectionsMu.Lock()
	h.connections[connID] = clientConn
	h.connectionsMu.Unlock()

	// Update metrics
	h.updateMetrics(0, 0, 0, 1)

	// Establish connection to local service
	localConn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", h.config.LocalPort), h.config.Timeout)
	if err != nil {
		h.config.Logger.WithError(err).WithField("local_port", h.config.LocalPort).Error("Failed to connect to local service")
		h.updateMetrics(0, 0, 1, -1)
		clientConn.Close()

		// Remove from connections map
		h.connectionsMu.Lock()
		delete(h.connections, connID)
		h.connectionsMu.Unlock()
		return
	}

	h.config.Logger.WithFields(logrus.Fields{
		"remote_addr": clientConn.RemoteAddr().String(),
		"local_port":  h.config.LocalPort,
	}).Debug("TCP connection established")

	// Create a wait group to wait for both copy operations
	var wg sync.WaitGroup
	wg.Add(2)

	// Copy from client to local service
	go func() {
		defer wg.Done()
		bytesCopied, err := io.Copy(localConn, clientConn)
		if err != nil && !isClosedConnError(err) {
			h.config.Logger.WithError(err).Error("Error copying data from client to local service")
			h.updateMetrics(0, 0, 1, 0)
		}
		h.updateMetrics(1, bytesCopied, 0, 0)

		// Signal the end of this direction by closing the write end
		if tcpConn, ok := localConn.(*net.TCPConn); ok {
			tcpConn.CloseWrite()
		} else {
			localConn.Close()
		}
	}()

	// Copy from local service to client
	go func() {
		defer wg.Done()
		bytesCopied, err := io.Copy(clientConn, localConn)
		if err != nil && !isClosedConnError(err) {
			h.config.Logger.WithError(err).Error("Error copying data from local service to client")
			h.updateMetrics(0, 0, 1, 0)
		}
		h.updateMetrics(1, bytesCopied, 0, 0)

		// Signal the end of this direction by closing the write end
		if tcpConn, ok := clientConn.(*net.TCPConn); ok {
			tcpConn.CloseWrite()
		} else {
			clientConn.Close()
		}
	}()

	// Wait for both copy operations to complete
	wg.Wait()

	// Close both connections
	localConn.Close()
	clientConn.Close()

	// Update metrics and clean up
	h.updateMetrics(0, 0, 0, -1)

	// Remove from connections map
	h.connectionsMu.Lock()
	delete(h.connections, connID)
	h.connectionsMu.Unlock()

	h.config.Logger.WithField("remote_addr", clientConn.RemoteAddr().String()).Debug("TCP connection closed")
}

// Forward forwards a TCP connection
func (h *TCPHandler) Forward(conn net.Conn) error {
	// Just handle the connection using our existing handler
	go h.handleConnection(conn)
	return nil
}

// Stop stops the TCP handler
func (h *TCPHandler) Stop() error {
	// Signal shutdown
	close(h.shutdownChan)
	h.cancelFunc()

	// Close the listener
	if h.listener != nil {
		h.listener.Close()
	}

	// Close all active connections
	h.connectionsMu.Lock()
	for id, conn := range h.connections {
		conn.Close()
		delete(h.connections, id)
	}
	h.connectionsMu.Unlock()

	h.config.Logger.Info("TCP handler stopped")
	return nil
}

// GetHandler returns the appropriate handler for the given protocol
func GetHandler(protocol string, config HandlerConfig) (Handler, error) {
	switch strings.ToLower(protocol) {
	case "http":
		return NewHTTPHandler(config), nil
	case "https":
		if config.TLSConfig == nil {
			return nil, fmt.Errorf("TLS configuration is required for HTTPS protocol")
		}
		return NewHTTPSHandler(config), nil
	case "tcp":
		return NewTCPHandler(config), nil
	default:
		return nil, fmt.Errorf("unsupported protocol: %s", protocol)
	}
}

// Helper types and functions

// responseWriter wraps http.ResponseWriter to capture response size and status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	size       int
}

// WriteHeader captures the status code
func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Write captures the response size
func (rw *responseWriter) Write(b []byte) (int, error) {
	size, err := rw.ResponseWriter.Write(b)
	rw.size += size
	return size, err
}

// getClientIP extracts the client IP from an HTTP request
func getClientIP(r *http.Request) string {
	// First check X-Forwarded-For header
	forwardedFor := r.Header.Get("X-Forwarded-For")
	if forwardedFor != "" {
		// X-Forwarded-For can contain multiple IPs (client, proxy1, proxy2, ...)
		// Take the leftmost one (client)
		ips := strings.Split(forwardedFor, ",")
		return strings.TrimSpace(ips[0])
	}

	// Then check X-Real-IP header
	realIP := r.Header.Get("X-Real-IP")
	if realIP != "" {
		return realIP
	}

	// Finally, use the direct remote address
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr // Return unsplit if there's an error
	}
	return ip
}

// isClosedConnError checks if an error is a closed connection error
func isClosedConnError(err error) bool {
	if err == nil {
		return false
	}
	// Check for common closed connection errors
	if err == io.EOF {
		return true
	}
	if strings.Contains(err.Error(), "use of closed network connection") {
		return true
	}
	if strings.Contains(err.Error(), "connection reset by peer") {
		return true
	}
	if strings.Contains(err.Error(), "broken pipe") {
		return true
	}
	return false
}
