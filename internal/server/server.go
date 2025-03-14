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
	"os"
	"path/filepath"
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

	httpServer         *http.Server
	tunnelServer       *http.Server
	acmeHTTPServer     *http.Server
	tunnels            *TunnelRegistry
	certificateManager *crypto.CertificateManager

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
		"scheme":    r.URL.Scheme,
		"proto":     r.Proto,
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

	// Determine if request is HTTP or HTTPS based on TLS connection
	isSecure := r.TLS != nil

	// Check if protocol matches the request scheme
	if (tunnel.Protocol == "https" && !isSecure) || (tunnel.Protocol == "http" && isSecure) {
		// Protocol mismatch, need to redirect
		var redirectURL string
		if tunnel.Protocol == "https" {
			// Redirect to HTTPS
			redirectURL = fmt.Sprintf("https://%s%s", r.Host, r.URL.Path)
			if r.URL.RawQuery != "" {
				redirectURL += "?" + r.URL.RawQuery
			}
			http.Redirect(w, r, redirectURL, http.StatusTemporaryRedirect)
			s.log.Debugf("Redirecting HTTP request to HTTPS: %s", redirectURL)
		} else {
			// This is unlikely (redirecting from HTTPS to HTTP), but handle it anyway
			redirectURL = fmt.Sprintf("http://%s%s", r.Host, r.URL.Path)
			if r.URL.RawQuery != "" {
				redirectURL += "?" + r.URL.RawQuery
			}
			http.Redirect(w, r, redirectURL, http.StatusTemporaryRedirect)
			s.log.Debugf("Redirecting HTTPS request to HTTP: %s", redirectURL)
		}
		return
	}

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

		// Set a custom header to indicate the original protocol
		if isSecure {
			r.Header.Set("X-Forwarded-Proto", "https")
		} else {
			r.Header.Set("X-Forwarded-Proto", "http")
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

// initCertificateManager initializes the Let's Encrypt certificate manager
func (s *Server) initCertificateManager(ctx context.Context) error {
	// Skip if Let's Encrypt is not enabled
	if !s.config.LetsEncrypt.Enabled {
		s.log.Info("Let's Encrypt is not enabled, using provided TLS certificates")
		return nil
	}

	s.log.Info("Initializing Let's Encrypt certificate manager...")

	// Validate required configuration
	if s.config.LetsEncrypt.Email == "" {
		return fmt.Errorf("Let's Encrypt email address is required")
	}

	// Set up domains to request certificates for
	domains := []string{
		"*." + s.config.BaseDomain, // Wildcard certificate
		s.config.BaseDomain,        // Base domain certificate
	}

	s.log.Infof("Requesting certificates for domains: %v", domains)

	// Determine storage directory
	storageDir := s.config.LetsEncrypt.StorageDir
	if storageDir == "" {
		// Use default location in home directory
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to determine home directory: %w", err)
		}
		storageDir = filepath.Join(homeDir, ".nxpose", "certificates")
		s.log.Infof("Using default certificate storage directory: %s", storageDir)
	}

	// Ensure the storage directory exists
	if err := os.MkdirAll(storageDir, 0700); err != nil {
		return fmt.Errorf("failed to create certificate storage directory: %w", err)
	}

	// Check directory permissions
	s.log.Debugf("Checking permissions on certificate storage directory: %s", storageDir)
	info, err := os.Stat(storageDir)
	if err != nil {
		s.log.Warnf("Failed to stat certificate storage directory: %v", err)
	} else {
		s.log.Debugf("Directory permissions: %v", info.Mode())
		if info.Mode().Perm()&0700 != 0700 {
			s.log.Warnf("Certificate directory has insufficient permissions. Setting to 0700")
			os.Chmod(storageDir, 0700)
		}
	}

	// Create HTTP server for ACME challenges on port 80
	acmeMux := http.NewServeMux()
	s.acmeHTTPServer = &http.Server{
		Addr:    fmt.Sprintf("%s:80", s.config.BindAddress),
		Handler: acmeMux,
	}

	// Check for DNS provider configuration
	hasDNSProvider := s.config.LetsEncrypt.DNSProvider != ""
	if !hasDNSProvider {
		s.log.Error("No DNS provider configured. Wildcard certificates REQUIRE DNS-01 challenge")
		s.log.Error("Please configure a DNS provider in your server-config.yaml file")
		s.log.Error("Example: letsencrypt.dns.provider: 'cloudflare'")
		s.log.Error("         letsencrypt.dns.credentials.api_token: 'your-token'")
		return fmt.Errorf("wildcard certificates require DNS-01 challenge provider")
	}

	s.log.Infof("Using DNS provider: %s for ACME DNS-01 challenges", s.config.LetsEncrypt.DNSProvider)

	// Ensure environment variables are populated if credentials use env vars
	for key, value := range s.config.LetsEncrypt.DNSCredentials {
		if strings.HasPrefix(value, "${") && strings.HasSuffix(value, "}") {
			envVar := strings.TrimSuffix(strings.TrimPrefix(value, "${"), "}")
			envValue := os.Getenv(envVar)
			if envValue == "" {
				s.log.Warnf("Environment variable %s not found or empty", envVar)
			} else {
				s.config.LetsEncrypt.DNSCredentials[key] = envValue
				s.log.Debugf("Using environment variable for %s", key)
			}
		}
	}

	// Check if the DNS credentials are configured
	if len(s.config.LetsEncrypt.DNSCredentials) == 0 {
		s.log.Error("DNS provider selected but no credentials provided")
		s.log.Error("Please add DNS credentials to your server-config.yaml")
		return fmt.Errorf("DNS provider credentials are required")
	}

	// Define ACME environment directory
	acmeEnvStr := "production"
	acmeEnv := crypto.ProductionEnv
	if s.config.LetsEncrypt.Environment == crypto.StagingEnv {
		acmeEnvStr = "staging"
		acmeEnv = crypto.StagingEnv
	}
	s.log.Infof("Using Let's Encrypt %s environment", acmeEnvStr)

	// Create certificate manager config
	cmConfig := crypto.CertificateManagerConfig{
		Email:          s.config.LetsEncrypt.Email,
		Domains:        domains,
		Environment:    acmeEnv,
		StorageDir:     storageDir,
		HTTPServer:     s.acmeHTTPServer,
		Logger:         s.log.Logger,
		DNSProvider:    s.config.LetsEncrypt.DNSProvider,
		DNSCredentials: s.config.LetsEncrypt.DNSCredentials,
	}

	// Create certificate manager
	s.log.Debug("Creating certificate manager")
	certManager, err := crypto.NewCertificateManager(cmConfig)
	if err != nil {
		s.log.Errorf("Failed to create certificate manager: %v", err)
		return fmt.Errorf("failed to create certificate manager: %w", err)
	}

	// Start certificate manager
	s.log.Info("Starting certificate manager to obtain/renew certificates...")
	if err := certManager.Start(ctx); err != nil {
		s.log.Errorf("Failed to start certificate manager: %v", err)
		return fmt.Errorf("failed to start certificate manager: %w", err)
	}

	// Store certificate manager
	s.certificateManager = certManager

	// Update TLS config with certificate manager
	s.tlsConfig = certManager.GetTLSConfig()

	s.log.Info("Certificate manager initialized successfully")

	// Log certificate status
	status := certManager.Status()
	if certs, ok := status["certificates"].(map[string]interface{}); ok {
		for domain, certInfo := range certs {
			if info, ok := certInfo.(map[string]interface{}); ok {
				if errMsg, hasError := info["error"]; hasError {
					s.log.Warnf("Certificate for %s: Error - %s", domain, errMsg)
				} else {
					issuer := info["issuer"]
					notAfter, ok := info["notAfter"].(time.Time)
					if ok {
						s.log.Infof("Certificate for %s: Issuer: %v, Valid until: %s",
							domain, issuer, notAfter.Format("2006-01-02 15:04:05"))
					} else {
						s.log.Infof("Certificate for %s: Issuer: %v", domain, issuer)
					}
				}
			}
		}
	}

	return nil
}

