// internal/server/server.go

package server

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"golang.org/x/net/context"
	"golang.org/x/net/websocket"
	"math/rand"
	"net/http"
	"nxpose/internal/crypto"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"nxpose/internal/config"
	"nxpose/internal/logger"
)

// Tunnel represents an active tunnel
type Tunnel struct {
	ID          string
	ClientID    string
	Protocol    string
	Subdomain   string
	TargetPort  int
	CreateTime  time.Time
	LastActive  time.Time
	connections int64
}

// TunnelRegistry keeps track of active tunnels
type TunnelRegistry struct {
	tunnels map[string]*Tunnel
	mu      sync.RWMutex
}

// Server represents the nxpose server
type Server struct {
	config    *config.ServerConfig
	tlsConfig *tls.Config
	log       *logger.Logger
	wsManager *WebSocketManager

	httpServer   *http.Server
	tunnelServer *http.Server
	tunnels      *TunnelRegistry

	mu       sync.Mutex
	stopping bool
}

// NewServer creates a new server instance
func NewServer(config *config.ServerConfig, tlsConfig *tls.Config, log *logger.Logger) (*Server, error) {
	return &Server{
		config:    config,
		tlsConfig: tlsConfig,
		log:       log,
		wsManager: NewWebSocketManager(),
		tunnels: &TunnelRegistry{
			tunnels: make(map[string]*Tunnel),
		},
	}, nil
}

// extractSubdomain extracts the subdomain from a hostname
func extractSubdomain(hostname, baseDomain string) string {
	// Remove potential port information
	if idx := strings.Index(hostname, ":"); idx != -1 {
		hostname = hostname[:idx]
	}

	// Check if hostname ends with baseDomain
	if !strings.HasSuffix(hostname, baseDomain) {
		return ""
	}

	// Remove the baseDomain part from hostname
	subdomain := hostname[:len(hostname)-len(baseDomain)-1] // -1 for the dot

	return subdomain
}

// handleWelcomePage shows a welcome page when no subdomain is provided
func (s *Server) handleWelcomePage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)

	welcomeHTML := `
<!DOCTYPE html>
<html>
<head>
    <title>NXpose Tunnel Service</title>
    <style>
        body {
            font-family: Arial, sans-serif;
            max-width: 800px;
            margin: 0 auto;
            padding: 20px;
        }
        h1 {
            color: #333;
        }
        .info {
            background-color: #f8f9fa;
            border-left: 4px solid #007bff;
            padding: 15px;
            margin-bottom: 20px;
        }
    </style>
</head>
<body>
    <h1>Welcome to NXpose Tunnel Service</h1>
    <div class="info">
        <p>This is the NXpose secure tunneling service that allows exposing local services to the internet through secure tunnels.</p>
        <p>To access a tunnel, use the subdomain format: <code>subdomain.` + s.config.BaseDomain + `</code></p>
    </div>
    <h2>Getting Started</h2>
    <p>Install the nxpose client and run:</p>
    <pre>nxpose register
nxpose expose http 3000</pre>
</body>
</html>
`
	w.Write([]byte(welcomeHTML))
}

// handleTunnelRequest handles incoming tunnel requests
func (s *Server) handleTunnelRequest(w http.ResponseWriter, r *http.Request) {
	// Extract subdomain from hostname
	host := r.Host
	subdomain := extractSubdomain(host, s.config.BaseDomain)

	s.log.WithFields(logrus.Fields{
		"host":      host,
		"subdomain": subdomain,
		"path":      r.URL.Path,
	}).Debug("Received tunnel request")

	// If no subdomain is found or it's empty, show a welcome page
	if subdomain == "" {
		s.handleWelcomePage(w, r)
		return
	}

	// Find the tunnel that matches this subdomain
	var tunnel *Tunnel
	s.tunnels.mu.RLock()
	for _, t := range s.tunnels.tunnels {
		if t.Subdomain == subdomain {
			tunnel = t
			break
		}
	}
	s.tunnels.mu.RUnlock()

	// If no matching tunnel is found, return 404
	if tunnel == nil {
		http.NotFound(w, r)
		return
	}

	// Update last active timestamp
	tunnel.LastActive = time.Now()
	tunnel.connections++

	// Forward the request based on protocol
	switch tunnel.Protocol {
	case "http", "https":
		// Get the WebSocket tunnel for this tunnel ID
		wsTunnel, exists := s.wsManager.GetWebSocketTunnel(tunnel.ID)
		if !exists {
			s.log.WithField("tunnel_id", tunnel.ID).Error("No WebSocket connection for tunnel")
			http.Error(w, "Tunnel not connected", http.StatusServiceUnavailable)
			return
		}

		// Forward the request to the client via WebSocket
		s.forwardHTTPRequest(w, r, wsTunnel)
	case "tcp":
		// For TCP, this would be handled by a different listener
		http.Error(w, "TCP tunneling not available via HTTP", http.StatusBadRequest)
	default:
		http.Error(w, "Unsupported protocol", http.StatusBadRequest)
	}
}

