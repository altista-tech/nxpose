// internal/tunnel/tunnel.go
// Complete implementation of the tunnel functionality

package tunnel

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/websocket"
)

// Tunnel represents an active tunnel
type Tunnel struct {
	ID         string
	Protocol   string
	LocalPort  int
	PublicURL  string
	CertData   []byte
	ServerHost string
	ServerPort int
	wsConn     *websocket.Conn
	running    bool
	mu         sync.Mutex
	stopCh     chan struct{}
	log        *logrus.Logger
	tcpTunnel  *TCPTunnel
	registered bool
}

// TunnelMessage represents a message sent over the WebSocket tunnel
type TunnelMessage struct {
	Type      string          `json:"type"`
	RequestID string          `json:"request_id,omitempty"`
	TunnelID  string          `json:"tunnel_id,omitempty"`
	Data      json.RawMessage `json:"data,omitempty"`
}

// HTTPRequest represents an HTTP request tunneled over WebSocket
type HTTPRequest struct {
	Method  string            `json:"method"`
	Path    string            `json:"path"`
	Query   string            `json:"query,omitempty"`
	Headers map[string]string `json:"headers"`
	Body    []byte            `json:"body,omitempty"`
}

// HTTPResponse represents an HTTP response tunneled over WebSocket
type HTTPResponse struct {
	StatusCode int               `json:"status_code"`
	Headers    map[string]string `json:"headers"`
	Body       []byte            `json:"body,omitempty"`
}

// TCPMessage represents a TCP data message tunneled over WebSocket
type TCPMessage struct {
	ConnectionID string `json:"connection_id"`
	Data         []byte `json:"data,omitempty"`
}

func ExposeLocalService(protocol string, port int, certData []byte, serverHost string, serverPort int) (string, string, error) {
	// Check if certificate data is provided
	if len(certData) == 0 {
		return "", "", fmt.Errorf("no certificate data available, please run 'nxpose register' first")
	}

	// Create logger for diagnostics
	log := logrus.New()
	if os.Getenv("NXPOSE_DEBUG") == "1" {
		log.SetLevel(logrus.DebugLevel)
	}

	// Create tunnel request
	clientID := uuid.New().String()
	req := struct {
		ClientID    string `json:"client_id"`
		Protocol    string `json:"protocol"`
		Port        int    `json:"port"`
		Certificate string `json:"certificate"`
	}{
		ClientID:    clientID,
		Protocol:    protocol,
		Port:        port,
		Certificate: string(certData),
	}

	// Marshal request to JSON
	reqBody, err := json.Marshal(req)
	if err != nil {
		return "", "", fmt.Errorf("failed to marshal request: %w", err)
	}

	// Build server URL for API
	serverURL := fmt.Sprintf("https://%s:%d/api/tunnel", serverHost, serverPort)

	fmt.Printf("Sending tunnel request to %s\n", serverURL)
	log.Debugf("Request body: %s", string(reqBody))

	// Create HTTP request
	httpReq, err := http.NewRequest("POST", serverURL, bytes.NewBuffer(reqBody))
	if err != nil {
		return "", "", fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("User-Agent", "nxpose-client/1.0")

	// Create HTTP client with proper TLS configuration and longer timeout
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true, // Only for development - should use proper certificates in production
			},
			DisableKeepAlives:   false,
			MaxIdleConnsPerHost: 10,
			MaxIdleConns:        100,
			IdleConnTimeout:     30 * time.Second,
		},
		Timeout: 30 * time.Second, // Increased timeout
	}

	// Add retries for better resilience
	maxRetries := 3
	var resp *http.Response
	var respBody []byte

	for attempt := 1; attempt <= maxRetries; attempt++ {
		// Send request
		resp, err = client.Do(httpReq)
		if err == nil {
			// Read response
			respBody, err = io.ReadAll(resp.Body)
			resp.Body.Close()

			if err == nil {
				break // Success, exit retry loop
			}

			log.Warnf("Failed to read response body (attempt %d/%d): %v", attempt, maxRetries, err)
		} else {
			log.Warnf("Failed to send request (attempt %d/%d): %v", attempt, maxRetries, err)
		}

		if attempt < maxRetries {
			// Wait before retrying with exponential backoff
			backoff := time.Duration(attempt*attempt) * time.Second
			log.Infof("Retrying in %v...", backoff)
			time.Sleep(backoff)

			// Create a fresh request for retry
			httpReq, _ = http.NewRequest("POST", serverURL, bytes.NewBuffer(reqBody))
			httpReq.Header.Set("Content-Type", "application/json")
			httpReq.Header.Set("User-Agent", "nxpose-client/1.0")
		}
	}

	// Check if all retries failed
	if err != nil {
		// For development: if the real server is not available, simulate the response
		if os.Getenv("NXPOSE_DEV_MODE") == "1" {
			fmt.Println("Development mode: Simulating server response")
			subdomain := generateSubdomain()
			tunnelID := uuid.New().String()
			return fmt.Sprintf("%s://%s.%s", protocol, subdomain, serverHost), tunnelID, nil
		}
		return "", "", fmt.Errorf("failed to send request to server after %d attempts: %w", maxRetries, err)
	}

	// Print raw response for debugging
	fmt.Printf("Server response (status %d): %s\n", resp.StatusCode, string(respBody))
	log.Debugf("Full response headers: %v", resp.Header)

	// Check response status
	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("server returned error: %s (status code: %d)", string(respBody), resp.StatusCode)
	}

	// Parse response
	var tunnelResp struct {
		Success   bool   `json:"success"`
		Message   string `json:"message"`
		TunnelID  string `json:"tunnel_id"`
		PublicURL string `json:"public_url"`
	}
	if err := json.Unmarshal(respBody, &tunnelResp); err != nil {
		return "", "", fmt.Errorf("failed to parse response: %w", err)
	}

	// Check success
	if !tunnelResp.Success {
		return "", "", fmt.Errorf("server returned error: %s", tunnelResp.Message)
	}

	// Validate response
	if tunnelResp.TunnelID == "" || tunnelResp.PublicURL == "" {
		return "", "", fmt.Errorf("server returned incomplete data: missing tunnel ID or public URL")
	}

	// Print additional debug info
	fmt.Printf("Tunnel created with ID: %s\n", tunnelResp.TunnelID)
	fmt.Printf("Public URL: %s\n", tunnelResp.PublicURL)

	// Ensure that HTTPS protocol request results in HTTPS URL
	if protocol == "https" && !strings.HasPrefix(tunnelResp.PublicURL, "https://") {
		originalURL := tunnelResp.PublicURL
		// Replace the protocol part
		tunnelResp.PublicURL = "https://" + strings.TrimPrefix(strings.TrimPrefix(originalURL, "http://"), "https://")
		fmt.Printf("Upgraded URL to HTTPS: %s\n", tunnelResp.PublicURL)
		log.Infof("Protocol requested was HTTPS but server returned HTTP URL. Upgraded to: %s", tunnelResp.PublicURL)
	}

	// Return both public URL and tunnel ID
	return tunnelResp.PublicURL, tunnelResp.TunnelID, nil
}

