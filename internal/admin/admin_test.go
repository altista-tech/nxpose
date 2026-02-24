package admin

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"nxpose/internal/config"
)

// mockProvider implements DataProvider for testing
type mockProvider struct {
	tunnels         []TunnelInfo
	clients         []ClientInfo
	stats           ServerStats
	maintenanceMode bool
	killErr         error
	mu              sync.Mutex
}

func newMockProvider() *mockProvider {
	return &mockProvider{
		tunnels: []TunnelInfo{
			{
				ID:          "tunnel-1",
				ClientID:    "client-1",
				Protocol:    "https",
				Subdomain:   "test",
				TargetPort:  3000,
				CreateTime:  time.Now().Add(-1 * time.Hour),
				LastActive:  time.Now().Add(-5 * time.Minute),
				ExpiresAt:   time.Now().Add(23 * time.Hour),
				Connections: 42,
				Connected:   true,
			},
			{
				ID:          "tunnel-2",
				ClientID:    "client-2",
				Protocol:    "http",
				Subdomain:   "api",
				TargetPort:  8080,
				CreateTime:  time.Now().Add(-30 * time.Minute),
				LastActive:  time.Now().Add(-2 * time.Minute),
				ExpiresAt:   time.Now().Add(23*time.Hour + 30*time.Minute),
				Connections: 10,
				Connected:   false,
			},
		},
		clients: []ClientInfo{
			{
				ID:          "client-1",
				TunnelCount: 1,
				LastActive:  time.Now().Add(-5 * time.Minute),
			},
			{
				ID:          "client-2",
				TunnelCount: 1,
				LastActive:  time.Now().Add(-2 * time.Minute),
			},
		},
		stats: ServerStats{
			ActiveTunnels:    2,
			ConnectedClients: 2,
			TotalConnections: 52,
			Uptime:           3 * time.Hour,
			UptimeStr:        "3h 0m",
			MaintenanceMode:  false,
		},
	}
}

func (m *mockProvider) GetTunnels() []TunnelInfo {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.tunnels
}

func (m *mockProvider) GetClients() []ClientInfo {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.clients
}

func (m *mockProvider) GetStats() ServerStats {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.stats
}

