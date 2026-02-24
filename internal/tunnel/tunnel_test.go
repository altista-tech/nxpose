package tunnel

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestTunnelManager creates a TunnelManager suitable for testing.
// It uses a temporary directory and shuts down background tasks quickly.
func newTestTunnelManager(t *testing.T, maxTunnels, maxPerUser int, maxConnTime string) *TunnelManager {
	t.Helper()
	dir := t.TempDir()
	tm := NewTunnelManager(dir, maxTunnels, maxPerUser, maxConnTime)
	t.Cleanup(func() { tm.Close() })
	return tm
}

// silentLogger returns a logrus logger that discards output.
func silentLogger() *logrus.Logger {
	l := logrus.New()
	l.SetOutput(io.Discard)
	return l
}

// --- NewTunnelManager tests ---

func TestNewTunnelManager_Basic(t *testing.T) {
	dir := t.TempDir()
	tm := NewTunnelManager(dir, 10, 5, "1h")
	defer tm.Close()

	assert.NotNil(t, tm)
	assert.Equal(t, dir, tm.configDir)
	assert.Equal(t, 10, tm.maxTunnels)
	assert.Equal(t, 5, tm.maxTunnelsPerUser)
	assert.Equal(t, time.Hour, tm.maxConnectionTime)
	assert.NotNil(t, tm.tunnels)
	assert.NotNil(t, tm.userTunnels)
}

func TestNewTunnelManager_ZeroLimits(t *testing.T) {
	tm := newTestTunnelManager(t, 0, 0, "")

	assert.Equal(t, 0, tm.maxTunnels)
	assert.Equal(t, 0, tm.maxTunnelsPerUser)
	assert.Equal(t, time.Duration(0), tm.maxConnectionTime)
}

func TestNewTunnelManager_InvalidDuration(t *testing.T) {
	tm := newTestTunnelManager(t, 0, 0, "invalid-duration")

	// Should fall back to zero (no limit) on invalid duration
	assert.Equal(t, time.Duration(0), tm.maxConnectionTime)
}

func TestNewTunnelManager_VariousDurations(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected time.Duration
	}{
		{"minutes", "30m", 30 * time.Minute},
		{"seconds", "90s", 90 * time.Second},
		{"hours", "2h", 2 * time.Hour},
		{"complex", "1h30m", 90 * time.Minute},
		{"empty", "", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tm := newTestTunnelManager(t, 0, 0, tt.input)
			assert.Equal(t, tt.expected, tm.maxConnectionTime)
		})
	}
}

func TestNewTunnelManager_CreatesConfigDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "config")
	tm := NewTunnelManager(dir, 0, 0, "")
	defer tm.Close()

	assert.DirExists(t, dir)
}

func TestNewTunnelManager_EmptyConfigDir(t *testing.T) {
	tm := NewTunnelManager("", 0, 0, "")
	defer tm.Close()

	assert.Equal(t, "", tm.configDir)
}

// --- CreateTunnel tests ---

func TestCreateTunnel_HTTP(t *testing.T) {
	tm := newTestTunnelManager(t, 0, 0, "")

	info, err := tm.CreateTunnel("http", 8080, "example.com", 443, nil, "user1")
	require.NoError(t, err)
	require.NotNil(t, info)

	assert.NotEmpty(t, info.ID)
	assert.Equal(t, "http", info.Protocol)
	assert.Equal(t, 8080, info.LocalPort)
	assert.Equal(t, "example.com", info.ServerHost)
	assert.Equal(t, 443, info.ServerPort)
	assert.Equal(t, "user1", info.UserID)
	assert.Contains(t, info.PublicURL, "http://")
	assert.Contains(t, info.PublicURL, "example.com")
	assert.False(t, info.Created.IsZero())
	assert.False(t, info.LastActive.IsZero())
}

func TestCreateTunnel_HTTPS(t *testing.T) {
	tm := newTestTunnelManager(t, 0, 0, "")

	info, err := tm.CreateTunnel("https", 8443, "example.com", 443, []byte("cert"), "user1")
	require.NoError(t, err)

	assert.Contains(t, info.PublicURL, "https://")
	assert.Contains(t, info.PublicURL, "-s.example.com")
}