func StartTunnel(protocol string, localPort int, publicURL string, tunnelID string, certData []byte, skipLocalCheck bool) error {
	// Parse the public URL
	parsedURL, err := url.Parse(publicURL)
	if err != nil {
		return fmt.Errorf("invalid public URL: %w", err)
	}

	// Extract server host and subdomain
	hostParts := strings.Split(parsedURL.Host, ".")
	if len(hostParts) < 2 {
		return fmt.Errorf("invalid hostname in public URL")
	}

	// Subdomain is the first part
	subdomain := hostParts[0]

	// Server domain is the rest joined together
	//serverDomain := strings.Join(hostParts[1:], ".")

	serverHost := parsedURL.Hostname()

	// Extract server port from URL if present, otherwise use default port
	serverPort := 8443
	if parsedURL.Port() != "" {
		port, err := strconv.Atoi(parsedURL.Port())
		if err == nil && port > 0 {
			serverPort = port
		}
	}

	fmt.Printf("Creating tunnel with ID: %s to URL: %s\n", tunnelID, publicURL)

	// Create tunnel with server-assigned ID
	tunnel := &Tunnel{
		ID:         tunnelID, // Use the ID from the server response
		Protocol:   protocol,
		LocalPort:  localPort,
		PublicURL:  publicURL,
		CertData:   certData,
		ServerHost: serverHost,
		ServerPort: serverPort,
		stopCh:     make(chan struct{}),
		log:        logrus.New(),
	}

	// Configure logger
	tunnel.log.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})

	if os.Getenv("NXPOSE_DEBUG") == "1" {
		tunnel.log.SetLevel(logrus.DebugLevel)
	} else {
		tunnel.log.SetLevel(logrus.InfoLevel)
	}

	// Verify local service is available before starting tunnel, unless skipLocalCheck is true
	if !skipLocalCheck {
		localURL := fmt.Sprintf("http://localhost:%d", localPort)
		localClient := &http.Client{
			Timeout: 2 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,
				},
			},
		}

		// Try a HEAD request first to minimize data transfer
		req, _ := http.NewRequest("HEAD", localURL, nil)
		resp, err := localClient.Do(req)
		if err != nil {
			// Try a GET request as fallback
			req, _ = http.NewRequest("GET", localURL, nil)
			resp, err = localClient.Do(req)

			if err != nil {
				tunnel.log.Warnf("Warning: Local service at %s doesn't appear to be running: %v", localURL, err)
				fmt.Printf("Warning: Can't connect to local service at %s\n", localURL)
				fmt.Println("Make sure your local service is running before starting the tunnel.")

				// Ask user if they want to continue anyway
				fmt.Print("Continue anyway? (y/n): ")
				var response string
				fmt.Scanln(&response)
				if strings.ToLower(response) != "y" {
					return fmt.Errorf("tunnel creation cancelled - please start your local service first")
				}
			} else {
				resp.Body.Close()
			}
		} else {
			resp.Body.Close()
			tunnel.log.Infof("Local service verified at %s (status: %d)", localURL, resp.StatusCode)
		}
	} else {
		tunnel.log.Info("Skipping local service check as requested")
		fmt.Println("Skipping local service check. Make sure your service will be available when needed.")
	}

	// Start the tunnel with retry mechanism
	maxRetries := 3
	var startErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		startErr = tunnel.Start()
		if startErr == nil {
			break // Success
		}

		tunnel.log.WithError(startErr).Warnf("Failed to start tunnel (attempt %d/%d)", attempt, maxRetries)

		if attempt < maxRetries {
			// Wait before retrying with exponential backoff
			backoff := time.Duration(attempt*attempt) * time.Second
			fmt.Printf("Retrying in %v...\n", backoff)
			time.Sleep(backoff)
		}
	}

	if startErr != nil {
		return fmt.Errorf("failed to start tunnel after %d attempts: %w", maxRetries, startErr)
	}

	// Print some status info
	fmt.Printf("Started tunnel from %s://localhost:%d to %s\n",
		protocol, localPort, publicURL)
	fmt.Printf("Subdomain: %s\n", subdomain)
	fmt.Printf("Tunnel ID: %s\n", tunnelID)
	fmt.Println("\nTunnel active. Press Ctrl+C to stop.")

	// Monitor for interrupt signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	// Wait for interrupt or stop signal
	select {
	case <-sigCh:
		fmt.Println("\nShutting down tunnel...")
	case <-tunnel.stopCh:
		fmt.Println("\nTunnel stopped")
	}

	// Stop the tunnel
	tunnel.Stop()

	return nil
}