// forwardHTTPRequest forwards an HTTP request to the client via WebSocket
func (s *Server) forwardHTTPRequest(w http.ResponseWriter, r *http.Request, wsTunnel *WebSocketTunnel) {
	// Send the request to the client via WebSocket and wait for a response
	response, err := wsTunnel.sendHTTPRequest(r)
	if err != nil {
		s.log.WithError(err).Error("Failed to forward HTTP request to client")
		http.Error(w, "Failed to forward request", http.StatusInternalServerError)
		return
	}

	// Copy the response headers
	for key, value := range response.Headers {
		w.Header().Set(key, value)
	}

	// Set the status code
	w.WriteHeader(response.StatusCode)

	// Write the response body
	if len(response.Body) > 0 {
		w.Write(response.Body)
	}

	s.log.WithFields(logrus.Fields{
		"status_code": response.StatusCode,
		"body_size":   len(response.Body),
	}).Debug("HTTP response forwarded to client")
}

// Start starts the server, initializing HTTP and tunnel servers
func (s *Server) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.stopping {
		return fmt.Errorf("server is shutting down")
	}

	// Set up HTTP server
	mux := http.NewServeMux()

	// API handlers
	mux.HandleFunc("/api/register", s.handleRegister)
	mux.HandleFunc("/api/tunnel", s.handleTunnel)
	mux.HandleFunc("/api/ws", s.handleWebSocket)

	// Default handler for tunnel requests
	mux.HandleFunc("/", s.handleTunnelRequest)

	// Create HTTPS server for API
	s.httpServer = &http.Server{
		Addr:      fmt.Sprintf("%s:%d", s.config.BindAddress, s.config.Port),
		Handler:   mux,
		TLSConfig: s.tlsConfig,
	}

	// Create HTTP server on port 80 for tunnel traffic
	tunnelMux := http.NewServeMux()
	tunnelMux.HandleFunc("/", s.handleTunnelRequest) // Only handle tunnel requests
	s.tunnelServer = &http.Server{
		Addr:    fmt.Sprintf("%s:80", s.config.BindAddress),
		Handler: tunnelMux,
	}

	// Start monitoring
	s.startMonitoring()

	// Start HTTPS server in TLS mode
	s.log.Infof("Starting HTTPS server on %s:%d", s.config.BindAddress, s.config.Port)
	go func() {
		if err := s.httpServer.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
			s.log.Errorf("HTTPS server failed: %v", err)
		}
	}()

	// Start HTTP server for tunnels
	s.log.Infof("Starting HTTP server for tunnels on %s:80", s.config.BindAddress)
	go func() {
		if err := s.tunnelServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.log.Errorf("HTTP tunnel server failed: %v", err)
		}
	}()

	return nil
}

func (s *Server) Stop() error {
	s.mu.Lock()
	if s.stopping {
		s.mu.Unlock()
		return nil
	}
	s.stopping = true
	s.mu.Unlock()

	// Create context with 15s timeout for graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Shutdown HTTP server
	if s.httpServer != nil {
		s.log.Info("Shutting down HTTPS server")
		if err := s.httpServer.Shutdown(ctx); err != nil {
			s.log.Errorf("Error shutting down HTTPS server: %v", err)
		}
	}

	// Shutdown tunnel server
	if s.tunnelServer != nil {
		s.log.Info("Shutting down HTTP tunnel server")
		if err := s.tunnelServer.Shutdown(ctx); err != nil {
			s.log.Errorf("Error shutting down HTTP tunnel server: %v", err)
		}
	}

	// Close all WebSocket connections
	s.log.Info("Closing all WebSocket connections")
	s.wsManager.CloseAll()

	s.log.Info("Server shutdown complete")
	return nil
}

// Add to the internal/server/server.go file

// RegistrationRequest represents a client registration request
type RegistrationRequest struct {
	ClientName   string `json:"client_name"`
	ClientRegion string `json:"client_region,omitempty"`
}

// RegistrationResponse represents a client registration response
type RegistrationResponse struct {
	Success     bool      `json:"success"`
	Message     string    `json:"message,omitempty"`
	ClientID    string    `json:"client_id"`
	Certificate string    `json:"certificate"`
	ExpiresAt   time.Time `json:"expires_at"`
}