func TestCreateTunnel_TCP(t *testing.T) {
	tm := newTestTunnelManager(t, 0, 0, "")

	info, err := tm.CreateTunnel("tcp", 3306, "example.com", 9000, nil, "user1")
	require.NoError(t, err)

	assert.Equal(t, "tcp://example.com:9000", info.PublicURL)
}

func TestCreateTunnel_UnsupportedProtocol(t *testing.T) {
	tm := newTestTunnelManager(t, 0, 0, "")

	_, err := tm.CreateTunnel("udp", 8080, "example.com", 443, nil, "user1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported protocol")
}

func TestCreateTunnel_DuplicatePort(t *testing.T) {
	tm := newTestTunnelManager(t, 0, 0, "")

	_, err := tm.CreateTunnel("http", 8080, "example.com", 443, nil, "user1")
	require.NoError(t, err)

	_, err = tm.CreateTunnel("http", 8080, "example.com", 443, nil, "user2")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tunnel already exists")
}

func TestCreateTunnel_DifferentProtocolSamePort(t *testing.T) {
	tm := newTestTunnelManager(t, 0, 0, "")

	_, err := tm.CreateTunnel("http", 8080, "example.com", 443, nil, "user1")
	require.NoError(t, err)

	// Same port but different protocol should succeed
	_, err = tm.CreateTunnel("tcp", 8080, "example.com", 9000, nil, "user1")
	require.NoError(t, err)
}

func TestCreateTunnel_MaxTunnelsLimit(t *testing.T) {
	tm := newTestTunnelManager(t, 2, 0, "")

	_, err := tm.CreateTunnel("http", 8080, "example.com", 443, nil, "user1")
	require.NoError(t, err)

	_, err = tm.CreateTunnel("http", 8081, "example.com", 443, nil, "user2")
	require.NoError(t, err)

	// Third tunnel should fail
	_, err = tm.CreateTunnel("http", 8082, "example.com", 443, nil, "user3")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "maximum number of tunnels reached")
}

