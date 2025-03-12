package tunnel

import (
	"encoding/json"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestLogger returns a logger that discards output.
func newTestLogger() *logrus.Logger {
	l := logrus.New()
	l.SetOutput(io.Discard)
	return l
}

// newTestTunnel creates a minimal Tunnel suitable for TCP tunnel tests.
func newTestTunnel() *Tunnel {
	return &Tunnel{
		ID:     "test-tunnel-id",
		log:    newTestLogger(),
		stopCh: make(chan struct{}),
	}
}

// --- NewTCPTunnel tests ---

func TestNewTCPTunnel_Basic(t *testing.T) {
	parent := newTestTunnel()
	logger := newTestLogger()

	tcp := NewTCPTunnel("tunnel-1", 8080, "remote:9090", parent, logger)

	assert.Equal(t, "tunnel-1", tcp.ID)
	assert.Equal(t, 8080, tcp.LocalPort)
	assert.Equal(t, "remote:9090", tcp.RemoteAddr)
	assert.Equal(t, 100, tcp.MaxConnections) // Default
	assert.NotNil(t, tcp.connections)
	assert.Empty(t, tcp.connections)
	assert.Equal(t, parent, tcp.tunnel)
	assert.Equal(t, logger, tcp.log)
	assert.False(t, tcp.stopped)
}

// --- TCPTunnel Start / Stop lifecycle tests ---

func TestTCPTunnel_Start_Success(t *testing.T) {
	parent := newTestTunnel()
	logger := newTestLogger()

	// Use port 0 so the OS assigns a free port
	tcp := NewTCPTunnel("tunnel-1", 0, "remote:9090", parent, logger)

	err := tcp.Start()
	require.NoError(t, err)
	defer tcp.Stop()

	assert.NotNil(t, tcp.listener)
	assert.False(t, tcp.stopped)
}

func TestTCPTunnel_Start_AlreadyRunning(t *testing.T) {
	parent := newTestTunnel()
	logger := newTestLogger()

	tcp := NewTCPTunnel("tunnel-1", 0, "remote:9090", parent, logger)

	err := tcp.Start()
	require.NoError(t, err)
	defer tcp.Stop()

	// Second start should fail
	err = tcp.Start()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already running")
}

func TestTCPTunnel_Start_AfterStopped(t *testing.T) {
	parent := newTestTunnel()
	logger := newTestLogger()

	tcp := NewTCPTunnel("tunnel-1", 0, "remote:9090", parent, logger)

	err := tcp.Start()
	require.NoError(t, err)
	tcp.Stop()

	// Starting after stop should fail
	err = tcp.Start()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "stopped")
}

func TestTCPTunnel_Stop_Idempotent(t *testing.T) {
	parent := newTestTunnel()
	logger := newTestLogger()

	tcp := NewTCPTunnel("tunnel-1", 0, "remote:9090", parent, logger)

	err := tcp.Start()
	require.NoError(t, err)

	// First stop
	err = tcp.Stop()
	assert.NoError(t, err)
	assert.True(t, tcp.stopped)

	// Second stop should be a no-op
	err = tcp.Stop()
	assert.NoError(t, err)
}

func TestTCPTunnel_Stop_ClosesConnections(t *testing.T) {
	parent := newTestTunnel()
	logger := newTestLogger()

	tcp := NewTCPTunnel("tunnel-1", 0, "remote:9090", parent, logger)

	err := tcp.Start()
	require.NoError(t, err)

	// Create a mock connection using net.Pipe
	serverConn, clientConn := net.Pipe()
	defer clientConn.Close()

	conn := &TCPConnection{
		ID:           "conn-1",
		TunnelID:     tcp.ID,
		LocalConn:    serverConn,
		RemoteAddr:   "127.0.0.1:12345",
		CreatedAt:    time.Now(),
		LastActivity: time.Now(),
	}

	tcp.mu.Lock()
	tcp.connections["conn-1"] = conn
	tcp.mu.Unlock()

	// Stop should close connections
	tcp.Stop()

	tcp.mu.RLock()
	assert.Empty(t, tcp.connections)
	tcp.mu.RUnlock()
}

// --- HandleRemoteData tests ---