// Start initializes and starts the tunnel
func (t *Tunnel) Start() error {
	t.mu.Lock()
	if t.running {
		t.mu.Unlock()
		return fmt.Errorf("tunnel is already running")
	}

	// Mark as running
	t.running = true
	t.mu.Unlock()

	// Make sure we clean up if start fails
	success := false
	defer func() {
		if !success {
			t.mu.Lock()
			t.running = false
			t.mu.Unlock()
		}
	}()

	// Connect to WebSocket for tunnel control
	t.log.Info("Connecting to WebSocket tunnel...")

	// Use retries for connection
	var err error
	maxRetries := 3

	for attempt := 1; attempt <= maxRetries; attempt++ {
		err = t.connectWebSocket()
		if err == nil {
			break
		}

		t.log.WithError(err).Warnf("Failed to connect to WebSocket (attempt %d/%d)", attempt, maxRetries)

		if attempt < maxRetries {
			delay := time.Duration(attempt) * time.Second
			t.log.Infof("Retrying in %s...", delay)

			select {
			case <-t.stopCh:
				return fmt.Errorf("tunnel stopped while connecting")
			case <-time.After(delay):
				// Continue
			}
		}
	}

	if err != nil {
		return fmt.Errorf("failed to connect to WebSocket after %d attempts: %w", maxRetries, err)
	}

	// Register the tunnel with the server
	for attempt := 1; attempt <= maxRetries; attempt++ {
		err = t.registerTunnel()
		if err == nil {
			break
		}

		t.log.WithError(err).Warnf("Failed to register tunnel (attempt %d/%d)", attempt, maxRetries)

		if attempt < maxRetries {
			delay := time.Duration(attempt) * time.Second
			t.log.Infof("Retrying in %s...", delay)

			select {
			case <-t.stopCh:
				return fmt.Errorf("tunnel stopped while registering")
			case <-time.After(delay):
				// Continue
			}
		}
	}

	if err != nil {
		return fmt.Errorf("failed to register tunnel after %d attempts: %w", maxRetries, err)
	}

	// Start local proxy for HTTP(S) tunnels
	if t.Protocol == "http" || t.Protocol == "https" {
		if err := t.startLocalProxy(); err != nil {
			return fmt.Errorf("failed to start local proxy: %w", err)
		}
	} else if t.Protocol == "tcp" {
		if err := t.startTCPProxy(); err != nil {
			return fmt.Errorf("failed to start TCP proxy: %w", err)
		}
	} else {
		return fmt.Errorf("unsupported protocol: %s", t.Protocol)
	}

	// Start successful
	success = true
	t.log.Infof("Tunnel started successfully. Public URL: %s", t.PublicURL)
	return nil
}

// Stop closes the tunnel and all associated connections
func (t *Tunnel) Stop() error {
	t.mu.Lock()
	if !t.running {
		t.mu.Unlock()
		return nil // Already stopped
	}

	// Prevent multiple stops
	if t.stopCh == nil {
		t.mu.Unlock()
		return fmt.Errorf("stop channel is nil")
	}

	// Check if already stopping
	select {
	case <-t.stopCh:
		// Already closed
		t.mu.Unlock()
		return nil
	default:
		// Continue with stop
	}

	t.log.Info("Stopping tunnel...")

	// Signal other goroutines to stop
	close(t.stopCh)

	// Close WebSocket connection if active
	if t.wsConn != nil {
		t.log.Debug("Closing WebSocket connection")
		err := t.wsConn.Close()
		if err != nil {
			t.log.WithError(err).Warn("Error closing WebSocket connection")
		}
		t.wsConn = nil
	}

	// Stop TCP tunnel if active
	if t.tcpTunnel != nil {
		t.log.Debug("Stopping TCP tunnel")
		t.tcpTunnel.Stop()
		t.tcpTunnel = nil
	}

	t.running = false
	t.registered = false
	t.mu.Unlock()

	t.log.Info("Tunnel stopped")
	return nil
}