func TestCreateTunnel_MaxTunnelsPerUserLimit(t *testing.T) {
	tm := newTestTunnelManager(t, 0, 1, "")

	_, err := tm.CreateTunnel("http", 8080, "example.com", 443, nil, "user1")
	require.NoError(t, err)

	// Second tunnel for same user should fail
	_, err = tm.CreateTunnel("http", 8081, "example.com", 443, nil, "user1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "maximum number of tunnels per user reached")

	// Different user should succeed
	_, err = tm.CreateTunnel("http", 8081, "example.com", 443, nil, "user2")
	require.NoError(t, err)
}

func TestCreateTunnel_EmptyUserBypassesPerUserLimit(t *testing.T) {
	tm := newTestTunnelManager(t, 0, 1, "")

	// Empty userID should bypass per-user limit checks
	_, err := tm.CreateTunnel("http", 8080, "example.com", 443, nil, "")
	require.NoError(t, err)

	_, err = tm.CreateTunnel("http", 8081, "example.com", 443, nil, "")
	require.NoError(t, err)
}

func TestCreateTunnel_SetsExpiration(t *testing.T) {
	tm := newTestTunnelManager(t, 0, 0, "1h")

	before := time.Now()
	info, err := tm.CreateTunnel("http", 8080, "example.com", 443, nil, "user1")
	require.NoError(t, err)

	// ExpiresAt should be roughly 1 hour from now
	assert.False(t, info.ExpiresAt.IsZero())
	assert.True(t, info.ExpiresAt.After(before.Add(59*time.Minute)))
	assert.True(t, info.ExpiresAt.Before(before.Add(61*time.Minute)))
}

func TestCreateTunnel_NoExpirationWhenZeroDuration(t *testing.T) {
	tm := newTestTunnelManager(t, 0, 0, "")

	info, err := tm.CreateTunnel("http", 8080, "example.com", 443, nil, "user1")
	require.NoError(t, err)

	assert.True(t, info.ExpiresAt.IsZero())
}

// --- GetTunnel tests ---

func TestGetTunnel_Found(t *testing.T) {
	tm := newTestTunnelManager(t, 0, 0, "")

	created, err := tm.CreateTunnel("http", 8080, "example.com", 443, nil, "user1")
	require.NoError(t, err)

	info, found := tm.GetTunnel(created.ID)
	assert.True(t, found)
	assert.Equal(t, created.ID, info.ID)
	assert.Equal(t, created.Protocol, info.Protocol)
	assert.Equal(t, created.LocalPort, info.LocalPort)
}

func TestGetTunnel_NotFound(t *testing.T) {
	tm := newTestTunnelManager(t, 0, 0, "")

	info, found := tm.GetTunnel("nonexistent-id")
	assert.False(t, found)
	assert.Nil(t, info)
}

// --- GetTunnelByPort tests ---

func TestGetTunnelByPort_Found(t *testing.T) {
	tm := newTestTunnelManager(t, 0, 0, "")

	_, err := tm.CreateTunnel("http", 8080, "example.com", 443, nil, "user1")
	require.NoError(t, err)

	info, found := tm.GetTunnelByPort("http", 8080)
	assert.True(t, found)
	assert.Equal(t, "http", info.Protocol)
	assert.Equal(t, 8080, info.LocalPort)
}

func TestGetTunnelByPort_NotFound(t *testing.T) {
	tm := newTestTunnelManager(t, 0, 0, "")

	_, found := tm.GetTunnelByPort("http", 9999)
	assert.False(t, found)
}

func TestGetTunnelByPort_WrongProtocol(t *testing.T) {
	tm := newTestTunnelManager(t, 0, 0, "")

	_, err := tm.CreateTunnel("http", 8080, "example.com", 443, nil, "user1")
	require.NoError(t, err)

	_, found := tm.GetTunnelByPort("tcp", 8080)
	assert.False(t, found)
}

// --- GetTunnelsByUserID tests ---

func TestGetTunnelsByUserID_WithTunnels(t *testing.T) {
	tm := newTestTunnelManager(t, 0, 0, "")

	_, err := tm.CreateTunnel("http", 8080, "example.com", 443, nil, "user1")
	require.NoError(t, err)
	_, err = tm.CreateTunnel("http", 8081, "example.com", 443, nil, "user1")
	require.NoError(t, err)
	_, err = tm.CreateTunnel("http", 8082, "example.com", 443, nil, "user2")
	require.NoError(t, err)

	user1Tunnels := tm.GetTunnelsByUserID("user1")
	assert.Len(t, user1Tunnels, 2)

	user2Tunnels := tm.GetTunnelsByUserID("user2")
	assert.Len(t, user2Tunnels, 1)
}

func TestGetTunnelsByUserID_NoTunnels(t *testing.T) {
	tm := newTestTunnelManager(t, 0, 0, "")

	tunnels := tm.GetTunnelsByUserID("nonexistent-user")
	assert.Empty(t, tunnels)
}

// --- ListTunnels tests ---

func TestListTunnels_Empty(t *testing.T) {
	tm := newTestTunnelManager(t, 0, 0, "")

	tunnels := tm.ListTunnels()
	assert.Empty(t, tunnels)
}

func TestListTunnels_WithTunnels(t *testing.T) {
	tm := newTestTunnelManager(t, 0, 0, "")

	_, err := tm.CreateTunnel("http", 8080, "example.com", 443, nil, "user1")
	require.NoError(t, err)
	_, err = tm.CreateTunnel("tcp", 3306, "example.com", 9000, nil, "user2")
	require.NoError(t, err)

	tunnels := tm.ListTunnels()
	assert.Len(t, tunnels, 2)
}

// --- RemoveTunnel tests ---

func TestRemoveTunnel_Exists(t *testing.T) {
	tm := newTestTunnelManager(t, 0, 0, "")

	info, err := tm.CreateTunnel("http", 8080, "example.com", 443, nil, "user1")
	require.NoError(t, err)

	removed := tm.RemoveTunnel(info.ID)
	assert.True(t, removed)

	// Verify it's actually gone
	_, found := tm.GetTunnel(info.ID)
	assert.False(t, found)

	// Verify user tunnel tracking is updated
	userTunnels := tm.GetTunnelsByUserID("user1")
	assert.Empty(t, userTunnels)
}

func TestRemoveTunnel_NotExists(t *testing.T) {
	tm := newTestTunnelManager(t, 0, 0, "")

	removed := tm.RemoveTunnel("nonexistent-id")
	assert.False(t, removed)
}

func TestRemoveTunnel_FreesUserSlot(t *testing.T) {
	tm := newTestTunnelManager(t, 0, 1, "")

	info, err := tm.CreateTunnel("http", 8080, "example.com", 443, nil, "user1")
	require.NoError(t, err)

	// User at limit
	_, err = tm.CreateTunnel("http", 8081, "example.com", 443, nil, "user1")
	require.Error(t, err)

	// Remove the first tunnel
	tm.RemoveTunnel(info.ID)

	// Now user should be able to create another
	_, err = tm.CreateTunnel("http", 8081, "example.com", 443, nil, "user1")
	require.NoError(t, err)
}

func TestRemoveTunnel_FreesGlobalSlot(t *testing.T) {
	tm := newTestTunnelManager(t, 1, 0, "")

	info, err := tm.CreateTunnel("http", 8080, "example.com", 443, nil, "user1")
	require.NoError(t, err)

	_, err = tm.CreateTunnel("http", 8081, "example.com", 443, nil, "user2")
	require.Error(t, err)

	tm.RemoveTunnel(info.ID)

	_, err = tm.CreateTunnel("http", 8081, "example.com", 443, nil, "user2")
	require.NoError(t, err)
}

// --- RestartTunnel tests ---

func TestRestartTunnel_NotFound(t *testing.T) {
	tm := newTestTunnelManager(t, 0, 0, "")

	err := tm.RestartTunnel("nonexistent-id")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tunnel not found")
}

func TestRestartTunnel_Exists(t *testing.T) {
	tm := newTestTunnelManager(t, 0, 0, "")

	info, err := tm.CreateTunnel("http", 8080, "example.com", 443, nil, "user1")
	require.NoError(t, err)

	// RestartTunnel should not error for an existing tunnel
	err = tm.RestartTunnel(info.ID)
	require.NoError(t, err)
}

// --- Tunnel expiration and cleanup tests ---

func TestCleanupStaleTunnels_RemovesExpired(t *testing.T) {
	tm := newTestTunnelManager(t, 0, 0, "")

	// Directly insert a tunnel with an expired time
	expiredTunnel := &ManagedTunnel{
		ID:            "expired-tunnel",
		UserID:        "user1",
		Protocol:      "http",
		LocalPort:     8080,
		PublicURL:     "http://test.example.com",
		Created:       time.Now().Add(-2 * time.Hour),
		LastActive:    time.Now().Add(-1 * time.Hour),
		Active:        false,
		ExpiresAt:     time.Now().Add(-30 * time.Minute), // Expired 30 min ago
		reconnectChan: make(chan struct{}, 1),
		stopChan:      make(chan struct{}),
	}

	tm.mu.Lock()
	tm.tunnels["expired-tunnel"] = expiredTunnel
	tm.userTunnels["user1"] = map[string]struct{}{"expired-tunnel": {}}
	tm.mu.Unlock()

	// Run cleanup
	tm.cleanupStaleTunnels()

	// Verify tunnel was removed
	_, found := tm.GetTunnel("expired-tunnel")
	assert.False(t, found)

	// Verify user tracking was cleaned up
	userTunnels := tm.GetTunnelsByUserID("user1")
	assert.Empty(t, userTunnels)
}

func TestCleanupStaleTunnels_KeepsNonExpired(t *testing.T) {
	tm := newTestTunnelManager(t, 0, 0, "")

	// Insert a tunnel that expires in the future
	futureTunnel := &ManagedTunnel{
		ID:            "future-tunnel",
		UserID:        "user1",
		Protocol:      "http",
		LocalPort:     8080,
		PublicURL:     "http://test.example.com",
		Created:       time.Now(),
		LastActive:    time.Now(),
		Active:        true,
		ExpiresAt:     time.Now().Add(1 * time.Hour),
		reconnectChan: make(chan struct{}, 1),
		stopChan:      make(chan struct{}),
	}

	tm.mu.Lock()
	tm.tunnels["future-tunnel"] = futureTunnel
	tm.mu.Unlock()

	tm.cleanupStaleTunnels()

	_, found := tm.GetTunnel("future-tunnel")
	assert.True(t, found)
}

func TestCleanupStaleTunnels_KeepsNoExpirationTunnels(t *testing.T) {
	tm := newTestTunnelManager(t, 0, 0, "")

	// Insert a tunnel with zero expiration (no limit)
	noExpiryTunnel := &ManagedTunnel{
		ID:            "no-expiry",
		UserID:        "user1",
		Protocol:      "http",
		LocalPort:     8080,
		PublicURL:     "http://test.example.com",
		Created:       time.Now().Add(-24 * time.Hour),
		LastActive:    time.Now().Add(-12 * time.Hour),
		Active:        false,
		ExpiresAt:     time.Time{}, // Zero value = no expiration
		reconnectChan: make(chan struct{}, 1),
		stopChan:      make(chan struct{}),
	}

	tm.mu.Lock()
	tm.tunnels["no-expiry"] = noExpiryTunnel
	tm.mu.Unlock()

	tm.cleanupStaleTunnels()

	_, found := tm.GetTunnel("no-expiry")
	assert.True(t, found)
}

func TestCleanupStaleTunnels_CleansUpUserTrackingWhenLastTunnel(t *testing.T) {
	tm := newTestTunnelManager(t, 0, 0, "")

	// Insert two tunnels for user1, only one expired
	expired := &ManagedTunnel{
		ID:            "expired-1",
		UserID:        "user1",
		Protocol:      "http",
		LocalPort:     8080,
		PublicURL:     "http://test.example.com",
		Created:       time.Now(),
		ExpiresAt:     time.Now().Add(-1 * time.Minute),
		reconnectChan: make(chan struct{}, 1),
		stopChan:      make(chan struct{}),
	}
	alive := &ManagedTunnel{
		ID:            "alive-1",
		UserID:        "user1",
		Protocol:      "http",
		LocalPort:     8081,
		PublicURL:     "http://test2.example.com",
		Created:       time.Now(),
		ExpiresAt:     time.Now().Add(1 * time.Hour),
		reconnectChan: make(chan struct{}, 1),
		stopChan:      make(chan struct{}),
	}

	tm.mu.Lock()
	tm.tunnels["expired-1"] = expired
	tm.tunnels["alive-1"] = alive
	tm.userTunnels["user1"] = map[string]struct{}{
		"expired-1": {},
		"alive-1":   {},
	}
	tm.mu.Unlock()

	tm.cleanupStaleTunnels()

	// Expired should be gone
	_, found := tm.GetTunnel("expired-1")
	assert.False(t, found)

	// Alive should remain
	_, found = tm.GetTunnel("alive-1")
	assert.True(t, found)

	// User should still exist in userTunnels with one tunnel
	userTunnels := tm.GetTunnelsByUserID("user1")
	assert.Len(t, userTunnels, 1)
}

// --- SaveTunnels / LoadTunnels tests ---

func TestSaveTunnels_Success(t *testing.T) {
	tm := newTestTunnelManager(t, 0, 0, "")

	_, err := tm.CreateTunnel("http", 8080, "example.com", 443, nil, "user1")
	require.NoError(t, err)

	err = tm.SaveTunnels()
	require.NoError(t, err)

	// Verify file exists
	configPath := filepath.Join(tm.configDir, "tunnels.json")
	assert.FileExists(t, configPath)

	// Verify it's valid JSON
	data, err := os.ReadFile(configPath)
	require.NoError(t, err)

	var saveData TunnelSaveData
	err = json.Unmarshal(data, &saveData)
	require.NoError(t, err)
	assert.Len(t, saveData.Tunnels, 1)
	assert.Equal(t, "http", saveData.Tunnels[0].Protocol)
	assert.Equal(t, 8080, saveData.Tunnels[0].LocalPort)
}

func TestSaveTunnels_WithCertData(t *testing.T) {
	tm := newTestTunnelManager(t, 0, 0, "")

	certData := []byte("test-certificate-data")
	_, err := tm.CreateTunnel("https", 8443, "example.com", 443, certData, "user1")
	require.NoError(t, err)

	err = tm.SaveTunnels()
	require.NoError(t, err)

	// Check that cert file was written
	files, err := filepath.Glob(filepath.Join(tm.configDir, "cert_*.pem"))
	require.NoError(t, err)
	assert.Len(t, files, 1)

	// Verify cert content
	savedCert, err := os.ReadFile(files[0])
	require.NoError(t, err)
	assert.Equal(t, certData, savedCert)
}

func TestSaveTunnels_NoConfigDir(t *testing.T) {
	tm := NewTunnelManager("", 0, 0, "")
	defer tm.Close()

	err := tm.SaveTunnels()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no config directory")
}

func TestLoadTunnels_NoConfigDir(t *testing.T) {
	tm := NewTunnelManager("", 0, 0, "")
	defer tm.Close()

	err := tm.LoadTunnels()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no config directory")
}

func TestLoadTunnels_NoSavedFile(t *testing.T) {
	tm := newTestTunnelManager(t, 0, 0, "")

	// Should not error if no file exists
	err := tm.LoadTunnels()
	require.NoError(t, err)
}

func TestLoadTunnels_InvalidJSON(t *testing.T) {
	tm := newTestTunnelManager(t, 0, 0, "")

	// Write invalid JSON
	configPath := filepath.Join(tm.configDir, "tunnels.json")
	err := os.WriteFile(configPath, []byte("not-valid-json"), 0644)
	require.NoError(t, err)

	err = tm.LoadTunnels()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to unmarshal")
}

func TestSaveAndLoadTunnels_RoundTrip(t *testing.T) {
	dir := t.TempDir()

	// Create and populate manager
	tm1 := NewTunnelManager(dir, 0, 0, "")

	info1, err := tm1.CreateTunnel("http", 8080, "example.com", 443, nil, "user1")
	require.NoError(t, err)
	info2, err := tm1.CreateTunnel("tcp", 3306, "example.com", 9000, nil, "user2")
	require.NoError(t, err)

	// Save
	err = tm1.SaveTunnels()
	require.NoError(t, err)
	tm1.Close()

	// Load into new manager
	tm2 := NewTunnelManager(dir, 0, 0, "")
	defer tm2.Close()

	err = tm2.LoadTunnels()
	require.NoError(t, err)

	// Give async tunnel starts a moment to register
	time.Sleep(100 * time.Millisecond)

	// Verify loaded tunnels
	loaded1, found := tm2.GetTunnel(info1.ID)
	assert.True(t, found)
	assert.Equal(t, "http", loaded1.Protocol)
	assert.Equal(t, 8080, loaded1.LocalPort)

	loaded2, found := tm2.GetTunnel(info2.ID)
	assert.True(t, found)
	assert.Equal(t, "tcp", loaded2.Protocol)
	assert.Equal(t, 3306, loaded2.LocalPort)
}

// --- StopAllTunnels / StartAllTunnels tests ---

func TestStopAllTunnels_Empty(t *testing.T) {
	tm := newTestTunnelManager(t, 0, 0, "")
	count := tm.StopAllTunnels()
	assert.Equal(t, 0, count)
}

func TestStartAllTunnels_Empty(t *testing.T) {
	tm := newTestTunnelManager(t, 0, 0, "")
	count := tm.StartAllTunnels()
	assert.Equal(t, 0, count)
}

// --- Close tests ---

func TestClose_ClearsAllTunnels(t *testing.T) {
	dir := t.TempDir()
	tm := NewTunnelManager(dir, 0, 0, "")

	_, err := tm.CreateTunnel("http", 8080, "example.com", 443, nil, "user1")
	require.NoError(t, err)

	err = tm.Close()
	require.NoError(t, err)

	tunnels := tm.ListTunnels()
	assert.Empty(t, tunnels)
}

// --- SetRedisClient tests ---

func TestSetRedisClient(t *testing.T) {
	tm := newTestTunnelManager(t, 0, 0, "")

	assert.Nil(t, tm.redisClient)

	mockRedis := "mock-redis-client"
	tm.SetRedisClient(mockRedis)
	assert.Equal(t, mockRedis, tm.redisClient)
}

// --- getTunnelInfo tests ---

func TestGetTunnelInfo_ReturnsCorrectFields(t *testing.T) {
	tm := newTestTunnelManager(t, 0, 0, "1h")

	created, err := tm.CreateTunnel("http", 8080, "example.com", 443, nil, "user1")
	require.NoError(t, err)

	info, found := tm.GetTunnel(created.ID)
	require.True(t, found)

	assert.Equal(t, created.ID, info.ID)
	assert.Equal(t, "user1", info.UserID)
	assert.Equal(t, "http", info.Protocol)
	assert.Equal(t, 8080, info.LocalPort)
	assert.Equal(t, "example.com", info.ServerHost)
	assert.Equal(t, 443, info.ServerPort)
	assert.False(t, info.ExpiresAt.IsZero())
}

// --- Concurrent operations tests ---

func TestConcurrentCreateAndList(t *testing.T) {
	tm := newTestTunnelManager(t, 0, 0, "")

	var wg sync.WaitGroup
	tunnelCount := 20

	// Create tunnels concurrently
	for i := 0; i < tunnelCount; i++ {
		wg.Add(1)
		go func(port int) {
			defer wg.Done()
			_, _ = tm.CreateTunnel("http", 8000+port, "example.com", 443, nil, "user1")
		}(i)
	}

	wg.Wait()

	tunnels := tm.ListTunnels()
	assert.Equal(t, tunnelCount, len(tunnels))
}

func TestConcurrentCreateAndRemove(t *testing.T) {
	tm := newTestTunnelManager(t, 0, 0, "")

	var ids []string
	var mu sync.Mutex

	// Create some tunnels first
	for i := 0; i < 10; i++ {
		info, err := tm.CreateTunnel("http", 8000+i, "example.com", 443, nil, "user1")
		require.NoError(t, err)
		ids = append(ids, info.ID)
	}

	var wg sync.WaitGroup

	// Concurrently remove and create tunnels
	for i := 0; i < 10; i++ {
		wg.Add(2)

		// Remove existing
		go func(idx int) {
			defer wg.Done()
			mu.Lock()
			id := ids[idx]
			mu.Unlock()
			tm.RemoveTunnel(id)
		}(i)

		// Create new
		go func(port int) {
			defer wg.Done()
			_, _ = tm.CreateTunnel("http", 9000+port, "example.com", 443, nil, "user2")
		}(i)
	}

	wg.Wait()

	// All operations should complete without data races
	tunnels := tm.ListTunnels()
	assert.NotNil(t, tunnels)
}

func TestConcurrentGetAndCreate(t *testing.T) {
	tm := newTestTunnelManager(t, 0, 0, "")

	var wg sync.WaitGroup

	// Concurrent reads and writes
	for i := 0; i < 20; i++ {
		wg.Add(2)

		go func(port int) {
			defer wg.Done()
			_, _ = tm.CreateTunnel("http", 7000+port, "example.com", 443, nil, "user1")
		}(i)

		go func() {
			defer wg.Done()
			tm.ListTunnels()
			tm.GetTunnelsByUserID("user1")
			tm.GetTunnelByPort("http", 7000)
		}()
	}

	wg.Wait()
}

func TestConcurrentCleanupAndCreate(t *testing.T) {
	tm := newTestTunnelManager(t, 0, 0, "")

	// Add some expired tunnels directly
	tm.mu.Lock()
	for i := 0; i < 5; i++ {
		id := "expired-" + string(rune('a'+i))
		tm.tunnels[id] = &ManagedTunnel{
			ID:            id,
			Protocol:      "http",
			LocalPort:     5000 + i,
			ExpiresAt:     time.Now().Add(-1 * time.Hour),
			reconnectChan: make(chan struct{}, 1),
			stopChan:      make(chan struct{}),
		}
	}
	tm.mu.Unlock()

	var wg sync.WaitGroup

	// Run cleanup and create concurrently
	wg.Add(2)
	go func() {
		defer wg.Done()
		tm.cleanupStaleTunnels()
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < 5; i++ {
			_, _ = tm.CreateTunnel("http", 6000+i, "example.com", 443, nil, "user1")
		}
	}()

	wg.Wait()
}

// --- Message type serialization tests ---

func TestTunnelMessage_Serialization(t *testing.T) {
	msg := TunnelMessage{
		Type:      "http_request",
		RequestID: "req-123",
		TunnelID:  "tunnel-456",
		Data:      json.RawMessage(`{"key":"value"}`),
	}

	data, err := json.Marshal(msg)
	require.NoError(t, err)

	var decoded TunnelMessage
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, msg.Type, decoded.Type)
	assert.Equal(t, msg.RequestID, decoded.RequestID)
	assert.Equal(t, msg.TunnelID, decoded.TunnelID)
	assert.JSONEq(t, `{"key":"value"}`, string(decoded.Data))
}

