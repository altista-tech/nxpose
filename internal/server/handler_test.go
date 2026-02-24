package server

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"nxpose/internal/config"
	"nxpose/internal/logger"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestServer creates a test server with default config and no external dependencies
func newTestServer(t *testing.T) *Server {
	t.Helper()
	cfg := config.DefaultServerConfig()
	log := logger.New(true)
	tlsConfig := &tls.Config{
		InsecureSkipVerify: true,
	}

	srv, err := NewServer(cfg, tlsConfig, log)
	require.NoError(t, err)
	return srv
}

// newTestServerWithConfig creates a test server with custom config
func newTestServerWithConfig(t *testing.T, cfg *config.ServerConfig) *Server {
	t.Helper()
	log := logger.New(true)
	tlsConfig := &tls.Config{
		InsecureSkipVerify: true,
	}

	srv, err := NewServer(cfg, tlsConfig, log)
	require.NoError(t, err)
	return srv
}

// registerTestClient registers a client and returns the response
func registerTestClient(t *testing.T, srv *Server) RegistrationResponse {
	t.Helper()
	regReq := RegistrationRequest{
		ClientName:   "test-client",
		ClientRegion: "us-west",
	}

	reqBody, err := json.Marshal(regReq)
	require.NoError(t, err)

	httpReq, err := http.NewRequest("POST", "/api/register", bytes.NewBuffer(reqBody))
	require.NoError(t, err)
	httpReq.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	http.HandlerFunc(srv.handleRegister).ServeHTTP(rr, httpReq)

	require.Equal(t, http.StatusOK, rr.Code)

	var resp RegistrationResponse
	err = json.NewDecoder(rr.Body).Decode(&resp)
	require.NoError(t, err)
	require.True(t, resp.Success)
	return resp
}

// createTestTunnel creates a tunnel using a registered client and returns the response
func createTestTunnel(t *testing.T, srv *Server, clientID, certificate, protocol string, port int) TunnelResponse {
	t.Helper()
	tunnelReq := TunnelRequest{
		ClientID:    clientID,
		Protocol:    protocol,
		Port:        port,
		Certificate: certificate,
	}

	reqBody, err := json.Marshal(tunnelReq)
	require.NoError(t, err)

	httpReq, err := http.NewRequest("POST", "/api/tunnel", bytes.NewBuffer(reqBody))
	require.NoError(t, err)
	httpReq.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	http.HandlerFunc(srv.handleTunnel).ServeHTTP(rr, httpReq)

	require.Equal(t, http.StatusOK, rr.Code)

	var resp TunnelResponse
	err = json.NewDecoder(rr.Body).Decode(&resp)
	require.NoError(t, err)
	require.True(t, resp.Success)
	return resp
}

// --- Health / Status endpoint tests ---

func TestHandleStatus_ReturnsJSON(t *testing.T) {
	srv := newTestServer(t)

	req, err := http.NewRequest("GET", "/api/status", nil)
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	http.HandlerFunc(srv.handleStatus).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))

	var status map[string]interface{}
	err = json.NewDecoder(rr.Body).Decode(&status)
	require.NoError(t, err)

	assert.Contains(t, status, "version")
	assert.Contains(t, status, "tunnels")
	assert.Contains(t, status, "features")
	assert.Contains(t, status, "tls")
}

func TestHandleStatus_ShowsFeatures(t *testing.T) {
	srv := newTestServer(t)

	req, err := http.NewRequest("GET", "/api/status", nil)
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	http.HandlerFunc(srv.handleStatus).ServeHTTP(rr, req)

	var status map[string]interface{}
	err = json.NewDecoder(rr.Body).Decode(&status)
	require.NoError(t, err)

	features, ok := status["features"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, false, features["oauth2_enabled"])
	assert.Equal(t, false, features["mongodb_enabled"])
	assert.Equal(t, false, features["letsencrypt_enabled"])
}

func TestHandleStatus_TunnelCountUpdates(t *testing.T) {
	srv := newTestServer(t)

	// Status before tunnels
	req, _ := http.NewRequest("GET", "/api/status", nil)
	rr := httptest.NewRecorder()
	http.HandlerFunc(srv.handleStatus).ServeHTTP(rr, req)

	var status map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&status)
	assert.Equal(t, float64(0), status["tunnels"])

	// Register and create a tunnel
	regResp := registerTestClient(t, srv)
	createTestTunnel(t, srv, regResp.ClientID, regResp.Certificate, "http", 8080)

	// Status after tunnel creation
	req, _ = http.NewRequest("GET", "/api/status", nil)
	rr = httptest.NewRecorder()
	http.HandlerFunc(srv.handleStatus).ServeHTTP(rr, req)

	json.NewDecoder(rr.Body).Decode(&status)
	assert.Equal(t, float64(1), status["tunnels"])
}

func TestHandleStatus_TLSInfo(t *testing.T) {
	srv := newTestServer(t)

	req, _ := http.NewRequest("GET", "/api/status", nil)
	rr := httptest.NewRecorder()
	http.HandlerFunc(srv.handleStatus).ServeHTTP(rr, req)

	var status map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&status)

	tlsInfo, ok := status["tls"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, true, tlsInfo["enabled"])
	assert.Equal(t, "None", tlsInfo["provider"])
}