// connectWebSocket establishes a WebSocket connection to the server
func (t *Tunnel) connectWebSocket() error {
	// Build WebSocket URL including both client_id and tunnel_id parameters
	wsURL := fmt.Sprintf("wss://%s:%d/api/ws?client_id=%s&tunnel_id=%s",
		t.ServerHost, t.ServerPort, t.ID, t.ID)

	// Log the connection attempt with detailed information
	t.log.WithFields(logrus.Fields{
		"wsURL":      wsURL,
		"tunnelID":   t.ID,
		"serverHost": t.ServerHost,
		"serverPort": t.ServerPort,
	}).Info("Attempting WebSocket connection")

	// Create WebSocket config with origin
	origin := fmt.Sprintf("https://%s:%d", t.ServerHost, t.ServerPort)
	config, err := websocket.NewConfig(wsURL, origin)
	if err != nil {
		return fmt.Errorf("failed to create WebSocket config: %w", err)
	}

	// Set proper headers
	config.Header.Add("User-Agent", "nxpose-client/1.0")
	config.Header.Add("X-Nxpose-Client-ID", t.ID)
	config.Header.Add("X-Nxpose-Tunnel-ID", t.ID)
	config.Header.Add("Connection", "upgrade")
	config.Header.Add("Upgrade", "websocket")

	// Add version and protocol headers
	config.Header.Add("X-Nxpose-Version", "1.0.0")
	config.Header.Add("X-Nxpose-Protocol", t.Protocol)

	// Configure TLS for secure connections
	config.TlsConfig = &tls.Config{
		InsecureSkipVerify: true, // Allow self-signed certs for development
		MinVersion:         tls.VersionTLS12,
	}

	// Try to connect with the main URL
	wsConn, err := websocket.DialConfig(config)

	// If connection fails, try alternative port (443)
	if err != nil {
		t.log.WithError(err).Warn("Failed to connect to WebSocket on primary port, trying standard HTTPS port (443)")

		// Build alternative WebSocket URL using standard HTTPS port
		altWsURL := fmt.Sprintf("wss://%s/api/ws?client_id=%s&tunnel_id=%s",
			t.ServerHost, t.ID, t.ID)
		altOrigin := fmt.Sprintf("https://%s", t.ServerHost)

		altConfig, configErr := websocket.NewConfig(altWsURL, altOrigin)
		if configErr != nil {
			return fmt.Errorf("failed to create alternative WebSocket config: %w", configErr)
		}

		// Copy headers to alternative config
		for name, values := range config.Header {
			for _, value := range values {
				altConfig.Header.Add(name, value)
			}
		}
		altConfig.TlsConfig = config.TlsConfig

		// Try connecting with alternative URL
		wsConn, err = websocket.DialConfig(altConfig)
		if err != nil {
			// Try one more time with the main domain instead of subdomain
			if dots := strings.Count(t.ServerHost, "."); dots >= 2 {
				mainDomainParts := strings.SplitN(t.ServerHost, ".", 2)
				mainDomain := mainDomainParts[1]

				t.log.WithField("mainDomain", mainDomain).Info("Trying with main domain instead of subdomain")

				finalWsURL := fmt.Sprintf("wss://%s/api/ws?client_id=%s&tunnel_id=%s",
					mainDomain, t.ID, t.ID)
				finalOrigin := fmt.Sprintf("https://%s", mainDomain)

				finalConfig, finalConfigErr := websocket.NewConfig(finalWsURL, finalOrigin)
				if finalConfigErr != nil {
					return fmt.Errorf("failed to create final WebSocket config: %w", finalConfigErr)
				}

				// Copy headers to final config
				for name, values := range config.Header {
					for _, value := range values {
						finalConfig.Header.Add(name, value)
					}
				}
				finalConfig.TlsConfig = config.TlsConfig

				// Try connecting with final URL
				wsConn, err = websocket.DialConfig(finalConfig)
				if err != nil {
					return fmt.Errorf("failed to dial WebSocket (all attempts failed): %w", err)
				}

				t.log.Info("WebSocket connection established using main domain")
			} else {
				return fmt.Errorf("failed to dial WebSocket (both primary and alternative ports): %w", err)
			}
		} else {
			t.log.Info("WebSocket connection established using standard HTTPS port")
		}
	} else {
		t.log.Info("WebSocket connection established using configured port")
	}

	// Store the WebSocket connection
	t.mu.Lock()
	t.wsConn = wsConn
	t.mu.Unlock()

	// Start a heartbeat routine
	go t.startHeartbeat()

	// Start error handling routine
	go t.handleWebSocketErrors()

	return nil
}