// TunnelRequest represents a tunnel creation request
type TunnelRequest struct {
	ClientID    string `json:"client_id"`
	Protocol    string `json:"protocol"`
	Port        int    `json:"port"`
	Certificate string `json:"certificate"`
}

// TunnelResponse represents a tunnel creation response
type TunnelResponse struct {
	Success   bool   `json:"success"`
	Message   string `json:"message,omitempty"`
	TunnelID  string `json:"tunnel_id"`
	PublicURL string `json:"public_url"`
}

// handleRegister handles client registration requests
func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	// Only accept POST requests
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request body
	var reqBody RegistrationRequest
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Generate client ID
	clientID := uuid.New().String()

	// Generate or sign client certificate
	// In a real implementation, this would use crypto.SignClientCertificate
	// or similar to create a proper client certificate
	certPEM, err := crypto.GenerateDummyClientCertificate()
	if err != nil {
		s.log.WithError(err).Error("Failed to generate client certificate")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Certificate expiration (30 days from now)
	expiresAt := time.Now().Add(30 * 24 * time.Hour)

	// Create response
	resp := RegistrationResponse{
		Success:     true,
		ClientID:    clientID,
		Certificate: string(certPEM),
		ExpiresAt:   expiresAt,
	}

	// Send response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)

	s.log.WithFields(logrus.Fields{
		"client_id":   clientID,
		"client_name": reqBody.ClientName,
	}).Info("Client registered successfully")
}

// handleTunnel handles tunnel creation requests
func (s *Server) handleTunnel(w http.ResponseWriter, r *http.Request) {
	// Only accept POST requests
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request body
	var reqBody TunnelRequest
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate certificate (in a real implementation)
	// For now, we'll just check that it's not empty
	if reqBody.Certificate == "" {
		http.Error(w, "Invalid or missing certificate", http.StatusBadRequest)
		return
	}

	// Generate tunnel ID
	tunnelID := uuid.New().String()

	// Generate a random subdomain
	subdomain := generateRandomSubdomain(8)

	// Create tunnel
	tunnel := &Tunnel{
		ID:         tunnelID,
		ClientID:   reqBody.ClientID,
		Protocol:   reqBody.Protocol,
		Subdomain:  subdomain,
		TargetPort: reqBody.Port,
		CreateTime: time.Now(),
		LastActive: time.Now(),
	}

	// Store tunnel
	s.tunnels.mu.Lock()
	s.tunnels.tunnels[tunnelID] = tunnel
	s.tunnels.mu.Unlock()

	// Construct public URL
	publicURL := fmt.Sprintf("%s://%s.%s", reqBody.Protocol, subdomain, s.config.BaseDomain)

	// Create response
	resp := TunnelResponse{
		Success:   true,
		TunnelID:  tunnelID,
		PublicURL: publicURL,
	}

	// Send response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)

	s.log.WithFields(logrus.Fields{
		"tunnel_id":  tunnelID,
		"client_id":  reqBody.ClientID,
		"protocol":   reqBody.Protocol,
		"port":       reqBody.Port,
		"subdomain":  subdomain,
		"public_url": publicURL,
	}).Info("Tunnel created successfully")
}

// handleWebSocket handles WebSocket connection requests
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Get client ID from query parameters
	clientID := r.URL.Query().Get("client_id")
	tunnelID := r.URL.Query().Get("tunnel_id") // Make sure you're getting this

	if clientID == "" {
		http.Error(w, "Missing client ID", http.StatusBadRequest)
		return
	}

	// Debug the headers and parameters
	s.log.WithFields(logrus.Fields{
		"client_id": clientID,
		"tunnel_id": tunnelID,
		"headers":   r.Header,
	}).Debug("WebSocket connection attempt")

	// Upgrade to WebSocket
	websocket.Handler(func(ws *websocket.Conn) {
		s.handleWebSocketConnection(ws, clientID)
	}).ServeHTTP(w, r)
}

// generateRandomSubdomain generates a random subdomain of the specified length
func generateRandomSubdomain(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	result := make([]byte, length)

	for i := range result {
		// Generate random index in charset
		randIndex := rand.Intn(len(charset))
		result[i] = charset[randIndex]
	}

	return string(result)
}

// Add the CloseAll method to WebSocketManager
func (m *WebSocketManager) CloseAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Close all WebSocket tunnels
	for _, tunnel := range m.tunnels {
		tunnel.Close()
	}

	// Clear the tunnels map
	m.tunnels = make(map[string]*WebSocketTunnel)

	// Clear the requests map and close all waiting channels
	for id, ch := range m.requests {
		close(ch)
		delete(m.requests, id)
	}
}
