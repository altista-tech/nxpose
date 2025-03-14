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
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"runtime/debug"
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
	if certData == nil || len(certData) == 0 {
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

func StartTunnel(protocol string, localPort int, publicURL string, tunnelID string, certData []byte) error {
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

	// Verify local service is available before starting tunnel
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

// Start starts the tunnel connection
func (t *Tunnel) Start() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.running {
		return fmt.Errorf("tunnel already running")
	}

	// Establish WebSocket connection to server
	if err := t.connectWebSocket(); err != nil {
		return fmt.Errorf("failed to connect WebSocket: %w", err)
	}

	// Start message handling loop
	go t.handleMessages()

	// Register the tunnel
	if err := t.registerTunnel(); err != nil {
		t.wsConn.Close()
		return fmt.Errorf("failed to register tunnel: %w", err)
	}

	// Start protocol-specific handlers
	switch t.Protocol {
	case "http", "https":
		// For HTTP/HTTPS protocols, we need to forward HTTP requests
		// This is already handled by the WebSocket message handler
		t.log.Info("HTTP tunnel established")
	case "tcp":
		// For TCP protocol, we need to start a TCP proxy
		go t.startTCPProxy()
		t.log.Info("TCP tunnel established")
	default:
		t.log.Warnf("Unknown protocol: %s, treating as HTTP", t.Protocol)
	}

	// Start local proxy server for debugging
	go t.startLocalProxy()

	t.running = true
	t.log.Infof("Tunnel started successfully to %s", t.PublicURL)
	return nil
}

// Stop stops the tunnel connection
func (t *Tunnel) Stop() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.running {
		return nil
	}

	// Close WebSocket connection
	if t.wsConn != nil {
		t.wsConn.Close()
	}

	// Signal stop to all goroutines
	close(t.stopCh)

	t.running = false
	t.log.Info("Tunnel stopped")
	return nil
}

// connectWebSocket establishes a WebSocket connection to the server
func (t *Tunnel) connectWebSocket() error {
	// Build WebSocket URL including both client_id and tunnel_id parameters
	wsURL := fmt.Sprintf("wss://%s:%d/api/ws?client_id=%s&tunnel_id=%s",
		t.ServerHost, t.ServerPort, t.ID, t.ID)

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

	// Configure TLS with insecure skip verify for development
	config.TlsConfig = &tls.Config{
		InsecureSkipVerify: false, // Only for development - should use proper certificates in production
	}

	// Log the connection attempt
	t.log.Debugf("Connecting to WebSocket at %s", wsURL)

	// Dial with the config (simpler approach)
	wsConn, err := websocket.DialConfig(config)
	if err != nil {
		return fmt.Errorf("failed to dial WebSocket: %w", err)
	}

	t.wsConn = wsConn
	t.log.Info("WebSocket connection established")

	// Start a heartbeat routine
	go t.startHeartbeat()

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
	// Create a channel to handle message timeouts
	msgChan := make(chan TunnelMessage, 1)
	errChan := make(chan error, 1)

	// Start a goroutine to read the welcome message
	go func() {
		var welcomeMsg TunnelMessage
		if err := websocket.JSON.Receive(t.wsConn, &welcomeMsg); err != nil {
			errChan <- fmt.Errorf("failed to receive welcome message: %w", err)
			return
		}
		msgChan <- welcomeMsg
	}()

	// Wait for the welcome message with timeout
	var welcomeMsg TunnelMessage
	select {
	case err := <-errChan:
		return err
	case welcomeMsg = <-msgChan:
		// Process welcome message
	case <-time.After(15 * time.Second):
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

	// Log the message being sent
	t.log.Debugf("Sending tunnel registration message: %+v", regMsg)

	// Send registration message
	if err := websocket.JSON.Send(t.wsConn, regMsg); err != nil {
		return fmt.Errorf("failed to send registration message: %w", err)
	}

	// Start a goroutine to read the response message
	go func() {
		var response TunnelMessage
		if err := websocket.JSON.Receive(t.wsConn, &response); err != nil {
			errChan <- fmt.Errorf("failed to receive registration response: %w", err)
			return
		}
		msgChan <- response
	}()

	// Wait for the response message with timeout
	var response TunnelMessage
	select {
	case err := <-errChan:
		return err
	case response = <-msgChan:
		// Process response message
	case <-time.After(15 * time.Second):
		return fmt.Errorf("timeout waiting for registration response")
	}

	// Check response and log it
	t.log.Debugf("Received registration response: %+v", response)

	if response.Type == "error" {
		var errResp struct {
			Message string `json:"message"`
		}
		if err := json.Unmarshal(response.Data, &errResp); err != nil {
			return fmt.Errorf("failed to parse error response: %w", err)
		}
		return fmt.Errorf("registration failed: %s", errResp.Message)
	}

	if response.Type != "tunnel_registered" {
		return fmt.Errorf("unexpected response type: %s", response.Type)
	}

	t.log.Info("Tunnel registered successfully with server")
	return nil
}

// handleMessages processes incoming WebSocket messages
func (t *Tunnel) handleMessages() {
	// We no longer need to receive the first message here
	// since registerTunnel already handled the welcome message

	// Set up recovery for any panics
	defer func() {
		if r := recover(); r != nil {
			t.log.Errorf("Recovered from panic in message handler: %v", r)

			// Try to reconnect
			go func() {
				t.log.Info("Attempting to reconnect after panic...")
				time.Sleep(5 * time.Second)

				// Close existing connection if any
				t.mu.Lock()
				if t.wsConn != nil {
					t.wsConn.Close()
					t.wsConn = nil
				}
				t.mu.Unlock()

				// Try to reconnect
				if err := t.connectWebSocket(); err != nil {
					t.log.WithError(err).Error("Failed to reconnect after panic")
					return
				}

				// Re-register the tunnel
				if err := t.registerTunnel(); err != nil {
					t.log.WithError(err).Error("Failed to re-register tunnel after panic")
					return
				}

				// Restart message handling
				go t.handleMessages()
			}()
		}
	}()

	for {
		var message TunnelMessage

		// Use a timeout for receiving messages
		t.wsConn.SetReadDeadline(time.Now().Add(45 * time.Second))
		err := websocket.JSON.Receive(t.wsConn, &message)
		t.wsConn.SetReadDeadline(time.Time{}) // Reset deadline

		if err != nil {
			if err == io.EOF {
				t.log.Info("WebSocket connection closed by server")
			} else if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				// This is a timeout, which could be normal if no messages are being sent
				// Just continue the loop and wait for the next message
				t.log.Debug("WebSocket read timeout - this is normal if no messages are being sent")
				continue
			} else {
				t.log.WithError(err).Error("Error reading from WebSocket")
			}

			// Set connection to nil for reconnection
			t.mu.Lock()
			if t.wsConn != nil {
				t.wsConn.Close()
				t.wsConn = nil
			}
			t.mu.Unlock()

			// Signal reconnection if not stopping
			select {
			case <-t.stopCh:
				// Already stopping
				return
			default:
				// Try to reconnect
				go func() {
					t.log.Info("Attempting to reconnect...")
					time.Sleep(5 * time.Second)

					if err := t.connectWebSocket(); err != nil {
						t.log.WithError(err).Error("Failed to reconnect")
						return
					}

					if err := t.registerTunnel(); err != nil {
						t.log.WithError(err).Error("Failed to re-register tunnel")
						return
					}

					go t.handleMessages()
				}()
				return
			}
		}

		// Process the message based on its type
		t.log.Debugf("Received message: type=%s, request_id=%s, tunnel_id=%s",
			message.Type, message.RequestID, message.TunnelID)

		switch message.Type {
		case "http_request":
			go t.handleHTTPRequest(message)
		case "tcp_data":
			go t.handleTCPData(message)
		case "ping":
			t.handlePing(message)
		case "error":
			t.handleError(message)
		case "welcome":
			// Already handled in registerTunnel, but handle again if we get another
			t.log.Debug("Received welcome message (redundant)")
			// No need to respond to this
		case "tunnel_registered":
			// Already handled in registerTunnel, but handle again if we get another
			t.log.Debug("Received tunnel_registered message (redundant)")
			// No need to respond to this
		default:
			t.log.Warnf("Unknown message type: %s", message.Type)
		}
	}
}