func TestHandleRemoteData_Success(t *testing.T) {
	parent := newTestTunnel()
	logger := newTestLogger()

	tcp := NewTCPTunnel("tunnel-1", 0, "remote:9090", parent, logger)

	// Create a pipe to simulate a connection
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	conn := &TCPConnection{
		ID:           "conn-1",
		TunnelID:     tcp.ID,
		LocalConn:    serverConn,
		RemoteAddr:   "127.0.0.1:12345",
		CreatedAt:    time.Now(),
		LastActivity: time.Now(),
	}

	tcp.mu.Lock()
	tcp.connections["conn-1"] = conn
	tcp.mu.Unlock()

	// Write data from remote
	testData := []byte("hello from remote")

	// Read in a goroutine
	done := make(chan []byte)
	go func() {
		buf := make([]byte, 1024)
		n, _ := clientConn.Read(buf)
		done <- buf[:n]
	}()

	err := tcp.HandleRemoteData("conn-1", testData)
	require.NoError(t, err)

	received := <-done
	assert.Equal(t, testData, received)
}

func TestHandleRemoteData_ConnectionNotFound(t *testing.T) {
	parent := newTestTunnel()
	logger := newTestLogger()

	tcp := NewTCPTunnel("tunnel-1", 0, "remote:9090", parent, logger)

	err := tcp.HandleRemoteData("nonexistent-conn", []byte("data"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestHandleRemoteData_ClosedConnection(t *testing.T) {
	parent := newTestTunnel()
	logger := newTestLogger()

	tcp := NewTCPTunnel("tunnel-1", 0, "remote:9090", parent, logger)

	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	conn := &TCPConnection{
		ID:           "conn-1",
		TunnelID:     tcp.ID,
		LocalConn:    serverConn,
		RemoteAddr:   "127.0.0.1:12345",
		CreatedAt:    time.Now(),
		LastActivity: time.Now(),
		closed:       true, // Already closed
	}

	tcp.mu.Lock()
	tcp.connections["conn-1"] = conn
	tcp.mu.Unlock()

	err := tcp.HandleRemoteData("conn-1", []byte("data"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "closed")
}

func TestHandleRemoteData_UpdatesActivity(t *testing.T) {
	parent := newTestTunnel()
	logger := newTestLogger()

	tcp := NewTCPTunnel("tunnel-1", 0, "remote:9090", parent, logger)

	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	oldTime := time.Now().Add(-1 * time.Hour)
	conn := &TCPConnection{
		ID:           "conn-1",
		TunnelID:     tcp.ID,
		LocalConn:    serverConn,
		RemoteAddr:   "127.0.0.1:12345",
		CreatedAt:    oldTime,
		LastActivity: oldTime,
	}

	tcp.mu.Lock()
	tcp.connections["conn-1"] = conn
	tcp.mu.Unlock()

	// Read asynchronously to prevent pipe blocking
	go func() {
		buf := make([]byte, 1024)
		clientConn.Read(buf)
	}()

	err := tcp.HandleRemoteData("conn-1", []byte("data"))
	require.NoError(t, err)

	conn.mu.Lock()
	assert.True(t, conn.LastActivity.After(oldTime))
	conn.mu.Unlock()
}

// --- GetConnections tests ---

func TestGetConnections_Empty(t *testing.T) {
	parent := newTestTunnel()
	logger := newTestLogger()

	tcp := NewTCPTunnel("tunnel-1", 0, "remote:9090", parent, logger)

	conns := tcp.GetConnections()
	assert.Empty(t, conns)
}

func TestGetConnections_WithConnections(t *testing.T) {
	parent := newTestTunnel()
	logger := newTestLogger()

	tcp := NewTCPTunnel("tunnel-1", 0, "remote:9090", parent, logger)

	serverConn1, clientConn1 := net.Pipe()
	serverConn2, clientConn2 := net.Pipe()
	defer serverConn1.Close()
	defer clientConn1.Close()
	defer serverConn2.Close()
	defer clientConn2.Close()

	now := time.Now()
	tcp.mu.Lock()
	tcp.connections["conn-1"] = &TCPConnection{
		ID:            "conn-1",
		TunnelID:      tcp.ID,
		LocalConn:     serverConn1,
		RemoteAddr:    "127.0.0.1:11111",
		CreatedAt:     now,
		LastActivity:  now,
		BytesSent:     100,
		BytesReceived: 200,
	}
	tcp.connections["conn-2"] = &TCPConnection{
		ID:            "conn-2",
		TunnelID:      tcp.ID,
		LocalConn:     serverConn2,
		RemoteAddr:    "127.0.0.1:22222",
		CreatedAt:     now,
		LastActivity:  now,
		BytesSent:     300,
		BytesReceived: 400,
	}
	tcp.mu.Unlock()

	conns := tcp.GetConnections()
	assert.Len(t, conns, 2)

	// Verify connection info fields are present
	for _, c := range conns {
		assert.Contains(t, c, "id")
		assert.Contains(t, c, "remote_addr")
		assert.Contains(t, c, "created_at")
		assert.Contains(t, c, "last_activity")
		assert.Contains(t, c, "bytes_sent")
		assert.Contains(t, c, "bytes_received")
		assert.Contains(t, c, "idle_time")
	}
}

// --- isStopped tests ---

func TestIsStopped_NotStopped(t *testing.T) {
	parent := newTestTunnel()
	logger := newTestLogger()

	tcp := NewTCPTunnel("tunnel-1", 0, "remote:9090", parent, logger)
	assert.False(t, tcp.isStopped())
}

func TestIsStopped_AfterStop(t *testing.T) {
	parent := newTestTunnel()
	logger := newTestLogger()

	tcp := NewTCPTunnel("tunnel-1", 0, "remote:9090", parent, logger)
	err := tcp.Start()
	require.NoError(t, err)

	tcp.Stop()
	assert.True(t, tcp.isStopped())
}

// --- closeConnection tests ---

func TestCloseConnection_Idempotent(t *testing.T) {
	parent := newTestTunnel()
	logger := newTestLogger()

	tcp := NewTCPTunnel("tunnel-1", 0, "remote:9090", parent, logger)

	serverConn, clientConn := net.Pipe()
	defer clientConn.Close()

	conn := &TCPConnection{
		ID:           "conn-1",
		TunnelID:     tcp.ID,
		LocalConn:    serverConn,
		RemoteAddr:   "127.0.0.1:12345",
		CreatedAt:    time.Now(),
		LastActivity: time.Now(),
	}

	tcp.mu.Lock()
	tcp.connections["conn-1"] = conn
	tcp.mu.Unlock()

	// First close
	tcp.closeConnection("conn-1", conn)
	assert.True(t, conn.closed)

	// Second close should be a no-op (no panic)
	tcp.closeConnection("conn-1", conn)
	assert.True(t, conn.closed)
}

// --- TCP tunnel connection acceptance tests ---

func TestTCPTunnel_AcceptsConnections(t *testing.T) {
	parent := newTestTunnel()
	logger := newTestLogger()

	tcp := NewTCPTunnel("tunnel-1", 0, "remote:9090", parent, logger)

	err := tcp.Start()
	require.NoError(t, err)
	defer tcp.Stop()

	// Get the actual listening address
	addr := tcp.listener.Addr().String()

	// Connect to the TCP tunnel
	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	require.NoError(t, err)
	defer conn.Close()

	// Give the goroutine a moment to register the connection
	time.Sleep(100 * time.Millisecond)

	tcp.mu.RLock()
	connCount := len(tcp.connections)
	tcp.mu.RUnlock()

	assert.Equal(t, 1, connCount)
}

func TestTCPTunnel_MaxConnectionsEnforced(t *testing.T) {
	parent := newTestTunnel()
	logger := newTestLogger()

	tcp := NewTCPTunnel("tunnel-1", 0, "remote:9090", parent, logger)
	tcp.MaxConnections = 2

	err := tcp.Start()
	require.NoError(t, err)
	defer tcp.Stop()

	addr := tcp.listener.Addr().String()

	// Create connections up to the max
	var conns []net.Conn
	for i := 0; i < 2; i++ {
		conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
		require.NoError(t, err)
		conns = append(conns, conn)
	}
	defer func() {
		for _, c := range conns {
			c.Close()
		}
	}()

	// Wait for connections to register
	time.Sleep(200 * time.Millisecond)

	tcp.mu.RLock()
	assert.Equal(t, 2, len(tcp.connections))
	tcp.mu.RUnlock()

	// Third connection should be accepted at TCP level but the handler should reject it
	conn3, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err == nil {
		defer conn3.Close()
		// The connection should be closed by the handler due to max connections
		time.Sleep(200 * time.Millisecond)

		tcp.mu.RLock()
		assert.LessOrEqual(t, len(tcp.connections), 2)
		tcp.mu.RUnlock()
	}
}

// --- Concurrent TCP tunnel tests ---

func TestTCPTunnel_ConcurrentGetConnections(t *testing.T) {
	parent := newTestTunnel()
	logger := newTestLogger()

	tcp := NewTCPTunnel("tunnel-1", 0, "remote:9090", parent, logger)

	// Add some connections
	for i := 0; i < 5; i++ {
		serverConn, clientConn := net.Pipe()
		defer serverConn.Close()
		defer clientConn.Close()

		connID := "conn-" + string(rune('a'+i))
		tcp.mu.Lock()
		tcp.connections[connID] = &TCPConnection{
			ID:           connID,
			TunnelID:     tcp.ID,
			LocalConn:    serverConn,
			RemoteAddr:   "127.0.0.1:12345",
			CreatedAt:    time.Now(),
			LastActivity: time.Now(),
		}
		tcp.mu.Unlock()
	}

	var wg sync.WaitGroup

	// Concurrent reads
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			conns := tcp.GetConnections()
			assert.NotNil(t, conns)
		}()
	}

	wg.Wait()
}

func TestTCPTunnel_ConcurrentHandleRemoteData(t *testing.T) {
	parent := newTestTunnel()
	logger := newTestLogger()

	tcp := NewTCPTunnel("tunnel-1", 0, "remote:9090", parent, logger)

	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()

	conn := &TCPConnection{
		ID:           "conn-1",
		TunnelID:     tcp.ID,
		LocalConn:    serverConn,
		RemoteAddr:   "127.0.0.1:12345",
		CreatedAt:    time.Now(),
		LastActivity: time.Now(),
	}

	tcp.mu.Lock()
	tcp.connections["conn-1"] = conn
	tcp.mu.Unlock()

	// Read data from the pipe in the background
	go func() {
		buf := make([]byte, 4096)
		for {
			_, err := clientConn.Read(buf)
			if err != nil {
				return
			}
		}
	}()

	var wg sync.WaitGroup
	iterations := 10

	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			data := []byte("data-from-remote")
			_ = tcp.HandleRemoteData("conn-1", data)
		}(i)
	}

	wg.Wait()

	// Verify bytes were tracked
	conn.mu.Lock()
	assert.Greater(t, conn.BytesReceived, int64(0))
	conn.mu.Unlock()
}