// Add this new function to implement heartbeat
func (t *Tunnel) startHeartbeat() {
	ticker := time.NewTicker(25 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Send ping message
			ping := TunnelMessage{
				Type:      "ping",
				RequestID: uuid.New().String(),
				TunnelID:  t.ID,
				Data:      json.RawMessage([]byte(`{"timestamp":"` + time.Now().Format(time.RFC3339) + `"}`)),
			}

			t.mu.Lock()
			if t.wsConn == nil {
				t.mu.Unlock()
				return
			}

			err := websocket.JSON.Send(t.wsConn, ping)
			t.mu.Unlock()

			if err != nil {
				t.log.WithError(err).Warn("Failed to send heartbeat ping")
				return
			}

			t.log.Debug("Sent heartbeat ping")
		case <-t.stopCh:
			return
		}
	}
}

func (t *Tunnel) registerTunnel() error {
	// If already registered, don't attempt again
	if t.registered {
		t.log.Debug("Tunnel already registered, skipping registration")
		return nil
	}

	// Create a channel to handle message timeouts
	msgChan := make(chan TunnelMessage, 1)
	errChan := make(chan error, 1)

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Start a goroutine to read the welcome message
	go func() {
		var welcomeMsg TunnelMessage
		t.mu.Lock()
		conn := t.wsConn
		t.mu.Unlock()

		if conn == nil {
			errChan <- fmt.Errorf("websocket connection is nil")
			return
		}

		if err := websocket.JSON.Receive(conn, &welcomeMsg); err != nil {
			errChan <- fmt.Errorf("failed to receive welcome message: %w", err)
			return
		}

		select {
		case msgChan <- welcomeMsg:
			// Message sent successfully
		case <-ctx.Done():
			// Context cancelled, do nothing
		}
	}()

	// Wait for the welcome message with timeout
	var welcomeMsg TunnelMessage
	select {
	case err := <-errChan:
		return err
	case welcomeMsg = <-msgChan:
		// Process welcome message
	case <-ctx.Done():
		return fmt.Errorf("timeout waiting for welcome message")
	}

	t.log.Debugf("Received welcome message: %+v", welcomeMsg)

	if welcomeMsg.Type != "welcome" {
		return fmt.Errorf("unexpected initial message type: %s", welcomeMsg.Type)
	}

	// Create registration message
	// Important: Include both client ID and tunnel ID
	regData := struct {
		TunnelID string `json:"tunnel_id"`
		ClientID string `json:"client_id,omitempty"` // Added ClientID
	}{
		TunnelID: t.ID,
		ClientID: t.ID, // For now, use tunnel ID as client ID
	}

	// Marshal the registration data
	regDataJSON, err := json.Marshal(regData)
	if err != nil {
		return fmt.Errorf("failed to marshal registration data: %w", err)
	}

	// Create the registration message
	regMsg := TunnelMessage{
		Type:      "register_tunnel",
		RequestID: uuid.New().String(),
		TunnelID:  t.ID,
		Data:      regDataJSON,
	}

	// Send registration message
	if err := t.sendMessage(regMsg); err != nil {
		return fmt.Errorf("failed to send registration message: %w", err)
	}

	// Wait for registration response
	respChan := make(chan TunnelMessage, 1)
	errChan = make(chan error, 1)

	// Start a goroutine to read the registration response
	go func() {
		var respMsg TunnelMessage
		t.mu.Lock()
		conn := t.wsConn
		t.mu.Unlock()

		if conn == nil {
			errChan <- fmt.Errorf("websocket connection is nil")
			return
		}

		if err := websocket.JSON.Receive(conn, &respMsg); err != nil {
			errChan <- fmt.Errorf("failed to receive registration response: %w", err)
			return
		}

		select {
		case respChan <- respMsg:
			// Message sent successfully
		case <-ctx.Done():
			// Context cancelled, do nothing
		}
	}()

	// Wait for the registration response with timeout
	var respMsg TunnelMessage
	select {
	case err := <-errChan:
		return err
	case respMsg = <-respChan:
		// Process registration response
	case <-ctx.Done():
		return fmt.Errorf("timeout waiting for registration response")
	}

	// Check if the response indicates success
	if respMsg.Type != "registration_success" && respMsg.Type != "tunnel_registered" {
		t.log.WithField("response_type", respMsg.Type).Warn("Unexpected registration response type")
		return fmt.Errorf("unexpected registration response: %s", respMsg.Type)
	}

	// Mark as registered
	t.registered = true
	t.log.Info("Tunnel registered successfully with server")

	// Start handling messages
	go t.handleMessages()

	return nil
}