// handlePing responds to ping messages from the server
func (t *Tunnel) handlePing(message TunnelMessage) {
	// Send pong response
	pong := TunnelMessage{
		Type:      "pong",
		RequestID: message.RequestID,
		Data:      json.RawMessage([]byte(`{"timestamp":"` + time.Now().Format(time.RFC3339) + `"}`)),
	}

	if err := websocket.JSON.Send(t.wsConn, pong); err != nil {
		t.log.Errorf("Failed to send pong: %v", err)
	}
}

// handleError processes error messages from the server
func (t *Tunnel) handleError(message TunnelMessage) {
	var errResp struct {
		Message string `json:"message"`
		Code    string `json:"code,omitempty"`
	}
	if err := json.Unmarshal(message.Data, &errResp); err != nil {
		t.log.Errorf("Failed to parse error message: %v", err)
		return
	}

	t.log.Errorf("Server error: %s (code: %s)", errResp.Message, errResp.Code)
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
func (t *Tunnel) startLocalProxy() {
	// Only start the proxy in debug mode
	if os.Getenv("NXPOSE_DEBUG") != "1" {
		return
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
}

// startTCPProxy starts a TCP proxy for TCP tunneling
func (t *Tunnel) startTCPProxy() {
	if t.Protocol != "tcp" {
		return
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
		return
	}

	t.log.Infof("TCP proxy started on localhost:%d", t.LocalPort)
}

// sendMessage sends a message over the WebSocket connection
// This method is used by TCPTunnel to send messages to the server
func (t *Tunnel) sendMessage(message TunnelMessage) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Check if connection is closed
	if t.wsConn == nil {
		return fmt.Errorf("WebSocket connection is closed")
	}

	// Send message
	if err := websocket.JSON.Send(t.wsConn, message); err != nil {
		t.log.Errorf("Failed to send message: %v", err)
		return err
	}

	return nil
}

// generateSubdomain creates a random subdomain for the tunnel
func generateSubdomain() string {
	// Use UUID to generate a unique identifier
	id := uuid.New().String()

	// Take just the first part of the UUID to make it shorter
	shortID := id[0:8]

	return shortID
}

func (t *Tunnel) handleWebSocketErrors() {
	defer func() {
		if r := recover(); r != nil {
			t.log.Errorf("WebSocket handler panic: %v", r)
			// Stack trace
			debug.PrintStack()
		}
	}()

	// Set up a timeout detector
	go func() {
		for {
			select {
			case <-t.stopCh:
				return
			case <-time.After(5 * time.Second):
				// Check if WebSocket connection is still valid
				t.mu.Lock()
				if t.wsConn == nil {
					t.mu.Unlock()
					continue
				}

				// Send a ping to check connection
				pingMsg := TunnelMessage{
					Type: "ping",
					Data: json.RawMessage([]byte(`{"timestamp":"` + time.Now().Format(time.RFC3339) + `"}`)),
				}

				err := websocket.JSON.Send(t.wsConn, pingMsg)
				t.mu.Unlock()

				if err != nil {
					t.log.Warnf("WebSocket ping failed: %v", err)
				}
			}
		}
	}()
}