// --- TCPConnection bytes tracking ---

func TestTCPConnection_BytesTracking(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	conn := &TCPConnection{
		ID:           "conn-1",
		TunnelID:     "tunnel-1",
		LocalConn:    serverConn,
		RemoteAddr:   "127.0.0.1:12345",
		CreatedAt:    time.Now(),
		LastActivity: time.Now(),
	}

	assert.Equal(t, int64(0), conn.BytesSent)
	assert.Equal(t, int64(0), conn.BytesReceived)
	assert.False(t, conn.closed)
}

// --- Reconnection backoff calculation test ---
// Tests the exponential backoff pattern used in handleWebSocketErrors

func TestExponentialBackoff_Calculation(t *testing.T) {
	baseDelay := 2 * time.Second
	maxDelay := 1 * time.Minute

	tests := []struct {
		name         string
		retryCount   uint
		expectedBase time.Duration
	}{
		{"first retry", 0, 2 * time.Second},
		{"second retry", 1, 4 * time.Second},
		{"third retry", 2, 8 * time.Second},
		{"fourth retry", 3, 16 * time.Second},
		{"fifth retry", 4, 32 * time.Second},
		{"sixth retry (capped)", 5, 1 * time.Minute},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			delay := baseDelay * time.Duration(1<<tt.retryCount)
			if delay > maxDelay {
				delay = maxDelay
			}
			assert.Equal(t, tt.expectedBase, delay)
		})
	}
}

// --- TunnelInfo JSON serialization test ---

func TestTunnelInfo_JSONSerialization(t *testing.T) {
	info := TunnelInfo{
		ID:         "test-id",
		UserID:     "user-1",
		Protocol:   "http",
		LocalPort:  8080,
		PublicURL:  "http://test.example.com",
		ServerHost: "example.com",
		ServerPort: 443,
		Created:    time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		LastActive: time.Date(2025, 1, 1, 1, 0, 0, 0, time.UTC),
		Active:     true,
	}

	data, err := json.MarshalIndent(info, "", "  ")
	require.NoError(t, err)

	var decoded TunnelInfo
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, info.ID, decoded.ID)
	assert.Equal(t, info.UserID, decoded.UserID)
	assert.Equal(t, info.Protocol, decoded.Protocol)
	assert.Equal(t, info.LocalPort, decoded.LocalPort)
	assert.Equal(t, info.PublicURL, decoded.PublicURL)
	assert.Equal(t, info.Active, decoded.Active)
	assert.True(t, decoded.ExpiresAt.IsZero(), "omitempty should leave ExpiresAt zero when not set")
}