// handleMessages processes incoming WebSocket messages
func (t *Tunnel) handleMessages() {
	// Add panic recovery
	defer func() {
		if r := recover(); r != nil {
			t.log.Errorf("Recovered from panic in message handler: %v", r)

			// Safe cleanup
			t.mu.Lock()
			if t.wsConn != nil {
				t.wsConn.Close()
				t.wsConn = nil
			}
			t.mu.Unlock()

			// Try to reconnect after panic
			t.log.Info("Attempting to reconnect after panic...")
			go func() {
				// Wait a bit before reconnecting
				time.Sleep(2 * time.Second)

				// Only attempt reconnect if we're still running
				if t.running {
					if err := t.connectWebSocket(); err != nil {
						t.log.WithError(err).Error("Failed to reconnect after panic")
						return
					}

					if err := t.registerTunnel(); err != nil {
						t.log.WithError(err).Error("Failed to register tunnel after panic recovery")
						return
					}

					// Restart message handling
					go t.handleMessages()
				}
			}()
		}
	}()

	// Process messages until connection is closed
	for {
		// Check if we need to stop
		select {
		case <-t.stopCh:
			return
		default:
			// Continue processing
		}

		// Check if connection exists
		t.mu.Lock()
		if t.wsConn == nil {
			t.mu.Unlock()
			return
		}
		t.mu.Unlock()

		// Read message with timeout
		var message TunnelMessage

		// Set read deadline to detect connection issues
		t.mu.Lock()
		if t.wsConn != nil {
			t.wsConn.SetReadDeadline(time.Now().Add(2 * time.Minute))
		} else {
			t.mu.Unlock()
			return
		}
		t.mu.Unlock()

		t.mu.Lock()
		conn := t.wsConn
		t.mu.Unlock()

		if conn == nil {
			return
		}

		err := websocket.JSON.Receive(conn, &message)

		// Reset read deadline
		t.mu.Lock()
		if t.wsConn != nil {
			t.wsConn.SetReadDeadline(time.Time{}) // Reset deadline
		}
		t.mu.Unlock()

		if err != nil {
			if err == io.EOF {
				t.log.Info("WebSocket connection closed by server")
			} else {
				t.log.WithError(err).Error("Error reading from WebSocket")
			}

			// Clean up
			t.mu.Lock()
			if t.wsConn != nil {
				t.wsConn.Close()
				t.wsConn = nil
			}
			t.mu.Unlock()

			// Try to reconnect if we're still running
			if t.running {
				t.log.Info("Attempting to reconnect...")
				go func() {
					if err := t.connectWebSocket(); err != nil {
						t.log.WithError(err).Error("Failed to reconnect")
						return
					}

					if err := t.registerTunnel(); err != nil {
						t.log.WithError(err).Error("Failed to register tunnel after reconnection")
						return
					}

					// Restart message handling
					go t.handleMessages()
				}()
			}
			return
		}

		// Process the message based on its type
		t.log.WithFields(logrus.Fields{
			"message_type": message.Type,
			"request_id":   message.RequestID,
		}).Debug("Received WebSocket message")

		switch message.Type {
		case "ping":
			// Send pong response
			pong := TunnelMessage{
				Type:      "pong",
				RequestID: uuid.New().String(),
				TunnelID:  t.ID,
				Data:      json.RawMessage([]byte(`{"timestamp":"` + time.Now().Format(time.RFC3339) + `"}`)),
			}

			if err := t.sendMessage(pong); err != nil {
				t.log.WithError(err).Warn("Failed to send pong response")
			} else {
				t.log.Debug("Sent pong response")
			}
		case "http_request":
			// Handle HTTP request
			t.handleHTTPRequest(message)
		case "tcp_data":
			// Handle TCP data
			t.handleTCPData(message)
		case "error":
			// Log error message
			t.log.WithField("request_id", message.RequestID).Warn("Received error message from server")
		default:
			t.log.WithField("message_type", message.Type).Debug("Unhandled message type")
		}
	}
}

// handleHTTPRequest processes an HTTP request from the server
func (t *Tunnel) handleHTTPRequest(message TunnelMessage) {
	// Parse HTTP request
	var httpReq HTTPRequest
	if err := json.Unmarshal(message.Data, &httpReq); err != nil {
		t.log.Errorf("Failed to parse HTTP request: %v", err)
		t.sendErrorResponse(message.RequestID, "Failed to parse request")
		return
	}

	// Determine which protocol to use for local connection
	// Check if the original request was HTTPS by examining the X-Forwarded-Proto header
	isSecureRequest := false
	for key, value := range httpReq.Headers {
		if strings.ToLower(key) == "x-forwarded-proto" && value == "https" {
			isSecureRequest = true
			break
		}
	}

	// Create URL for local service
	// Note: We always use HTTP for local connection, regardless of the external protocol
	// This is because we're connecting to a local service which typically doesn't use HTTPS
	localURL := fmt.Sprintf("http://localhost:%d%s", t.LocalPort, httpReq.Path)
	if httpReq.Query != "" {
		localURL += "?" + httpReq.Query
	}

	// Log the request with the original protocol
	t.log.Debugf("Forwarding %s request to local service: %s %s",
		map[bool]string{true: "HTTPS", false: "HTTP"}[isSecureRequest],
		httpReq.Method, localURL)

	// Create request
	req, err := http.NewRequest(httpReq.Method, localURL, bytes.NewReader(httpReq.Body))
	if err != nil {
		t.log.Errorf("Failed to create HTTP request: %v", err)
		t.sendErrorResponse(message.RequestID, "Failed to create request")
		return
	}

	// Add headers
	for key, value := range httpReq.Headers {
		req.Header.Set(key, value)
	}

	// Add original protocol info in headers if not already present
	if isSecureRequest && req.Header.Get("X-Forwarded-Proto") == "" {
		req.Header.Set("X-Forwarded-Proto", "https")
	}

	// Forward request to local service
	client := &http.Client{
		Timeout: 30 * time.Second,
		// Skip TLS verification for local connections if needed
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		t.log.Errorf("Failed to forward request to local service: %v", err)
		t.sendErrorResponse(message.RequestID, "Failed to reach local service")
		return
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.log.Errorf("Failed to read response body: %v", err)
		t.sendErrorResponse(message.RequestID, "Failed to read response")
		return
	}

	// Create HTTP response
	headers := make(map[string]string)
	for key, values := range resp.Header {
		if len(values) > 0 {
			headers[key] = values[0]
		}
	}

	httpResp := HTTPResponse{
		StatusCode: resp.StatusCode,
		Headers:    headers,
		Body:       body,
	}

	// Marshal response
	respData, err := json.Marshal(httpResp)
	if err != nil {
		t.log.Errorf("Failed to marshal HTTP response: %v", err)
		t.sendErrorResponse(message.RequestID, "Failed to marshal response")
		return
	}

	// Send response
	respMsg := TunnelMessage{
		Type:      "http_response",
		RequestID: message.RequestID,
		TunnelID:  t.ID,
		Data:      respData,
	}

	if err := websocket.JSON.Send(t.wsConn, respMsg); err != nil {
		t.log.Errorf("Failed to send HTTP response: %v", err)
	}

	// Use additional logging for HTTPS requests
	t.log.Infof("Forwarded %s request: %s %s, status: %d, body size: %d",
		map[bool]string{true: "HTTPS", false: "HTTP"}[isSecureRequest],
		httpReq.Method, httpReq.Path, resp.StatusCode, len(body))
}

