package server

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"nxpose/internal/config"
	"nxpose/internal/logger"
)

// TestRegistrationEndpoint tests the client registration endpoint
func TestRegistrationEndpoint(t *testing.T) {
	// Create test server
	cfg := config.DefaultServerConfig()
	log := logger.New(true)

	// Create simple TLS config for testing
	tlsConfig := &tls.Config{
		InsecureSkipVerify: true,
	}

	srv, err := NewServer(cfg, tlsConfig, log)
	assert.NoError(t, err)

	// Create test registration request
	req := RegistrationRequest{
		ClientName:   "test-client",
		ClientRegion: "us-west",
	}

	// Convert to JSON
	reqBody, err := json.Marshal(req)
	assert.NoError(t, err)

	// Create HTTP request
	httpReq, err := http.NewRequest("POST", "/api/register", bytes.NewBuffer(reqBody))
	assert.NoError(t, err)
	httpReq.Header.Set("Content-Type", "application/json")

	// Create response recorder
	rr := httptest.NewRecorder()

	// Call handler directly
	handler := http.HandlerFunc(srv.handleRegister)
	handler.ServeHTTP(rr, httpReq)

	// Check response
	assert.Equal(t, http.StatusOK, rr.Code)

	// Parse response
	var resp RegistrationResponse
	err = json.NewDecoder(rr.Body).Decode(&resp)
	assert.NoError(t, err)

	// Verify response fields
	assert.True(t, resp.Success)
	assert.NotEmpty(t, resp.ClientID)
	assert.NotEmpty(t, resp.Certificate)
	assert.NotEmpty(t, resp.ExpiresAt)
}

// TestTunnelCreation tests the tunnel creation endpoint
func TestTunnelCreation(t *testing.T) {
	// Create test server
	cfg := config.DefaultServerConfig()
	log := logger.New(true)

	// Create simple TLS config for testing
	tlsConfig := &tls.Config{
		InsecureSkipVerify: true,
	}

	srv, err := NewServer(cfg, tlsConfig, log)
	assert.NoError(t, err)

	// First register a client
	regReq := RegistrationRequest{
		ClientName: "test-client",
	}

	regReqBody, err := json.Marshal(regReq)
	assert.NoError(t, err)

	regHttpReq, err := http.NewRequest("POST", "/api/register", bytes.NewBuffer(regReqBody))
	assert.NoError(t, err)
	regHttpReq.Header.Set("Content-Type", "application/json")

	regRR := httptest.NewRecorder()
	http.HandlerFunc(srv.handleRegister).ServeHTTP(regRR, regHttpReq)

	var regResp RegistrationResponse
	err = json.NewDecoder(regRR.Body).Decode(&regResp)
	assert.NoError(t, err)

	// Now create a tunnel
	tunnelReq := TunnelRequest{
		ClientID:    regResp.ClientID,
		Protocol:    "http",
		Port:        8080,
		Certificate: regResp.Certificate,
	}

	tunnelReqBody, err := json.Marshal(tunnelReq)
	assert.NoError(t, err)

	tunnelHttpReq, err := http.NewRequest("POST", "/api/tunnel", bytes.NewBuffer(tunnelReqBody))
	assert.NoError(t, err)
	tunnelHttpReq.Header.Set("Content-Type", "application/json")

	tunnelRR := httptest.NewRecorder()
	http.HandlerFunc(srv.handleTunnel).ServeHTTP(tunnelRR, tunnelHttpReq)

	// Check response
	assert.Equal(t, http.StatusOK, tunnelRR.Code)

	// Parse response
	var tunnelResp TunnelResponse
	err = json.NewDecoder(tunnelRR.Body).Decode(&tunnelResp)
	assert.NoError(t, err)

	// Verify response fields
	assert.True(t, tunnelResp.Success)
	assert.NotEmpty(t, tunnelResp.TunnelID)
	assert.NotEmpty(t, tunnelResp.PublicURL)

	// Verify tunnel was created
	srv.tunnels.mu.RLock()
	tunnel, exists := srv.tunnels.tunnels[tunnelResp.TunnelID]
	srv.tunnels.mu.RUnlock()

	assert.True(t, exists)
	assert.Equal(t, regResp.ClientID, tunnel.ClientID)
	assert.Equal(t, "http", tunnel.Protocol)
	assert.Equal(t, 8080, tunnel.TargetPort)
}

// TestExtractSubdomain tests the subdomain extraction function
func TestExtractSubdomain(t *testing.T) {
	tests := []struct {
		hostname      string
		baseDomain    string
		wantSubdomain string
	}{
		{"abc123.example.com", "example.com", "abc123"},
		{"test.nxpose.local", "nxpose.local", "test"},
		{"example.com", "example.com", ""},
		{"sub.domain.example.com", "example.com", "sub.domain"},
		{"notrelated.com", "example.com", ""},
	}

	for _, tt := range tests {
		got := extractSubdomain(tt.hostname, tt.baseDomain)
		assert.Equal(t, tt.wantSubdomain, got)
	}
}