func (m *mockProvider) KillTunnel(tunnelID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.killErr != nil {
		return m.killErr
	}
	// Remove tunnel from the list
	for i, t := range m.tunnels {
		if t.ID == tunnelID {
			m.tunnels = append(m.tunnels[:i], m.tunnels[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("tunnel %s not found", tunnelID)
}

func (m *mockProvider) GetMaintenanceMode() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.maintenanceMode
}

func (m *mockProvider) SetMaintenanceMode(enabled bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.maintenanceMode = enabled
}

func setupTestHandler(t *testing.T) (*Handler, *mux.Router, *mockProvider) {
	t.Helper()

	provider := newMockProvider()
	adminConfig := &config.AdminConfig{
		Enabled:    true,
		PathPrefix: "/admin",
		AuthMethod: "none",
	}
	serverConfig := &config.ServerConfig{
		BindAddress: "0.0.0.0",
		Port:        8443,
		BaseDomain:  "example.com",
		TunnelLimits: config.TunnelLimitsConfig{
			MaxPerUser: 5,
		},
	}

	handler, err := NewHandler(adminConfig, serverConfig, provider)
	require.NoError(t, err)

	router := mux.NewRouter()
	handler.RegisterRoutes(router)

	return handler, router, provider
}

func setupBasicAuthHandler(t *testing.T) (*Handler, *mux.Router, *mockProvider) {
	t.Helper()

	provider := newMockProvider()
	adminConfig := &config.AdminConfig{
		Enabled:    true,
		PathPrefix: "/admin",
		AuthMethod: "basic",
		Username:   "admin",
		Password:   "secret",
	}
	serverConfig := &config.ServerConfig{
		BindAddress: "0.0.0.0",
		Port:        8443,
		BaseDomain:  "example.com",
	}

	handler, err := NewHandler(adminConfig, serverConfig, provider)
	require.NoError(t, err)

	router := mux.NewRouter()
	handler.RegisterRoutes(router)

	return handler, router, provider
}

// TestNewHandler tests handler creation
func TestNewHandler(t *testing.T) {
	provider := newMockProvider()
	adminConfig := &config.AdminConfig{
		Enabled:    true,
		PathPrefix: "/admin",
		AuthMethod: "none",
	}
	serverConfig := &config.ServerConfig{}

	handler, err := NewHandler(adminConfig, serverConfig, provider)
	assert.NoError(t, err)
	assert.NotNil(t, handler)
	assert.NotNil(t, handler.templates)
}

// TestDashboardPage tests the dashboard page renders
func TestDashboardPage(t *testing.T) {
	_, router, _ := setupTestHandler(t)

	req := httptest.NewRequest("GET", "/admin/", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, "Dashboard")
	assert.Contains(t, body, "Active Tunnels")
	assert.Contains(t, body, "Connected Clients")
	assert.Contains(t, body, "Uptime")
}

// TestTunnelsPage tests the tunnels page renders
func TestTunnelsPage(t *testing.T) {
	_, router, _ := setupTestHandler(t)

	req := httptest.NewRequest("GET", "/admin/tunnels", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, "Tunnel Management")
	assert.Contains(t, body, "tunnel-1")
	assert.Contains(t, body, "tunnel-2")
	assert.Contains(t, body, "test")
	assert.Contains(t, body, "api")
}

// TestClientsPage tests the clients page renders
func TestClientsPage(t *testing.T) {
	_, router, _ := setupTestHandler(t)

	req := httptest.NewRequest("GET", "/admin/clients", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, "Connected Clients")
	assert.Contains(t, body, "client-1")
	assert.Contains(t, body, "client-2")
}

// TestSettingsPage tests the settings page renders
func TestSettingsPage(t *testing.T) {
	_, router, _ := setupTestHandler(t)

	req := httptest.NewRequest("GET", "/admin/settings", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, "Server Settings")
	assert.Contains(t, body, "example.com")
	assert.Contains(t, body, "8443")
	assert.Contains(t, body, "Maintenance Mode")
}

// TestAPIStatsJSON tests JSON stats endpoint
func TestAPIStatsJSON(t *testing.T) {
	_, router, _ := setupTestHandler(t)

	req := httptest.NewRequest("GET", "/admin/api/stats", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "application/json")

	var stats ServerStats
	err := json.NewDecoder(w.Body).Decode(&stats)
	assert.NoError(t, err)
	assert.Equal(t, 2, stats.ActiveTunnels)
	assert.Equal(t, 2, stats.ConnectedClients)
	assert.Equal(t, int64(52), stats.TotalConnections)
}

// TestAPIStatsHTMX tests HTMX stats fragment endpoint
func TestAPIStatsHTMX(t *testing.T) {
	_, router, _ := setupTestHandler(t)

	req := httptest.NewRequest("GET", "/admin/api/stats", nil)
	req.Header.Set("HX-Request", "true")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "text/html")
	body := w.Body.String()
	assert.Contains(t, body, "Active Tunnels")
	assert.Contains(t, body, "2") // ActiveTunnels count
}

// TestAPITunnelsJSON tests JSON tunnels endpoint
func TestAPITunnelsJSON(t *testing.T) {
	_, router, _ := setupTestHandler(t)

	req := httptest.NewRequest("GET", "/admin/api/tunnels", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "application/json")

	var tunnels []TunnelInfo
	err := json.NewDecoder(w.Body).Decode(&tunnels)
	assert.NoError(t, err)
	assert.Len(t, tunnels, 2)
	assert.Equal(t, "tunnel-1", tunnels[0].ID)
}

// TestAPITunnelsHTMX tests HTMX tunnels fragment endpoint
func TestAPITunnelsHTMX(t *testing.T) {
	_, router, _ := setupTestHandler(t)

	req := httptest.NewRequest("GET", "/admin/api/tunnels", nil)
	req.Header.Set("HX-Request", "true")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "text/html")
	body := w.Body.String()
	assert.Contains(t, body, "tunnel-1")
	assert.Contains(t, body, "tunnel-2")
}

// TestAPIKillTunnel tests killing a tunnel
func TestAPIKillTunnel(t *testing.T) {
	_, router, provider := setupTestHandler(t)

	req := httptest.NewRequest("POST", "/admin/api/tunnels/tunnel-1/kill", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify tunnel was removed
	provider.mu.Lock()
	assert.Len(t, provider.tunnels, 1)
	assert.Equal(t, "tunnel-2", provider.tunnels[0].ID)
	provider.mu.Unlock()
}

// TestAPIKillTunnelHTMX tests killing a tunnel returns HTML fragment
func TestAPIKillTunnelHTMX(t *testing.T) {
	_, router, _ := setupTestHandler(t)

	req := httptest.NewRequest("POST", "/admin/api/tunnels/tunnel-1/kill", nil)
	req.Header.Set("HX-Request", "true")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "text/html")
	body := w.Body.String()
	// tunnel-1 should be gone, tunnel-2 should remain
	assert.NotContains(t, body, "tunnel-1")
	assert.Contains(t, body, "tunnel-2")
}

// TestAPIKillTunnelNotFound tests killing a non-existent tunnel
func TestAPIKillTunnelNotFound(t *testing.T) {
	_, router, _ := setupTestHandler(t)

	req := httptest.NewRequest("POST", "/admin/api/tunnels/nonexistent/kill", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// TestAPIClientsJSON tests JSON clients endpoint
func TestAPIClientsJSON(t *testing.T) {
	_, router, _ := setupTestHandler(t)

	req := httptest.NewRequest("GET", "/admin/api/clients", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "application/json")

	var clients []ClientInfo
	err := json.NewDecoder(w.Body).Decode(&clients)
	assert.NoError(t, err)
	assert.Len(t, clients, 2)
}

// TestAPIClientsHTMX tests HTMX clients fragment endpoint
func TestAPIClientsHTMX(t *testing.T) {
	_, router, _ := setupTestHandler(t)

	req := httptest.NewRequest("GET", "/admin/api/clients", nil)
	req.Header.Set("HX-Request", "true")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "text/html")
	body := w.Body.String()
	assert.Contains(t, body, "client-1")
	assert.Contains(t, body, "client-2")
}

// TestAPIToggleMaintenance tests toggling maintenance mode
func TestAPIToggleMaintenance(t *testing.T) {
	_, router, provider := setupTestHandler(t)

	// Initially off
	assert.False(t, provider.GetMaintenanceMode())

	// Toggle on
	req := httptest.NewRequest("POST", "/admin/api/settings/maintenance", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, provider.GetMaintenanceMode())

	// Toggle off
	req = httptest.NewRequest("POST", "/admin/api/settings/maintenance", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.False(t, provider.GetMaintenanceMode())
}

// TestAPIToggleMaintenanceHTMX tests maintenance toggle returns HTML fragment
func TestAPIToggleMaintenanceHTMX(t *testing.T) {
	_, router, _ := setupTestHandler(t)

	req := httptest.NewRequest("POST", "/admin/api/settings/maintenance", nil)
	req.Header.Set("HX-Request", "true")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "text/html")
	body := w.Body.String()
	assert.Contains(t, body, "enabled")
}

// TestBasicAuthRequired tests basic auth enforcement
func TestBasicAuthRequired(t *testing.T) {
	_, router, _ := setupBasicAuthHandler(t)

	// Without credentials
	req := httptest.NewRequest("GET", "/admin/", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Header().Get("WWW-Authenticate"), "Basic")
}

// TestBasicAuthWrongCredentials tests wrong basic auth credentials
func TestBasicAuthWrongCredentials(t *testing.T) {
	_, router, _ := setupBasicAuthHandler(t)

	req := httptest.NewRequest("GET", "/admin/", nil)
	req.SetBasicAuth("admin", "wrong")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// TestBasicAuthCorrectCredentials tests correct basic auth credentials
func TestBasicAuthCorrectCredentials(t *testing.T) {
	_, router, _ := setupBasicAuthHandler(t)

	req := httptest.NewRequest("GET", "/admin/", nil)
	req.SetBasicAuth("admin", "secret")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Dashboard")
}

// TestBasicAuthEmptyPassword tests that empty password skips auth
func TestBasicAuthEmptyPassword(t *testing.T) {
	provider := newMockProvider()
	adminConfig := &config.AdminConfig{
		Enabled:    true,
		PathPrefix: "/admin",
		AuthMethod: "basic",
		Username:   "admin",
		Password:   "", // empty password = auth disabled
	}
	serverConfig := &config.ServerConfig{
		BindAddress: "0.0.0.0",
		Port:        8443,
		BaseDomain:  "example.com",
	}

	handler, err := NewHandler(adminConfig, serverConfig, provider)
	require.NoError(t, err)

	router := mux.NewRouter()
	handler.RegisterRoutes(router)

	req := httptest.NewRequest("GET", "/admin/", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// TestNoAuthMethod tests "none" auth method allows all access
func TestNoAuthMethod(t *testing.T) {
	_, router, _ := setupTestHandler(t)

	req := httptest.NewRequest("GET", "/admin/", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// TestCSSEndpoint tests the CSS static asset endpoint
func TestCSSEndpoint(t *testing.T) {
	_, router, _ := setupTestHandler(t)

	req := httptest.NewRequest("GET", "/admin/static/style.css", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "text/css", w.Header().Get("Content-Type"))
	assert.Contains(t, w.Body.String(), "--background")
}

// TestJSEndpoint tests the JS static asset endpoint
func TestJSEndpoint(t *testing.T) {
	_, router, _ := setupTestHandler(t)

	req := httptest.NewRequest("GET", "/admin/static/app.js", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/javascript", w.Header().Get("Content-Type"))
	assert.Contains(t, w.Body.String(), "htmx")
}

// TestFormatDuration tests duration formatting
func TestFormatDuration(t *testing.T) {
	tests := []struct {
		duration time.Duration
		expected string
	}{
		{30 * time.Minute, "30m"},
		{2 * time.Hour, "2h 0m"},
		{2*time.Hour + 30*time.Minute, "2h 30m"},
		{26 * time.Hour, "1d 2h 0m"},
		{50*time.Hour + 15*time.Minute, "2d 2h 15m"},
	}

	for _, tc := range tests {
		result := FormatDuration(tc.duration)
		assert.Equal(t, tc.expected, result, "for duration %v", tc.duration)
	}
}

// TestFormatTime tests time formatting
func TestFormatTime(t *testing.T) {
	assert.Equal(t, "Never", FormatTime(time.Time{}))

	now := time.Date(2025, 1, 15, 10, 30, 45, 0, time.UTC)
	assert.Equal(t, "2025-01-15 10:30:45", FormatTime(now))
}

// TestTimeSince tests relative time formatting
func TestTimeSince(t *testing.T) {
	assert.Equal(t, "Never", TimeSince(time.Time{}))
	assert.Equal(t, "just now", TimeSince(time.Now()))
	assert.Contains(t, TimeSince(time.Now().Add(-30*time.Minute)), "m ago")
	assert.Contains(t, TimeSince(time.Now().Add(-3*time.Hour)), "h ago")
	assert.Contains(t, TimeSince(time.Now().Add(-48*time.Hour)), "d ago")
}

// TestEmptyTunnelsPage tests tunnels page with no tunnels
func TestEmptyTunnelsPage(t *testing.T) {
	provider := newMockProvider()
	provider.tunnels = nil
	adminConfig := &config.AdminConfig{
		Enabled:    true,
		PathPrefix: "/admin",
		AuthMethod: "none",
	}
	serverConfig := &config.ServerConfig{
		BindAddress: "0.0.0.0",
		Port:        8443,
		BaseDomain:  "example.com",
	}

	handler, err := NewHandler(adminConfig, serverConfig, provider)
	require.NoError(t, err)

	router := mux.NewRouter()
	handler.RegisterRoutes(router)

	req := httptest.NewRequest("GET", "/admin/tunnels", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "No active tunnels")
}

// TestEmptyClientsPage tests clients page with no clients
func TestEmptyClientsPage(t *testing.T) {
	provider := newMockProvider()
	provider.clients = nil
	adminConfig := &config.AdminConfig{
		Enabled:    true,
		PathPrefix: "/admin",
		AuthMethod: "none",
	}
	serverConfig := &config.ServerConfig{
		BindAddress: "0.0.0.0",
		Port:        8443,
		BaseDomain:  "example.com",
	}

	handler, err := NewHandler(adminConfig, serverConfig, provider)
	require.NoError(t, err)

	router := mux.NewRouter()
	handler.RegisterRoutes(router)

	req := httptest.NewRequest("GET", "/admin/clients", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "No connected clients")
}

// TestCustomPathPrefix tests admin panel with custom path prefix
func TestCustomPathPrefix(t *testing.T) {
	provider := newMockProvider()
	adminConfig := &config.AdminConfig{
		Enabled:    true,
		PathPrefix: "/custom-admin",
		AuthMethod: "none",
	}
	serverConfig := &config.ServerConfig{
		BindAddress: "0.0.0.0",
		Port:        8443,
		BaseDomain:  "example.com",
	}

	handler, err := NewHandler(adminConfig, serverConfig, provider)
	require.NoError(t, err)

	router := mux.NewRouter()
	handler.RegisterRoutes(router)

	req := httptest.NewRequest("GET", "/custom-admin/", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Dashboard")
	// Links should use custom prefix
	assert.Contains(t, w.Body.String(), "/custom-admin/tunnels")
}

// TestDashboardWithRedirectFromBase tests dashboard without trailing slash
func TestDashboardWithRedirectFromBase(t *testing.T) {
	_, router, _ := setupTestHandler(t)

	req := httptest.NewRequest("GET", "/admin", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should either serve the page or redirect
	assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusMovedPermanently)
}