// --- Registration endpoint tests ---

func TestHandleRegister_Success(t *testing.T) {
	srv := newTestServer(t)
	resp := registerTestClient(t, srv)

	assert.True(t, resp.Success)
	assert.NotEmpty(t, resp.ClientID)
	assert.NotEmpty(t, resp.Certificate)
	assert.NotEmpty(t, resp.Message)
	assert.False(t, resp.ExpiresAt.IsZero())
}

func TestHandleRegister_InvalidJSON(t *testing.T) {
	srv := newTestServer(t)

	req, _ := http.NewRequest("POST", "/api/register", bytes.NewBuffer([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	http.HandlerFunc(srv.handleRegister).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHandleRegister_EmptyBody(t *testing.T) {
	srv := newTestServer(t)

	req, _ := http.NewRequest("POST", "/api/register", bytes.NewBuffer([]byte("{}")))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	http.HandlerFunc(srv.handleRegister).ServeHTTP(rr, req)

	// Empty body with valid JSON should still succeed (client name is optional)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestHandleRegister_MultipleRegistrations(t *testing.T) {
	srv := newTestServer(t)

	resp1 := registerTestClient(t, srv)
	resp2 := registerTestClient(t, srv)

	assert.NotEqual(t, resp1.ClientID, resp2.ClientID, "each registration should produce a unique client ID")
}

// --- Tunnel creation endpoint tests ---

func TestHandleTunnel_HTTPProtocol(t *testing.T) {
	srv := newTestServer(t)
	regResp := registerTestClient(t, srv)

	resp := createTestTunnel(t, srv, regResp.ClientID, regResp.Certificate, "http", 8080)

	assert.True(t, resp.Success)
	assert.NotEmpty(t, resp.TunnelID)
	assert.Contains(t, resp.PublicURL, "http://")
	assert.Contains(t, resp.PublicURL, ".localhost")
}

func TestHandleTunnel_HTTPSProtocol(t *testing.T) {
	srv := newTestServer(t)
	regResp := registerTestClient(t, srv)

	resp := createTestTunnel(t, srv, regResp.ClientID, regResp.Certificate, "https", 8443)

	assert.True(t, resp.Success)
	assert.NotEmpty(t, resp.TunnelID)
	// Without certs configured, HTTPS falls back to HTTP
	assert.Contains(t, resp.PublicURL, ".localhost")
}

func TestHandleTunnel_TCPProtocol(t *testing.T) {
	srv := newTestServer(t)
	regResp := registerTestClient(t, srv)

	resp := createTestTunnel(t, srv, regResp.ClientID, regResp.Certificate, "tcp", 5432)

	assert.True(t, resp.Success)
	assert.NotEmpty(t, resp.TunnelID)
	assert.Contains(t, resp.PublicURL, "tcp://")
}

func TestHandleTunnel_InvalidProtocol(t *testing.T) {
	srv := newTestServer(t)
	regResp := registerTestClient(t, srv)

	tunnelReq := TunnelRequest{
		ClientID:    regResp.ClientID,
		Protocol:    "ftp",
		Port:        21,
		Certificate: regResp.Certificate,
	}

	reqBody, _ := json.Marshal(tunnelReq)
	req, _ := http.NewRequest("POST", "/api/tunnel", bytes.NewBuffer(reqBody))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	http.HandlerFunc(srv.handleTunnel).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHandleTunnel_MissingCertificate(t *testing.T) {
	srv := newTestServer(t)
	regResp := registerTestClient(t, srv)

	tunnelReq := TunnelRequest{
		ClientID: regResp.ClientID,
		Protocol: "http",
		Port:     8080,
		// No certificate
	}

	reqBody, _ := json.Marshal(tunnelReq)
	req, _ := http.NewRequest("POST", "/api/tunnel", bytes.NewBuffer(reqBody))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	http.HandlerFunc(srv.handleTunnel).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHandleTunnel_InvalidJSON(t *testing.T) {
	srv := newTestServer(t)

	req, _ := http.NewRequest("POST", "/api/tunnel", bytes.NewBuffer([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	http.HandlerFunc(srv.handleTunnel).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHandleTunnel_MethodNotAllowed(t *testing.T) {
	srv := newTestServer(t)

	req, _ := http.NewRequest("GET", "/api/tunnel", nil)
	rr := httptest.NewRecorder()
	http.HandlerFunc(srv.handleTunnel).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusMethodNotAllowed, rr.Code)
}

func TestHandleTunnel_TunnelStoredInRegistry(t *testing.T) {
	srv := newTestServer(t)
	regResp := registerTestClient(t, srv)
	tunnelResp := createTestTunnel(t, srv, regResp.ClientID, regResp.Certificate, "http", 3000)

	srv.tunnels.mu.RLock()
	tunnel, exists := srv.tunnels.tunnels[tunnelResp.TunnelID]
	srv.tunnels.mu.RUnlock()

	assert.True(t, exists)
	assert.Equal(t, regResp.ClientID, tunnel.ClientID)
	assert.Equal(t, "http", tunnel.Protocol)
	assert.Equal(t, 3000, tunnel.TargetPort)
	assert.NotEmpty(t, tunnel.Subdomain)
	assert.False(t, tunnel.CreateTime.IsZero())
}

func TestHandleTunnel_UniqueSubdomains(t *testing.T) {
	srv := newTestServer(t)
	regResp := registerTestClient(t, srv)

	resp1 := createTestTunnel(t, srv, regResp.ClientID, regResp.Certificate, "http", 8080)
	resp2 := createTestTunnel(t, srv, regResp.ClientID, regResp.Certificate, "http", 8081)

	// Extract subdomains from tunnel registry
	srv.tunnels.mu.RLock()
	t1 := srv.tunnels.tunnels[resp1.TunnelID]
	t2 := srv.tunnels.tunnels[resp2.TunnelID]
	srv.tunnels.mu.RUnlock()

	assert.NotEqual(t, t1.Subdomain, t2.Subdomain, "each tunnel should have a unique subdomain")
}

// --- Subdomain routing and wildcard matching tests ---

func TestExtractSubdomain_BasicSubdomain(t *testing.T) {
	srv := newTestServer(t)
	srv.config.BaseDomain = "example.com"

	got := srv.extractSubdomain("abc123.example.com", "example.com")
	assert.Equal(t, "abc123", got)
}

func TestExtractSubdomain_NestedSubdomain(t *testing.T) {
	srv := newTestServer(t)
	srv.config.BaseDomain = "example.com"

	got := srv.extractSubdomain("sub.domain.example.com", "example.com")
	assert.Equal(t, "sub.domain", got)
}

func TestExtractSubdomain_NoSubdomain(t *testing.T) {
	srv := newTestServer(t)
	srv.config.BaseDomain = "example.com"

	got := srv.extractSubdomain("example.com", "example.com")
	assert.Equal(t, "", got)
}

func TestExtractSubdomain_UnrelatedDomain(t *testing.T) {
	srv := newTestServer(t)
	srv.config.BaseDomain = "example.com"

	got := srv.extractSubdomain("notrelated.com", "example.com")
	assert.Equal(t, "", got)
}

func TestExtractSubdomain_WithPort(t *testing.T) {
	srv := newTestServer(t)
	srv.config.BaseDomain = "example.com"

	got := srv.extractSubdomain("abc123.example.com:8443", "example.com")
	assert.Equal(t, "abc123", got)
}

func TestExtractSubdomain_BaseDomainWithPort(t *testing.T) {
	srv := newTestServer(t)
	srv.config.BaseDomain = "example.com"

	got := srv.extractSubdomain("example.com:8443", "example.com")
	assert.Equal(t, "", got)
}

func TestHandleTunnelRequest_NoSubdomain_ShowsWelcome(t *testing.T) {
	srv := newTestServer(t)
	srv.config.BaseDomain = "example.com"

	req, _ := http.NewRequest("GET", "/", nil)
	req.Host = "example.com"

	rr := httptest.NewRecorder()
	http.HandlerFunc(srv.handleTunnelRequest).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "Welcome to NXpose")
}

func TestHandleTunnelRequest_UnknownSubdomain_Returns404(t *testing.T) {
	srv := newTestServer(t)
	srv.config.BaseDomain = "example.com"

	req, _ := http.NewRequest("GET", "/", nil)
	req.Host = "nonexistent.example.com"

	rr := httptest.NewRecorder()
	http.HandlerFunc(srv.handleTunnelRequest).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestHandleTunnelRequest_MatchingSubdomain_NoWebSocket_Returns503(t *testing.T) {
	srv := newTestServer(t)
	srv.config.BaseDomain = "example.com"

	// Create a tunnel in the registry directly
	tunnelID := "test-tunnel-id"
	srv.tunnels.mu.Lock()
	srv.tunnels.tunnels[tunnelID] = &Tunnel{
		ID:         tunnelID,
		ClientID:   "test-client",
		Protocol:   "http",
		Subdomain:  "myapp",
		TargetPort: 8080,
		CreateTime: time.Now(),
		LastActive: time.Now(),
	}
	srv.tunnels.mu.Unlock()

	req, _ := http.NewRequest("GET", "/somepath", nil)
	req.Host = "myapp.example.com"

	rr := httptest.NewRecorder()
	http.HandlerFunc(srv.handleTunnelRequest).ServeHTTP(rr, req)

	// Should return 503 because there's no WebSocket connection for the tunnel
	assert.Equal(t, http.StatusServiceUnavailable, rr.Code)
}

func TestHandleTunnelRequest_UpdatesLastActive(t *testing.T) {
	srv := newTestServer(t)
	srv.config.BaseDomain = "example.com"

	tunnelID := "test-tunnel-active"
	oldTime := time.Now().Add(-1 * time.Hour)
	srv.tunnels.mu.Lock()
	srv.tunnels.tunnels[tunnelID] = &Tunnel{
		ID:         tunnelID,
		ClientID:   "test-client",
		Protocol:   "http",
		Subdomain:  "activetest",
		TargetPort: 8080,
		CreateTime: oldTime,
		LastActive: oldTime,
	}
	srv.tunnels.mu.Unlock()

	req, _ := http.NewRequest("GET", "/", nil)
	req.Host = "activetest.example.com"

	rr := httptest.NewRecorder()
	http.HandlerFunc(srv.handleTunnelRequest).ServeHTTP(rr, req)

	srv.tunnels.mu.RLock()
	tunnel := srv.tunnels.tunnels[tunnelID]
	srv.tunnels.mu.RUnlock()

	assert.True(t, tunnel.LastActive.After(oldTime), "LastActive should be updated after a request")
}

func TestHandleTunnelRequest_TCPOverHTTP_Returns400(t *testing.T) {
	srv := newTestServer(t)
	srv.config.BaseDomain = "example.com"

	tunnelID := "tcp-tunnel"
	srv.tunnels.mu.Lock()
	srv.tunnels.tunnels[tunnelID] = &Tunnel{
		ID:         tunnelID,
		ClientID:   "test-client",
		Protocol:   "tcp",
		Subdomain:  "tcptest",
		TargetPort: 5432,
		CreateTime: time.Now(),
		LastActive: time.Now(),
	}
	srv.tunnels.mu.Unlock()

	// Register a fake WebSocket tunnel so we get to protocol check
	srv.wsManager.RegisterWebSocketTunnel(tunnelID, &WebSocketTunnel{
		ID:        "ws-test",
		TunnelID:  tunnelID,
		Connected: true,
	})

	req, _ := http.NewRequest("GET", "/", nil)
	req.Host = "tcptest.example.com"

	rr := httptest.NewRecorder()
	http.HandlerFunc(srv.handleTunnelRequest).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

// --- Tunnel limit enforcement tests ---

func TestHandleTunnel_LimitEnforcement_ManualCount(t *testing.T) {
	cfg := config.DefaultServerConfig()
	cfg.TunnelLimits.MaxPerUser = 2
	srv := newTestServerWithConfig(t, cfg)

	regResp := registerTestClient(t, srv)

	// Create tunnels up to the limit
	createTestTunnel(t, srv, regResp.ClientID, regResp.Certificate, "http", 8080)
	createTestTunnel(t, srv, regResp.ClientID, regResp.Certificate, "http", 8081)

	// Third tunnel should be rejected
	tunnelReq := TunnelRequest{
		ClientID:    regResp.ClientID,
		Protocol:    "http",
		Port:        8082,
		Certificate: regResp.Certificate,
	}

	reqBody, _ := json.Marshal(tunnelReq)
	req, _ := http.NewRequest("POST", "/api/tunnel", bytes.NewBuffer(reqBody))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	http.HandlerFunc(srv.handleTunnel).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusTooManyRequests, rr.Code)
	assert.Contains(t, rr.Body.String(), "Maximum number of tunnels reached")
}

func TestHandleTunnel_LimitNotEnforced_WhenZero(t *testing.T) {
	cfg := config.DefaultServerConfig()
	cfg.TunnelLimits.MaxPerUser = 0
	srv := newTestServerWithConfig(t, cfg)

	regResp := registerTestClient(t, srv)

	// Should be able to create many tunnels when limit is 0
	for i := 0; i < 10; i++ {
		resp := createTestTunnel(t, srv, regResp.ClientID, regResp.Certificate, "http", 8080+i)
		assert.True(t, resp.Success)
	}
}

func TestHandleTunnel_LimitPerUser_DifferentUsers(t *testing.T) {
	cfg := config.DefaultServerConfig()
	cfg.TunnelLimits.MaxPerUser = 1
	srv := newTestServerWithConfig(t, cfg)

	// Register two different clients
	regResp1 := registerTestClient(t, srv)
	regResp2 := registerTestClient(t, srv)

	// Each user should be able to create 1 tunnel
	createTestTunnel(t, srv, regResp1.ClientID, regResp1.Certificate, "http", 8080)
	createTestTunnel(t, srv, regResp2.ClientID, regResp2.Certificate, "http", 8081)

	// Both should be rejected for a second tunnel
	tunnelReq := TunnelRequest{
		ClientID:    regResp1.ClientID,
		Protocol:    "http",
		Port:        8082,
		Certificate: regResp1.Certificate,
	}

	reqBody, _ := json.Marshal(tunnelReq)
	req, _ := http.NewRequest("POST", "/api/tunnel", bytes.NewBuffer(reqBody))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	http.HandlerFunc(srv.handleTunnel).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusTooManyRequests, rr.Code)
}

func TestHandleTunnel_ExpirationSet(t *testing.T) {
	cfg := config.DefaultServerConfig()
	cfg.TunnelLimits.MaxConnection = "1h"
	srv := newTestServerWithConfig(t, cfg)

	regResp := registerTestClient(t, srv)
	tunnelResp := createTestTunnel(t, srv, regResp.ClientID, regResp.Certificate, "http", 8080)

	srv.tunnels.mu.RLock()
	tunnel := srv.tunnels.tunnels[tunnelResp.TunnelID]
	srv.tunnels.mu.RUnlock()

	assert.False(t, tunnel.ExpiresAt.IsZero(), "tunnel should have an expiration time set")
	assert.True(t, tunnel.ExpiresAt.After(time.Now()), "expiration should be in the future")
	assert.True(t, tunnel.ExpiresAt.Before(time.Now().Add(2*time.Hour)), "expiration should be within 2 hours")
}

func TestHandleTunnel_NoExpiration_WhenNotConfigured(t *testing.T) {
	cfg := config.DefaultServerConfig()
	cfg.TunnelLimits.MaxConnection = ""
	srv := newTestServerWithConfig(t, cfg)

	regResp := registerTestClient(t, srv)
	tunnelResp := createTestTunnel(t, srv, regResp.ClientID, regResp.Certificate, "http", 8080)

	srv.tunnels.mu.RLock()
	tunnel := srv.tunnels.tunnels[tunnelResp.TunnelID]
	srv.tunnels.mu.RUnlock()

	assert.True(t, tunnel.ExpiresAt.IsZero(), "tunnel should have no expiration when MaxConnection is empty")
}

// --- Welcome page tests ---

func TestHandleWelcomePage(t *testing.T) {
	srv := newTestServer(t)

	req, _ := http.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	http.HandlerFunc(srv.handleWelcomePage).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Header().Get("Content-Type"), "text/html")
	assert.Contains(t, rr.Body.String(), "Welcome to NXpose")
	assert.Contains(t, rr.Body.String(), srv.config.BaseDomain)
}

// --- WebSocket handler validation tests ---

func TestHandleWebSocket_MissingClientID(t *testing.T) {
	srv := newTestServer(t)

	req, _ := http.NewRequest("GET", "/api/ws", nil)
	rr := httptest.NewRecorder()
	http.HandlerFunc(srv.handleWebSocket).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

// --- Session management tests ---

func TestSessionStore_CookieStoreCreated(t *testing.T) {
	srv := newTestServer(t)
	assert.NotNil(t, srv.sessionStore, "session store should be initialized")
}

func TestSessionStore_WithCustomKey(t *testing.T) {
	cfg := config.DefaultServerConfig()
	cfg.OAuth2.SessionKey = "test-secure-session-key-32chars!"
	srv := newTestServerWithConfig(t, cfg)
	assert.NotNil(t, srv.sessionStore)
}

func TestSessionStore_InsecureKeyWarning(t *testing.T) {
	// When no session key is provided, server should use insecure fallback
	cfg := config.DefaultServerConfig()
	cfg.OAuth2.SessionKey = ""
	srv := newTestServerWithConfig(t, cfg)
	assert.NotNil(t, srv.sessionStore)
}

// --- generateRandomSubdomain tests ---

func TestGenerateRandomSubdomain_Length(t *testing.T) {
	for _, length := range []int{4, 8, 12, 16} {
		subdomain := generateRandomSubdomain(length)
		assert.Equal(t, length, len(subdomain))
	}
}

func TestGenerateRandomSubdomain_ValidCharacters(t *testing.T) {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	subdomain := generateRandomSubdomain(100)
	for _, c := range subdomain {
		assert.Contains(t, charset, string(c))
	}
}

func TestGenerateRandomSubdomain_Uniqueness(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		subdomain := generateRandomSubdomain(8)
		seen[subdomain] = true
	}
	// With 36^8 possible values, collisions should be very rare
	assert.Greater(t, len(seen), 90, "most generated subdomains should be unique")
}

// --- OAuth2 validation tests ---

func TestValidateOAuthConfig_Disabled(t *testing.T) {
	log := logger.New(true)
	result := ValidateOAuthConfig(OAuthConfig{Enabled: false}, log.Logger)
	assert.Equal(t, true, result["valid"])
}

func TestValidateOAuthConfig_MissingRedirectURL(t *testing.T) {
	log := logger.New(true)
	oauthCfg := OAuthConfig{
		Enabled:    true,
		SessionKey: "a-long-enough-session-key",
		Providers: []ProviderConfig{
			{Name: "github", ClientID: "id", ClientSecret: "secret", Scopes: []string{"user"}},
		},
	}
	result := ValidateOAuthConfig(oauthCfg, log.Logger)
	assert.Equal(t, false, result["valid"])
}

func TestValidateOAuthConfig_NonHTTPSRedirect(t *testing.T) {
	log := logger.New(true)
	oauthCfg := OAuthConfig{
		Enabled:     true,
		RedirectURL: "http://example.com/callback",
		SessionKey:  "a-long-enough-session-key",
		Providers: []ProviderConfig{
			{Name: "github", ClientID: "id", ClientSecret: "secret", Scopes: []string{"user"}},
		},
	}
	result := ValidateOAuthConfig(oauthCfg, log.Logger)
	assert.Equal(t, false, result["valid"])
}

func TestValidateOAuthConfig_ShortSessionKey(t *testing.T) {
	log := logger.New(true)
	oauthCfg := OAuthConfig{
		Enabled:     true,
		RedirectURL: "https://example.com/callback",
		SessionKey:  "short",
		Providers: []ProviderConfig{
			{Name: "github", ClientID: "id", ClientSecret: "secret", Scopes: []string{"user"}},
		},
	}
	result := ValidateOAuthConfig(oauthCfg, log.Logger)
	assert.Equal(t, false, result["valid"])
}

func TestValidateOAuthConfig_DefaultSessionKey(t *testing.T) {
	log := logger.New(true)
	oauthCfg := OAuthConfig{
		Enabled:     true,
		RedirectURL: "https://example.com/callback",
		SessionKey:  "change-this-to-a-random-secret-key",
		Providers: []ProviderConfig{
			{Name: "github", ClientID: "id", ClientSecret: "secret", Scopes: []string{"user"}},
		},
	}
	result := ValidateOAuthConfig(oauthCfg, log.Logger)
	assert.Equal(t, false, result["valid"])
}

func TestValidateOAuthConfig_NoProviders(t *testing.T) {
	log := logger.New(true)
	oauthCfg := OAuthConfig{
		Enabled:     true,
		RedirectURL: "https://example.com/callback",
		SessionKey:  "a-long-enough-session-key",
		Providers:   []ProviderConfig{},
	}
	result := ValidateOAuthConfig(oauthCfg, log.Logger)
	assert.Equal(t, false, result["valid"])
}

func TestValidateOAuthConfig_ProviderMissingClientID(t *testing.T) {
	log := logger.New(true)
	oauthCfg := OAuthConfig{
		Enabled:     true,
		RedirectURL: "https://example.com/callback",
		SessionKey:  "a-long-enough-session-key",
		Providers: []ProviderConfig{
			{Name: "github", ClientID: "", ClientSecret: "secret", Scopes: []string{"user"}},
		},
	}
	result := ValidateOAuthConfig(oauthCfg, log.Logger)
	assert.Equal(t, false, result["valid"])
}

func TestValidateOAuthConfig_ProviderMissingScopes(t *testing.T) {
	log := logger.New(true)
	oauthCfg := OAuthConfig{
		Enabled:     true,
		RedirectURL: "https://example.com/callback",
		SessionKey:  "a-long-enough-session-key",
		Providers: []ProviderConfig{
			{Name: "github", ClientID: "id", ClientSecret: "secret", Scopes: []string{}},
		},
	}
	result := ValidateOAuthConfig(oauthCfg, log.Logger)
	assert.Equal(t, false, result["valid"])
}

func TestValidateOAuthConfig_UnsupportedProvider(t *testing.T) {
	log := logger.New(true)
	oauthCfg := OAuthConfig{
		Enabled:     true,
		RedirectURL: "https://example.com/callback",
		SessionKey:  "a-long-enough-session-key",
		Providers: []ProviderConfig{
			{Name: "unsupported", ClientID: "id", ClientSecret: "secret", Scopes: []string{"user"}},
		},
	}
	result := ValidateOAuthConfig(oauthCfg, log.Logger)
	assert.Equal(t, false, result["valid"])
}

func TestValidateOAuthConfig_ValidConfig(t *testing.T) {
	log := logger.New(true)
	oauthCfg := OAuthConfig{
		Enabled:     true,
		RedirectURL: "https://example.com/callback",
		SessionKey:  "a-long-enough-session-key",
		Providers: []ProviderConfig{
			{Name: "github", ClientID: "id", ClientSecret: "secret", Scopes: []string{"user"}},
		},
	}
	result := ValidateOAuthConfig(oauthCfg, log.Logger)
	assert.Equal(t, true, result["valid"])
}

// --- OAuth2 callback handling tests ---

func TestOAuthService_NewOAuthService_Disabled(t *testing.T) {
	log := logger.New(true)
	svc, err := NewOAuthService(OAuthConfig{Enabled: false}, log.Logger, "https://example.com", nil)
	assert.NoError(t, err)
	assert.Nil(t, svc)
}

func TestOAuthService_NewOAuthService_GitHubProvider(t *testing.T) {
	log := logger.New(true)
	oauthCfg := OAuthConfig{
		Enabled:    true,
		SessionKey: "test-session-key-long-enough",
		Providers: []ProviderConfig{
			{Name: "github", ClientID: "test-id", ClientSecret: "test-secret", Scopes: []string{"user", "repo"}},
		},
	}

	svc, err := NewOAuthService(oauthCfg, log.Logger, "https://example.com", nil)
	require.NoError(t, err)
	require.NotNil(t, svc)

	assert.Contains(t, svc.providers, "github")
	assert.Equal(t, "test-id", svc.providers["github"].ClientID)
	assert.Equal(t, "test-secret", svc.providers["github"].ClientSecret)
	assert.Contains(t, svc.providers["github"].RedirectURL, "/auth/callback/github")
}

func TestOAuthService_NewOAuthService_GoogleProvider(t *testing.T) {
	log := logger.New(true)
	oauthCfg := OAuthConfig{
		Enabled:    true,
		SessionKey: "test-session-key-long-enough",
		Providers: []ProviderConfig{
			{Name: "google", ClientID: "google-id", ClientSecret: "google-secret", Scopes: []string{"openid", "profile"}},
		},
	}

	svc, err := NewOAuthService(oauthCfg, log.Logger, "https://example.com", nil)
	require.NoError(t, err)
	require.NotNil(t, svc)

	assert.Contains(t, svc.providers, "google")
	assert.Equal(t, "google-id", svc.providers["google"].ClientID)
}

func TestOAuthService_NewOAuthService_SkipsMissingCredentials(t *testing.T) {
	log := logger.New(true)
	oauthCfg := OAuthConfig{
		Enabled:    true,
		SessionKey: "test-session-key-long-enough",
		Providers: []ProviderConfig{
			{Name: "github", ClientID: "", ClientSecret: "secret", Scopes: []string{"user"}},
			{Name: "google", ClientID: "id", ClientSecret: "", Scopes: []string{"openid"}},
		},
	}

	svc, err := NewOAuthService(oauthCfg, log.Logger, "https://example.com", nil)
	require.NoError(t, err)
	require.NotNil(t, svc)

	assert.Empty(t, svc.providers, "providers with missing credentials should be skipped")
}

func TestOAuthService_NewOAuthService_UnsupportedProvider(t *testing.T) {
	log := logger.New(true)
	oauthCfg := OAuthConfig{
		Enabled:    true,
		SessionKey: "test-session-key-long-enough",
		Providers: []ProviderConfig{
			{Name: "unsupported", ClientID: "id", ClientSecret: "secret", Scopes: []string{"user"}},
		},
	}

	svc, err := NewOAuthService(oauthCfg, log.Logger, "https://example.com", nil)
	require.NoError(t, err)
	require.NotNil(t, svc)

	assert.Empty(t, svc.providers)
}

func TestOAuthService_DefaultDurations(t *testing.T) {
	log := logger.New(true)
	oauthCfg := OAuthConfig{
		Enabled:    true,
		SessionKey: "test-session-key-long-enough",
		Providers:  []ProviderConfig{},
	}

	svc, err := NewOAuthService(oauthCfg, log.Logger, "https://example.com", nil)
	require.NoError(t, err)
	require.NotNil(t, svc)

	assert.Equal(t, 5*time.Minute, svc.config.TokenDuration)
	assert.Equal(t, 24*time.Hour, svc.config.CookieDuration)
}

func TestOAuthService_RegisterPage(t *testing.T) {
	log := logger.New(true)
	oauthCfg := OAuthConfig{
		Enabled:    true,
		SessionKey: "test-session-key-long-enough",
		Providers: []ProviderConfig{
			{Name: "github", ClientID: "id", ClientSecret: "secret", Scopes: []string{"user"}},
		},
	}

	svc, err := NewOAuthService(oauthCfg, log.Logger, "https://example.com", nil)
	require.NoError(t, err)

	req, _ := http.NewRequest("GET", "/auth/register", nil)
	rr := httptest.NewRecorder()
	svc.handleRegister(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Header().Get("Content-Type"), "text/html")
	assert.Contains(t, rr.Body.String(), "GitHub")
}

func TestOAuthService_RegisterPage_NoProviders(t *testing.T) {
	log := logger.New(true)
	oauthCfg := OAuthConfig{
		Enabled:    true,
		SessionKey: "test-session-key-long-enough",
		Providers:  []ProviderConfig{},
	}

	svc, err := NewOAuthService(oauthCfg, log.Logger, "https://example.com", nil)
	require.NoError(t, err)

	req, _ := http.NewRequest("GET", "/auth/register", nil)
	rr := httptest.NewRecorder()
	svc.handleRegister(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "No OAuth providers are properly configured")
}

func TestOAuthService_LoginHandler_InvalidProvider(t *testing.T) {
	log := logger.New(true)
	oauthCfg := OAuthConfig{
		Enabled:    true,
		SessionKey: "test-session-key-long-enough",
		Providers: []ProviderConfig{
			{Name: "github", ClientID: "id", ClientSecret: "secret", Scopes: []string{"user"}},
		},
	}

	svc, err := NewOAuthService(oauthCfg, log.Logger, "https://example.com", nil)
	require.NoError(t, err)

	// Try to login with a provider that doesn't exist
	handler := svc.handleLogin("nonexistent")
	req, _ := http.NewRequest("GET", "/auth/login/nonexistent", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestOAuthService_LoginHandler_ValidProvider_Redirects(t *testing.T) {
	log := logger.New(true)
	oauthCfg := OAuthConfig{
		Enabled:    true,
		SessionKey: "test-session-key-long-enough",
		Providers: []ProviderConfig{
			{Name: "github", ClientID: "id", ClientSecret: "secret", Scopes: []string{"user"}},
		},
	}

	svc, err := NewOAuthService(oauthCfg, log.Logger, "https://example.com", nil)
	require.NoError(t, err)

	handler := svc.handleLogin("github")
	req, _ := http.NewRequest("GET", "/auth/login/github", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)

	// Should redirect to GitHub OAuth
	assert.Equal(t, http.StatusFound, rr.Code)
	location := rr.Header().Get("Location")
	assert.Contains(t, location, "github.com")
}

func TestOAuthService_OAuthDone_MissingParams(t *testing.T) {
	log := logger.New(true)
	oauthCfg := OAuthConfig{
		Enabled:    true,
		SessionKey: "test-session-key-long-enough",
		Providers:  []ProviderConfig{},
	}

	svc, err := NewOAuthService(oauthCfg, log.Logger, "https://example.com", nil)
	require.NoError(t, err)

	req, _ := http.NewRequest("GET", "/auth/oauth-done", nil)
	rr := httptest.NewRecorder()
	svc.handleOAuthDone(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestOAuthService_OAuthDone_WithUserInfo(t *testing.T) {
	log := logger.New(true)
	oauthCfg := OAuthConfig{
		Enabled:    true,
		SessionKey: "test-session-key-long-enough",
		Providers:  []ProviderConfig{},
	}

	svc, err := NewOAuthService(oauthCfg, log.Logger, "https://example.com", nil)
	require.NoError(t, err)

	req, _ := http.NewRequest("GET", "/auth/oauth-done?user_id=test-user-id&user_name=TestUser", nil)
	rr := httptest.NewRecorder()
	svc.handleOAuthDone(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))

	var resp map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&resp)
	assert.Equal(t, "TestUser", resp["name"])
	assert.Equal(t, "test-user-id", resp["id"])
}

func TestOAuthService_GenerateCertificate(t *testing.T) {
	log := logger.New(true)
	oauthCfg := OAuthConfig{
		Enabled:    true,
		SessionKey: "test-session-key-long-enough",
		Providers:  []ProviderConfig{},
	}

	svc, err := NewOAuthService(oauthCfg, log.Logger, "https://example.com", nil)
	require.NoError(t, err)

	cert, err := svc.GenerateCertificate("test-user-123456", "Test User")
	require.NoError(t, err)

	assert.NotEmpty(t, cert["cert_id"])
	assert.Equal(t, "test-user-123456", cert["user_id"])
	assert.Equal(t, "Test User", cert["user_name"])
	assert.NotEmpty(t, cert["certificate"])
	assert.NotEmpty(t, cert["private_key"])
	assert.NotEmpty(t, cert["issued_at"])
	assert.NotEmpty(t, cert["expires_at"])
	assert.Contains(t, cert["certificate"].(string), "BEGIN CERTIFICATE")
	assert.Contains(t, cert["private_key"].(string), "BEGIN RSA PRIVATE KEY")
}

func TestOAuthService_ExtractUserName_GitHub(t *testing.T) {
	log := logger.New(true)
	svc := &OAuthService{logger: log.Logger}

	// Name present
	info := map[string]interface{}{"name": "John Doe", "login": "johnd"}
	assert.Equal(t, "John Doe", svc.extractUserName(info, "github"))

	// Name empty, fallback to login
	info = map[string]interface{}{"name": "", "login": "johnd"}
	assert.Equal(t, "johnd", svc.extractUserName(info, "github"))

	// Neither present
	info = map[string]interface{}{}
	assert.Equal(t, "github_user", svc.extractUserName(info, "github"))
}

func TestOAuthService_ExtractUserName_Google(t *testing.T) {
	log := logger.New(true)
	svc := &OAuthService{logger: log.Logger}

	// Full name present
	info := map[string]interface{}{"name": "Jane Doe"}
	assert.Equal(t, "Jane Doe", svc.extractUserName(info, "google"))

	// Only given + family name
	info = map[string]interface{}{"given_name": "Jane", "family_name": "Doe"}
	assert.Equal(t, "Jane Doe", svc.extractUserName(info, "google"))

	// Only email
	info = map[string]interface{}{"email": "jane@example.com"}
	assert.Equal(t, "jane@example.com", svc.extractUserName(info, "google"))

	// Nothing
	info = map[string]interface{}{}
	assert.Equal(t, "google_user", svc.extractUserName(info, "google"))
}

// --- Helper function tests ---

func TestMaskString(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"short", "*****"},                   // len <= 8: all stars
		{"12345678", "********"},             // len <= 8: all stars
		{"longer-string-here", "**********here"}, // len=18, len-8=10 stars + last 4
	}

	for _, tt := range tests {
		assert.Equal(t, tt.expected, maskString(tt.input))
	}
}

func TestFirstChars(t *testing.T) {
	assert.Equal(t, "abc", firstChars("abcdef", 3))
	assert.Equal(t, "ab", firstChars("ab", 5))
}

func TestLastChars(t *testing.T) {
	assert.Equal(t, "def", lastChars("abcdef", 3))
	assert.Equal(t, "ab", lastChars("ab", 5))
}