func TestHTTPRequest_Serialization(t *testing.T) {
	req := HTTPRequest{
		Method:  "POST",
		Path:    "/api/data",
		Query:   "foo=bar",
		Headers: map[string]string{"Content-Type": "application/json"},
		Body:    []byte(`{"hello":"world"}`),
	}

	data, err := json.Marshal(req)
	require.NoError(t, err)

	var decoded HTTPRequest
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, req.Method, decoded.Method)
	assert.Equal(t, req.Path, decoded.Path)
	assert.Equal(t, req.Query, decoded.Query)
	assert.Equal(t, req.Headers, decoded.Headers)
	assert.Equal(t, req.Body, decoded.Body)
}

func TestHTTPResponse_Serialization(t *testing.T) {
	resp := HTTPResponse{
		StatusCode: 200,
		Headers:    map[string]string{"Content-Type": "text/plain"},
		Body:       []byte("OK"),
	}

	data, err := json.Marshal(resp)
	require.NoError(t, err)

	var decoded HTTPResponse
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, resp.StatusCode, decoded.StatusCode)
	assert.Equal(t, resp.Headers, decoded.Headers)
	assert.Equal(t, resp.Body, decoded.Body)
}

func TestTCPMessage_Serialization(t *testing.T) {
	msg := TCPMessage{
		ConnectionID: "conn-123",
		Data:         []byte("tcp-payload-data"),
	}

	data, err := json.Marshal(msg)
	require.NoError(t, err)

	var decoded TCPMessage
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, msg.ConnectionID, decoded.ConnectionID)
	assert.Equal(t, msg.Data, decoded.Data)
}