// Start starts the server, initializing HTTP and tunnel servers
func (s *Server) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.stopping {
		return fmt.Errorf("server is shutting down")
	}

	// Always check Let's Encrypt configuration first before loading certificates
	ctx := context.Background()
	if s.config.LetsEncrypt.Enabled {
		s.log.Info("Let's Encrypt is enabled, initializing certificate manager...")

		// We'll initially set up a temporary TLS configuration if needed,
		// which will be replaced by the Let's Encrypt certificates
		if s.tlsConfig == nil {
			tempConfig, err := crypto.LoadOrGenerateServerCertificate(s.config.TLSCert, s.config.TLSKey, s.log.Logger)
			if err != nil {
				return fmt.Errorf("failed to load temporary certificates: %w", err)
			}
			s.tlsConfig = tempConfig
			s.log.Info("Using temporary TLS configuration while initializing Let's Encrypt")
		}

		if err := s.initCertificateManager(ctx); err != nil {
			s.log.Errorf("Let's Encrypt initialization failed: %v", err)
			s.log.Warn("Falling back to standard TLS certificates")

			// Don't return an error here, just fall back to whatever certificates were loaded earlier
		} else {
			s.log.Info("Using Let's Encrypt certificates for HTTPS connections")
		}
	} else {
		s.log.Info("Let's Encrypt is not enabled, using standard TLS certificates")

		// Load regular certificates if Let's Encrypt is not enabled
		if s.tlsConfig == nil {
			var err error
			s.tlsConfig, err = crypto.LoadOrGenerateServerCertificate(s.config.TLSCert, s.config.TLSKey, s.log.Logger)
			if err != nil {
				return fmt.Errorf("failed to initialize TLS configuration: %w", err)
			}
		}
	}

	// Now proceed with HTTP server setup
	s.log.Info("Setting up HTTP servers...")

	// Set up HTTP server
	mux := http.NewServeMux()

	// API handlers
	mux.HandleFunc("/api/register", s.handleRegister)
	mux.HandleFunc("/api/tunnel", s.handleTunnel)
	mux.HandleFunc("/api/ws", s.handleWebSocket)
	mux.HandleFunc("/api/status", s.handleStatus)

	// Default handler for tunnel requests
	mux.HandleFunc("/", s.handleTunnelRequest)

	// Create HTTPS server for API
	s.httpServer = &http.Server{
		Addr:      fmt.Sprintf("%s:%d", s.config.BindAddress, s.config.Port),
		Handler:   mux,
		TLSConfig: s.tlsConfig,
	}

	// If Let's Encrypt is not enabled, create dedicated HTTP server for tunnel traffic
	if !s.config.LetsEncrypt.Enabled {
		tunnelMux := http.NewServeMux()
		tunnelMux.HandleFunc("/", s.handleTunnelRequest) // Only handle tunnel requests
		s.tunnelServer = &http.Server{
			Addr:    fmt.Sprintf("%s:80", s.config.BindAddress),
			Handler: tunnelMux,
		}
	} else {
		// When Let's Encrypt is enabled, we use the acmeHTTPServer that was created
		// in initCertificateManager to handle both ACME challenges and tunnel requests

		// Check if acmeHTTPServer exists - it should have been created during
		// initCertificateManager if Let's Encrypt was properly initialized
		if s.acmeHTTPServer == nil {
			s.log.Warn("Let's Encrypt HTTP server not initialized, creating a basic HTTP server")
			s.acmeHTTPServer = &http.Server{
				Addr:    fmt.Sprintf("%s:80", s.config.BindAddress),
				Handler: http.NewServeMux(),
			}
		}

		// Add the tunnel request handler to the ACME mux
		// Get the mux from the ACME HTTP server
		acmeMux, ok := s.acmeHTTPServer.Handler.(*http.ServeMux)
		if ok {
			// Add tunnel request handler
			acmeMux.HandleFunc("/", s.handleTunnelRequest)
		} else {
			return fmt.Errorf("failed to add tunnel handler to ACME HTTP server")
		}

		// The ACME server is already using port 80, so we don't need a separate tunnel server
		s.tunnelServer = s.acmeHTTPServer
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

	// Stop certificate manager if it exists
	if s.certificateManager != nil {
		s.log.Info("Stopping certificate manager")
		if err := s.certificateManager.Stop(); err != nil {
			s.log.Errorf("Error stopping certificate manager: %v", err)
		}
	}

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

	// Validate protocol
	if reqBody.Protocol != "http" && reqBody.Protocol != "https" && reqBody.Protocol != "tcp" {
		http.Error(w, "Unsupported protocol", http.StatusBadRequest)
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

	// Check if HTTPS is available and requested
	httpsAvailable := false

	// HTTPS is available if Let's Encrypt is configured and active
	if s.certificateManager != nil {
		httpsAvailable = true
		s.log.Debug("HTTPS is available via certificate manager")
	} else if s.config.TLSCert != "" && s.config.TLSKey != "" {
		httpsAvailable = true
		s.log.Debug("HTTPS is available via static certificates")
	}

	// Construct public URL with proper protocol
	var publicURL string
	if reqBody.Protocol == "https" {
		if httpsAvailable {
			publicURL = fmt.Sprintf("https://%s.%s", subdomain, s.config.BaseDomain)
			s.log.Infof("Created HTTPS tunnel: %s", publicURL)
		} else {
			// Fall back to HTTP if HTTPS was requested but not available
			publicURL = fmt.Sprintf("http://%s.%s", subdomain, s.config.BaseDomain)
			s.log.Warnf("HTTPS requested but certificates not available, falling back to HTTP: %s", publicURL)
			tunnel.Protocol = "http" // Update the protocol in the stored tunnel
		}
	} else {
		publicURL = fmt.Sprintf("%s://%s.%s", reqBody.Protocol, subdomain, s.config.BaseDomain)
		s.log.Infof("Created %s tunnel: %s", reqBody.Protocol, publicURL)
	}

	// Add port to URL if non-standard
	if (reqBody.Protocol == "http" && s.config.Port != 80) ||
		(reqBody.Protocol == "https" && s.config.Port != 443) {
		publicURL = fmt.Sprintf("%s:%d", publicURL, s.config.Port)
	}

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

// handleStatus returns the current status of the server
func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	// Collect status information
	status := map[string]interface{}{
		"version": "1.0.0",                         // Replace with actual version
		"uptime":  time.Since(time.Now()).String(), // Just a placeholder - real impl would use server start time
		"tunnels": len(s.tunnels.tunnels),
	}

	// Add TLS configuration information
	tlsInfo := map[string]interface{}{
		"enabled": s.tlsConfig != nil,
	}

	// Add certificate information if Let's Encrypt is enabled
	if s.config.LetsEncrypt.Enabled && s.certificateManager != nil {
		tlsInfo["provider"] = "Let's Encrypt"
		tlsInfo["certificates"] = s.certificateManager.Status()
	} else if s.config.TLSCert != "" && s.config.TLSKey != "" {
		tlsInfo["provider"] = "Custom certificate"
		tlsInfo["cert_file"] = s.config.TLSCert
	} else {
		tlsInfo["provider"] = "None"
	}

	status["tls"] = tlsInfo

	// Return status as JSON
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(status)
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