// handleTCPData processes TCP data from the server
func (t *Tunnel) handleTCPData(message TunnelMessage) {
	// Parse TCP data message
	var tcpMsg TCPMessage
	if err := json.Unmarshal(message.Data, &tcpMsg); err != nil {
		t.log.Errorf("Failed to parse TCP data: %v", err)
		return
	}

	// Forward data to the appropriate TCP connection
	// In a real implementation, you would have a map of TCP connections
	// and forward the data to the correct one
	t.log.Debugf("Received TCP data for connection %s (%d bytes)",
		tcpMsg.ConnectionID, len(tcpMsg.Data))
}

// sendErrorResponse sends an error response for a request
func (t *Tunnel) sendErrorResponse(requestID, message string) {
	// Create error response
	errorResp := HTTPResponse{
		StatusCode: http.StatusInternalServerError,
		Headers:    map[string]string{"Content-Type": "text/plain"},
		Body:       []byte(message),
	}

	// Marshal response
	respData, err := json.Marshal(errorResp)
	if err != nil {
		t.log.Errorf("Failed to marshal error response: %v", err)
		return
	}

	// Send response
	respMsg := TunnelMessage{
		Type:      "http_response",
		RequestID: requestID,
		TunnelID:  t.ID,
		Data:      respData,
	}

	if err := websocket.JSON.Send(t.wsConn, respMsg); err != nil {
		t.log.Errorf("Failed to send error response: %v", err)
	}
}

// startLocalProxy starts a local HTTP server for proxying requests
// This is useful for debugging but not required for the main functionality
func (t *Tunnel) startLocalProxy() error {
	// Only start the proxy in debug mode
	if os.Getenv("NXPOSE_DEBUG") != "1" {
		return nil
	}

	// Create HTTP server
	proxyPort := t.LocalPort + 1000
	server := &http.Server{
		Addr: fmt.Sprintf("localhost:%d", proxyPort),
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Forward request to local service
			localURL := fmt.Sprintf("http://localhost:%d%s", t.LocalPort, r.URL.Path)
			if r.URL.RawQuery != "" {
				localURL += "?" + r.URL.RawQuery
			}

			// Create request
			req, err := http.NewRequest(r.Method, localURL, r.Body)
			if err != nil {
				http.Error(w, fmt.Sprintf("Failed to create HTTP request: %v", err), http.StatusInternalServerError)
				return
			}

			// Copy headers
			for key, values := range r.Header {
				for _, value := range values {
					req.Header.Add(key, value)
				}
			}

			// Forward request
			client := &http.Client{}
			resp, err := client.Do(req)
			if err != nil {
				http.Error(w, fmt.Sprintf("Failed to forward request: %v", err), http.StatusInternalServerError)
				return
			}
			defer resp.Body.Close()

			// Copy response headers
			for key, values := range resp.Header {
				for _, value := range values {
					w.Header().Add(key, value)
				}
			}

			// Set status code
			w.WriteHeader(resp.StatusCode)

			// Copy response body
			io.Copy(w, resp.Body)
		}),
	}

	// Start server
	go func() {
		t.log.Infof("Debug proxy listening on http://%s", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			t.log.Errorf("Debug proxy server failed: %v", err)
		}
	}()

	// Stop server when tunnel stops
	go func() {
		<-t.stopCh
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		server.Shutdown(ctx)
	}()

	return nil
}