// --- Tunnel struct tests ---

func TestTunnel_Stop_NotRunning(t *testing.T) {
	tunnel := &Tunnel{
		ID:      "test-tunnel",
		running: false,
		stopCh:  make(chan struct{}),
		log:     silentLogger(),
	}

	err := tunnel.Stop()
	assert.NoError(t, err) // Stop on non-running tunnel should be a no-op
}

func TestTunnel_Stop_Running(t *testing.T) {
	tunnel := &Tunnel{
		ID:      "test-tunnel",
		running: true,
		stopCh:  make(chan struct{}),
		log:     silentLogger(),
	}

	err := tunnel.Stop()
	assert.NoError(t, err)
	assert.False(t, tunnel.running)
	assert.False(t, tunnel.registered)
}

func TestTunnel_Start_AlreadyRunning(t *testing.T) {
	tunnel := &Tunnel{
		ID:      "test-tunnel",
		running: true,
		stopCh:  make(chan struct{}),
		log:     silentLogger(),
	}

	err := tunnel.Start()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already running")
}

func TestTunnel_SendMessage_NilConnection(t *testing.T) {
	tunnel := &Tunnel{
		ID:     "test-tunnel",
		wsConn: nil,
		log:    silentLogger(),
	}

	msg := TunnelMessage{Type: "ping"}
	err := tunnel.sendMessage(msg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "websocket connection is nil")
}

// --- generateSubdomain tests ---

func TestGenerateSubdomain(t *testing.T) {
	s1 := generateSubdomain()
	s2 := generateSubdomain()

	assert.Len(t, s1, 8)
	assert.Len(t, s2, 8)
	assert.NotEqual(t, s1, s2, "subdomains should be unique")
}