// startTCPProxy starts a TCP proxy for TCP tunneling
func (t *Tunnel) startTCPProxy() error {
	if t.Protocol != "tcp" {
		return nil
	}

	// Create the TCP tunnel
	tcpTunnel := NewTCPTunnel(t.ID, t.LocalPort, "", t, t.log)

	// Store TCP tunnel in the tunnel struct
	t.mu.Lock()
	t.tcpTunnel = tcpTunnel
	t.mu.Unlock()

	// Start the TCP tunnel
	if err := tcpTunnel.Start(); err != nil {
		t.log.Errorf("Failed to start TCP tunnel: %v", err)
		return err
	}

	t.log.Infof("TCP proxy started on localhost:%d", t.LocalPort)
	return nil
}

// sendMessage sends a message over the WebSocket connection with proper locking
func (t *Tunnel) sendMessage(message TunnelMessage) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.wsConn == nil {
		return fmt.Errorf("websocket connection is nil")
	}

	// Set write deadline to detect potential timeouts
	t.wsConn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	defer t.wsConn.SetWriteDeadline(time.Time{}) // Reset deadline

	// Use context with timeout for sending
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Create channels for result and error
	errCh := make(chan error, 1)
	doneCh := make(chan struct{}, 1)

	// Use a goroutine for sending to catch any potential panic
	go func() {
		defer func() {
			if r := recover(); r != nil {
				errCh <- fmt.Errorf("panic while sending WebSocket message: %v", r)
			}
		}()

		// Send the message
		err := websocket.JSON.Send(t.wsConn, message)
		if err != nil {
			errCh <- err
			return
		}

		// Signal success
		doneCh <- struct{}{}
	}()

	// Wait for result or timeout
	select {
	case err := <-errCh:
		return fmt.Errorf("failed to send WebSocket message: %w", err)
	case <-doneCh:
		// Message sent successfully
		return nil
	case <-ctx.Done():
		return fmt.Errorf("timeout sending WebSocket message")
	}
}

// generateSubdomain creates a random subdomain for the tunnel
func generateSubdomain() string {
	// Use UUID to generate a unique identifier
	id := uuid.New().String()

	// Take just the first part of the UUID to make it shorter
	shortID := id[0:8]

	return shortID
}

// handleWebSocketErrors monitors the WebSocket connection for errors and
// attempts to recover if possible
func (t *Tunnel) handleWebSocketErrors() {
	maxRetries := 5
	retryCount := 0
	baseDelay := 2 * time.Second
	maxDelay := 1 * time.Minute

	for {
		// Check if we should exit this goroutine
		select {
		case <-t.stopCh:
			return
		default:
			// Continue monitoring
		}

		// Skip if connection is nil
		t.mu.Lock()
		if t.wsConn == nil {
			t.mu.Unlock()
			return
		}
		t.mu.Unlock()

		// Create a message to test connection health
		testMessage := TunnelMessage{
			Type:      "ping",
			RequestID: uuid.New().String(),
			TunnelID:  t.ID,
		}

		// Send a test message periodically (every 45 seconds)
		time.Sleep(45 * time.Second)

		t.mu.Lock()
		if t.wsConn == nil {
			t.mu.Unlock()
			return
		}

		// Try to send a test message
		err := websocket.JSON.Send(t.wsConn, testMessage)
		t.mu.Unlock()

		// If there's an error, attempt to reconnect
		if err != nil {
			if retryCount >= maxRetries {
				t.log.WithError(err).Error("WebSocket connection failed and max retries reached")

				// Signal that we need to reset the tunnel
				if t.running {
					go t.Stop() // This will close the connection and stopCh, ending this goroutine
				}
				return
			}

			// Calculate backoff delay with exponential increase and jitter
			delay := baseDelay * time.Duration(1<<uint(retryCount))
			if delay > maxDelay {
				delay = maxDelay
			}
			jitter := time.Duration(rand.Int63n(int64(delay) / 4))
			delay = delay + jitter

			t.log.WithFields(logrus.Fields{
				"retry":    retryCount + 1,
				"delay":    delay.String(),
				"maxTries": maxRetries,
			}).Warn("WebSocket error detected, attempting to reconnect")

			// Wait before reconnecting
			select {
			case <-time.After(delay):
				// Continue with reconnect
			case <-t.stopCh:
				return
			}

			// Attempt to reconnect
			t.mu.Lock()
			// Close existing connection
			if t.wsConn != nil {
				t.wsConn.Close()
				t.wsConn = nil
			}
			t.mu.Unlock()

			// Reconnect
			if err := t.connectWebSocket(); err != nil {
				t.log.WithError(err).Error("Failed to reconnect WebSocket")
				retryCount++
			} else {
				t.log.Info("Successfully reconnected WebSocket")
				retryCount = 0 // Reset retry counter on successful reconnection

				// Re-register the tunnel
				if err := t.registerTunnel(); err != nil {
					t.log.WithError(err).Error("Failed to re-register tunnel after reconnection")
					retryCount++ // Count this as a retry failure
				}
			}
		} else {
			// If message was sent successfully, reset retry counter
			retryCount = 0
		}
	}
}
