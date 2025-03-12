package protocol

import (
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Helper functions ---

// newTestConfig returns a HandlerConfig suitable for testing with sensible defaults.
func newTestConfig() HandlerConfig {
	logger := logrus.New()
	logger.SetOutput(io.Discard)
	return HandlerConfig{
		LocalPort:  8080,
		RemoteAddr: "127.0.0.1:0",
		Logger:     logger,
		Timeout:    5 * time.Second,
	}
}

// startLocalHTTPServer starts a local HTTP server that echoes back request info.
// Returns the server and its listener port.
func startLocalHTTPServer(t *testing.T) (*httptest.Server, int) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Header().Set("X-Test-Method", r.Method)
		w.Header().Set("X-Test-Path", r.URL.Path)
		// Echo forwarded headers back
		if v := r.Header.Get("X-Forwarded-Proto"); v != "" {
			w.Header().Set("X-Echo-Forwarded-Proto", v)
		}
		if v := r.Header.Get("X-Forwarded-Host"); v != "" {
			w.Header().Set("X-Echo-Forwarded-Host", v)
		}
		if v := r.Header.Get("X-Custom-Header"); v != "" {
			w.Header().Set("X-Echo-Custom", v)
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "OK: %s %s", r.Method, r.URL.Path)
	}))
	t.Cleanup(server.Close)

	// Extract port from server URL
	_, portStr, err := net.SplitHostPort(server.Listener.Addr().String())
	require.NoError(t, err)
	var port int
	fmt.Sscanf(portStr, "%d", &port)
	return server, port
}

// --- HTTP Request/Response Serialization Tests ---

func TestHTTPHandler_ProxyForwardsRequests(t *testing.T) {
	_, localPort := startLocalHTTPServer(t)

	config := newTestConfig()
	config.LocalPort = localPort
	config.CustomHeaders = map[string]string{"X-Custom-Header": "proxied"}

	handler := NewHTTPHandler(config)
	require.NoError(t, handler.setupProxy())

	// Use the proxy directly with httptest to test end-to-end forwarding
	wrapped := handler.wrapHandlerWithMetrics(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler.proxy.ServeHTTP(w, r)
	}))
	ts := httptest.NewServer(wrapped)
	t.Cleanup(ts.Close)

	resp, err := http.Get(ts.URL + "/hello")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Contains(t, string(body), "OK: GET /hello")

	// Verify forwarded headers were echoed back
	assert.Equal(t, "http", resp.Header.Get("X-Echo-Forwarded-Proto"))
	assert.Equal(t, "proxied", resp.Header.Get("X-Echo-Custom"))

	// Verify metrics recorded the request
	metrics := handler.GetMetrics()
	assert.Equal(t, int64(1), metrics["requests_forwarded"])
	assert.Greater(t, metrics["bytes_forwarded"].(int64), int64(0))
}

func TestHTTPHandler_SetupProxy_SetsForwardedHeaders(t *testing.T) {
	_, localPort := startLocalHTTPServer(t)

	config := newTestConfig()
	config.LocalPort = localPort
	config.CustomHeaders = map[string]string{
		"X-Custom-Header": "test-value",
	}

	handler := NewHTTPHandler(config)
	require.NoError(t, handler.setupProxy())
	require.NotNil(t, handler.proxy)

	// Create a test request and verify the director modifies it correctly
	req := httptest.NewRequest(http.MethodGet, "http://example.com/test", nil)
	req.RemoteAddr = "192.168.1.100:12345"

	handler.proxy.Director(req)

	assert.Equal(t, "http", req.Header.Get("X-Forwarded-Proto"))
	assert.NotEmpty(t, req.Header.Get("X-Forwarded-Host"))
	assert.Equal(t, "192.168.1.100", req.Header.Get("X-Forwarded-For"))
	assert.Equal(t, "test-value", req.Header.Get("X-Custom-Header"))
}

func TestHTTPHandler_SetupProxy_LocalHTTPS(t *testing.T) {
	config := newTestConfig()
	config.LocalHTTPS = true

	handler := NewHTTPHandler(config)
	require.NoError(t, handler.setupProxy())

	// Verify the transport allows insecure TLS for local HTTPS
	transport, ok := handler.proxy.Transport.(*http.Transport)
	require.True(t, ok)
	assert.True(t, transport.TLSClientConfig.InsecureSkipVerify)
}

func TestHTTPHandler_SetupProxy_DefaultScheme(t *testing.T) {
	config := newTestConfig()
	config.LocalHTTPS = false

	handler := NewHTTPHandler(config)
	require.NoError(t, handler.setupProxy())

	transport, ok := handler.proxy.Transport.(*http.Transport)
	require.True(t, ok)
	assert.False(t, transport.TLSClientConfig.InsecureSkipVerify)
}

func TestHTTPSHandler_SetupProxy_SetsHTTPSHeaders(t *testing.T) {
	_, localPort := startLocalHTTPServer(t)

	config := newTestConfig()
	config.LocalPort = localPort
	config.TLSConfig = &tls.Config{}

	handler := NewHTTPSHandler(config)
	require.NoError(t, handler.setupProxy())

	req := httptest.NewRequest(http.MethodGet, "https://example.com/test", nil)
	req.RemoteAddr = "10.0.0.1:54321"

	handler.proxy.Director(req)

	assert.Equal(t, "https", req.Header.Get("X-Forwarded-Proto"))
	assert.Equal(t, "443", req.Header.Get("X-Forwarded-Port"))
	assert.Equal(t, "on", req.Header.Get("X-Forwarded-Ssl"))
	assert.Equal(t, "10.0.0.1", req.Header.Get("X-Forwarded-For"))
}

func TestHTTPSHandler_SetupProxy_CustomHeaders(t *testing.T) {
	config := newTestConfig()
	config.TLSConfig = &tls.Config{}
	config.CustomHeaders = map[string]string{
		"X-Service-Name": "nxpose",
		"X-Request-ID":   "abc-123",
	}

	handler := NewHTTPSHandler(config)
	require.NoError(t, handler.setupProxy())

	req := httptest.NewRequest(http.MethodGet, "https://example.com/", nil)
	req.RemoteAddr = "1.2.3.4:9999"
	handler.proxy.Director(req)

	assert.Equal(t, "nxpose", req.Header.Get("X-Service-Name"))
	assert.Equal(t, "abc-123", req.Header.Get("X-Request-ID"))
}

func TestResponseWriter_CapturesStatusAndSize(t *testing.T) {
	recorder := httptest.NewRecorder()
	rw := &responseWriter{
		ResponseWriter: recorder,
		statusCode:     http.StatusOK,
	}

	// Write a custom status code
	rw.WriteHeader(http.StatusCreated)
	assert.Equal(t, http.StatusCreated, rw.statusCode)
	assert.Equal(t, http.StatusCreated, recorder.Code)

	// Write body data
	n, err := rw.Write([]byte("hello world"))
	require.NoError(t, err)
	assert.Equal(t, 11, n)
	assert.Equal(t, 11, rw.size)

	// Write more data - size should accumulate
	n, err = rw.Write([]byte("!!"))
	require.NoError(t, err)
	assert.Equal(t, 2, n)
	assert.Equal(t, 13, rw.size)
}

func TestResponseWriter_DefaultStatusOK(t *testing.T) {
	recorder := httptest.NewRecorder()
	rw := &responseWriter{
		ResponseWriter: recorder,
		statusCode:     http.StatusOK,
	}

	// Without calling WriteHeader, status should remain as initialized
	assert.Equal(t, http.StatusOK, rw.statusCode)
}

func TestHTTPHandler_WrapHandlerWithMetrics(t *testing.T) {
	config := newTestConfig()
	handler := NewHTTPHandler(config)

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("response body"))
	})

	wrapped := handler.wrapHandlerWithMetrics(inner)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	wrapped.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "response body", rec.Body.String())

	// Check metrics were updated
	metrics := handler.GetMetrics()
	assert.Equal(t, int64(1), metrics["requests_forwarded"])
	assert.Equal(t, int64(len("response body")), metrics["bytes_forwarded"])
	assert.Equal(t, int64(0), metrics["error_count"])
	assert.Equal(t, 0, metrics["active_connections"]) // connection opened and closed
}

func TestHTTPHandler_Forward_ReturnsError(t *testing.T) {
	config := newTestConfig()
	handler := NewHTTPHandler(config)

	// Forward is not implemented for HTTP handlers
	err := handler.Forward(nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not implemented for HTTP")
}

// --- Protocol Message Type Handling (GetHandler factory, handler lifecycle) ---

func TestGetHandler_HTTP(t *testing.T) {
	config := newTestConfig()
	h, err := GetHandler("http", config)
	require.NoError(t, err)
	require.NotNil(t, h)

	_, ok := h.(*HTTPHandler)
	assert.True(t, ok, "expected *HTTPHandler")
}

func TestGetHandler_HTTPS_WithTLS(t *testing.T) {
	config := newTestConfig()
	config.TLSConfig = &tls.Config{}

	h, err := GetHandler("https", config)
	require.NoError(t, err)
	require.NotNil(t, h)

	_, ok := h.(*HTTPSHandler)
	assert.True(t, ok, "expected *HTTPSHandler")
}

func TestGetHandler_HTTPS_WithoutTLS_ReturnsError(t *testing.T) {
	config := newTestConfig()
	config.TLSConfig = nil

	h, err := GetHandler("https", config)
	require.Error(t, err)
	assert.Nil(t, h)
	assert.Contains(t, err.Error(), "TLS configuration is required")
}

func TestGetHandler_TCP(t *testing.T) {
	config := newTestConfig()
	h, err := GetHandler("tcp", config)
	require.NoError(t, err)
	require.NotNil(t, h)

	_, ok := h.(*TCPHandler)
	assert.True(t, ok, "expected *TCPHandler")
}

func TestGetHandler_UnsupportedProtocol(t *testing.T) {
	config := newTestConfig()
	h, err := GetHandler("ftp", config)
	require.Error(t, err)
	assert.Nil(t, h)
	assert.Contains(t, err.Error(), "unsupported protocol: ftp")
}

func TestGetHandler_CaseInsensitive(t *testing.T) {
	config := newTestConfig()

	tests := []struct {
		protocol string
		expected string
	}{
		{"HTTP", "*protocol.HTTPHandler"},
		{"Http", "*protocol.HTTPHandler"},
		{"TCP", "*protocol.TCPHandler"},
		{"Tcp", "*protocol.TCPHandler"},
	}

	for _, tt := range tests {
		t.Run(tt.protocol, func(t *testing.T) {
			h, err := GetHandler(tt.protocol, config)
			require.NoError(t, err)
			require.NotNil(t, h)
			assert.Equal(t, tt.expected, fmt.Sprintf("%T", h))
		})
	}
}

func TestHTTPSHandler_Start_WithoutTLSConfig_ReturnsError(t *testing.T) {
	config := newTestConfig()
	config.TLSConfig = nil

	handler := NewHTTPSHandler(config)
	err := handler.Start()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "TLS configuration is required")
}

func TestHTTPHandler_StartAndStop(t *testing.T) {
	_, localPort := startLocalHTTPServer(t)

	config := newTestConfig()
	config.LocalPort = localPort
	// Use port 0 to let OS assign a free port
	config.RemoteAddr = "127.0.0.1:0"

	handler := NewHTTPHandler(config)

	// Start should succeed (server starts on ephemeral port)
	err := handler.Start()
	require.NoError(t, err)

	// Stop should succeed
	err = handler.Stop()
	require.NoError(t, err)
}

func TestHTTPHandler_Stop_NilServer(t *testing.T) {
	config := newTestConfig()
	handler := NewHTTPHandler(config)

	// Stop with nil server should not panic
	err := handler.Stop()
	assert.NoError(t, err)
}

func TestTCPHandler_StartAndStop(t *testing.T) {
	config := newTestConfig()
	config.RemoteAddr = "127.0.0.1:0"

	handler := NewTCPHandler(config)
	require.NoError(t, handler.Start())

	// Verify listener is active
	require.NotNil(t, handler.listener)

	// Stop should work cleanly
	require.NoError(t, handler.Stop())
}

func TestTCPHandler_Forward(t *testing.T) {
	// Start a local TCP echo server
	echoListener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { echoListener.Close() })

	go func() {
		for {
			conn, err := echoListener.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				io.Copy(c, c) // echo back
			}(conn)
		}
	}()

	_, portStr, _ := net.SplitHostPort(echoListener.Addr().String())
	var echoPort int
	fmt.Sscanf(portStr, "%d", &echoPort)

	config := newTestConfig()
	config.LocalPort = echoPort
	config.RemoteAddr = "127.0.0.1:0"

	handler := NewTCPHandler(config)
	require.NoError(t, handler.Start())
	t.Cleanup(func() { handler.Stop() })

	// Connect to the TCP handler
	conn, err := net.Dial("tcp", handler.listener.Addr().String())
	require.NoError(t, err)
	defer conn.Close()

	// Send data through the tunnel
	testData := "hello through tunnel"
	_, err = conn.Write([]byte(testData))
	require.NoError(t, err)

	// Read echoed response
	buf := make([]byte, len(testData))
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, err := io.ReadFull(conn, buf)
	require.NoError(t, err)
	assert.Equal(t, testData, string(buf[:n]))
}

func TestTCPHandler_Forward_Method(t *testing.T) {
	// Start a local TCP echo server
	echoListener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { echoListener.Close() })

	go func() {
		for {
			conn, err := echoListener.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				io.Copy(c, c)
			}(conn)
		}
	}()

	_, portStr, _ := net.SplitHostPort(echoListener.Addr().String())
	var echoPort int
	fmt.Sscanf(portStr, "%d", &echoPort)

	config := newTestConfig()
	config.LocalPort = echoPort
	config.RemoteAddr = "127.0.0.1:0"

	handler := NewTCPHandler(config)
	require.NoError(t, handler.Start())
	t.Cleanup(func() { handler.Stop() })

	// Create a pipe to simulate a connection and call Forward directly
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	err = handler.Forward(serverConn)
	assert.NoError(t, err)
}

func TestTCPHandler_ConnectionTracking(t *testing.T) {
	// Start a local TCP server that holds connections open
	holdListener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { holdListener.Close() })

	go func() {
		for {
			conn, err := holdListener.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				io.Copy(io.Discard, c) // read and discard
			}(conn)
		}
	}()

	_, portStr, _ := net.SplitHostPort(holdListener.Addr().String())
	var holdPort int
	fmt.Sscanf(portStr, "%d", &holdPort)

	config := newTestConfig()
	config.LocalPort = holdPort
	config.RemoteAddr = "127.0.0.1:0"

	handler := NewTCPHandler(config)
	require.NoError(t, handler.Start())
	t.Cleanup(func() { handler.Stop() })

	// Open a connection
	conn, err := net.Dial("tcp", handler.listener.Addr().String())
	require.NoError(t, err)

	// Give handler time to accept and track the connection
	time.Sleep(100 * time.Millisecond)

	handler.connectionsMu.RLock()
	connCount := len(handler.connections)
	handler.connectionsMu.RUnlock()
	assert.Equal(t, 1, connCount, "expected one tracked connection")

	// Close the connection and verify cleanup
	conn.Close()
	time.Sleep(200 * time.Millisecond)

	handler.connectionsMu.RLock()
	connCount = len(handler.connections)
	handler.connectionsMu.RUnlock()
	assert.Equal(t, 0, connCount, "expected no tracked connections after close")
}

func TestTCPHandler_Start_InvalidAddress(t *testing.T) {
	config := newTestConfig()
	// Use an address that should fail to bind
	config.RemoteAddr = "999.999.999.999:0"

	handler := NewTCPHandler(config)
	err := handler.Start()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create TCP listener")
}

// --- Metrics Collection Tests ---

func TestBaseHandler_GetMetrics_Initial(t *testing.T) {
	config := newTestConfig()
	base := newBaseHandler(config)

	metrics := base.GetMetrics()
	assert.Equal(t, int64(0), metrics["requests_forwarded"])
	assert.Equal(t, int64(0), metrics["bytes_forwarded"])
	assert.Equal(t, int64(0), metrics["error_count"])
	assert.Equal(t, 0, metrics["active_connections"])
	assert.IsType(t, time.Time{}, metrics["last_request_time"])
	assert.IsType(t, "", metrics["uptime"])
}

func TestBaseHandler_UpdateMetrics_SingleUpdate(t *testing.T) {
	config := newTestConfig()
	base := newBaseHandler(config)

	base.updateMetrics(5, 1024, 1, 2)

	metrics := base.GetMetrics()
	assert.Equal(t, int64(5), metrics["requests_forwarded"])
	assert.Equal(t, int64(1024), metrics["bytes_forwarded"])
	assert.Equal(t, int64(1), metrics["error_count"])
	assert.Equal(t, 2, metrics["active_connections"])
}

func TestBaseHandler_UpdateMetrics_Accumulates(t *testing.T) {
	config := newTestConfig()
	base := newBaseHandler(config)

	base.updateMetrics(1, 100, 0, 1)
	base.updateMetrics(2, 200, 1, -1)
	base.updateMetrics(3, 300, 0, 0)

	metrics := base.GetMetrics()
	assert.Equal(t, int64(6), metrics["requests_forwarded"])
	assert.Equal(t, int64(600), metrics["bytes_forwarded"])
	assert.Equal(t, int64(1), metrics["error_count"])
	assert.Equal(t, 0, metrics["active_connections"])
}

func TestBaseHandler_UpdateMetrics_SetsLastRequestTime(t *testing.T) {
	config := newTestConfig()
	base := newBaseHandler(config)

	before := time.Now()
	base.updateMetrics(1, 0, 0, 0)
	after := time.Now()

	metrics := base.GetMetrics()
	lastTime := metrics["last_request_time"].(time.Time)
	assert.True(t, !lastTime.Before(before), "last_request_time should be after start")
	assert.True(t, !lastTime.After(after), "last_request_time should be before end")
}

func TestBaseHandler_UpdateMetrics_ConcurrentSafety(t *testing.T) {
	config := newTestConfig()
	base := newBaseHandler(config)

	var wg sync.WaitGroup
	iterations := 100

	// Concurrent writers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				base.updateMetrics(1, 10, 0, 0)
			}
		}()
	}

	// Concurrent readers
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				_ = base.GetMetrics()
			}
		}()
	}

	wg.Wait()

	metrics := base.GetMetrics()
	assert.Equal(t, int64(10*iterations), metrics["requests_forwarded"])
	assert.Equal(t, int64(10*iterations*10), metrics["bytes_forwarded"])
}

func TestBaseHandler_GetMetrics_UptimeFormat(t *testing.T) {
	config := newTestConfig()
	base := newBaseHandler(config)

	// Update to set last_request_time
	base.updateMetrics(1, 0, 0, 0)
	time.Sleep(10 * time.Millisecond)

	metrics := base.GetMetrics()
	uptime := metrics["uptime"].(string)
	assert.NotEmpty(t, uptime, "uptime should be a non-empty duration string")
}

func TestHTTPHandler_MetricsAfterProxyError(t *testing.T) {
	config := newTestConfig()
	// Point to a port that is not listening - will cause proxy error
	config.LocalPort = 1 // unlikely to be in use
	config.RemoteAddr = "127.0.0.1:0"

	handler := NewHTTPHandler(config)
	require.NoError(t, handler.setupProxy())

	// Trigger the error handler directly
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	handler.proxy.ErrorHandler(rec, req, fmt.Errorf("connection refused"))

	assert.Equal(t, http.StatusBadGateway, rec.Code)

	metrics := handler.GetMetrics()
	assert.Equal(t, int64(1), metrics["error_count"])
}

// --- Error Message Formatting Tests ---

func TestGetHandler_ErrorMessages(t *testing.T) {
	tests := []struct {
		name     string
		protocol string
		config   HandlerConfig
		errMsg   string
	}{
		{
			name:     "unsupported protocol",
			protocol: "websocket",
			config:   newTestConfig(),
			errMsg:   "unsupported protocol: websocket",
		},
		{
			name:     "empty protocol",
			protocol: "",
			config:   newTestConfig(),
			errMsg:   "unsupported protocol: ",
		},
		{
			name:     "HTTPS without TLS",
			protocol: "https",
			config:   newTestConfig(),
			errMsg:   "TLS configuration is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := GetHandler(tt.protocol, tt.config)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.errMsg)
		})
	}
}

func TestIsClosedConnError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "EOF",
			err:      io.EOF,
			expected: true,
		},
		{
			name:     "closed connection",
			err:      fmt.Errorf("use of closed network connection"),
			expected: true,
		},
		{
			name:     "connection reset",
			err:      fmt.Errorf("connection reset by peer"),
			expected: true,
		},
		{
			name:     "broken pipe",
			err:      fmt.Errorf("broken pipe"),
			expected: true,
		},
		{
			name:     "generic error",
			err:      fmt.Errorf("something went wrong"),
			expected: false,
		},
		{
			name:     "timeout error",
			err:      fmt.Errorf("i/o timeout"),
			expected: false,
		},
		{
			name:     "message containing closed connection substring",
			err:      fmt.Errorf("read tcp 127.0.0.1:8080: use of closed network connection"),
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isClosedConnError(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetClientIP(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
		headers    map[string]string
		expected   string
	}{
		{
			name:       "from RemoteAddr",
			remoteAddr: "192.168.1.1:12345",
			headers:    nil,
			expected:   "192.168.1.1",
		},
		{
			name:       "from X-Forwarded-For single",
			remoteAddr: "10.0.0.1:9999",
			headers:    map[string]string{"X-Forwarded-For": "203.0.113.50"},
			expected:   "203.0.113.50",
		},
		{
			name:       "from X-Forwarded-For multiple",
			remoteAddr: "10.0.0.1:9999",
			headers:    map[string]string{"X-Forwarded-For": "203.0.113.50, 70.41.3.18, 150.172.238.178"},
			expected:   "203.0.113.50",
		},
		{
			name:       "from X-Real-IP",
			remoteAddr: "10.0.0.1:9999",
			headers:    map[string]string{"X-Real-IP": "198.51.100.25"},
			expected:   "198.51.100.25",
		},
		{
			name:       "X-Forwarded-For takes precedence over X-Real-IP",
			remoteAddr: "10.0.0.1:9999",
			headers: map[string]string{
				"X-Forwarded-For": "203.0.113.50",
				"X-Real-IP":       "198.51.100.25",
			},
			expected: "203.0.113.50",
		},
		{
			name:       "RemoteAddr without port",
			remoteAddr: "192.168.1.1",
			headers:    nil,
			expected:   "192.168.1.1", // Returned unsplit when SplitHostPort fails
		},
		{
			name:       "X-Forwarded-For with spaces",
			remoteAddr: "10.0.0.1:9999",
			headers:    map[string]string{"X-Forwarded-For": "  203.0.113.50  , 70.41.3.18"},
			expected:   "203.0.113.50",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = tt.remoteAddr
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}
			result := getClientIP(req)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNewBaseHandler_Defaults(t *testing.T) {
	// Test that newBaseHandler fills in missing values
	config := HandlerConfig{}
	base := newBaseHandler(config)

	assert.Equal(t, 30*time.Second, base.config.Timeout)
	assert.NotNil(t, base.config.Logger)
	assert.NotNil(t, base.ctx)
	assert.NotNil(t, base.cancelFunc)
}

func TestNewBaseHandler_PreservesExplicitValues(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(io.Discard)

	config := HandlerConfig{
		Timeout: 60 * time.Second,
		Logger:  logger,
	}
	base := newBaseHandler(config)

	assert.Equal(t, 60*time.Second, base.config.Timeout)
	assert.Same(t, logger, base.config.Logger)
}

func TestNewHTTPHandler_StoresConfig(t *testing.T) {
	config := newTestConfig()
	config.RemoteAddr = "0.0.0.0:9090"

	handler := NewHTTPHandler(config)
	assert.Equal(t, "0.0.0.0:9090", handler.serverAddr)
}

func TestNewHTTPSHandler_StoresConfig(t *testing.T) {
	config := newTestConfig()
	config.RemoteAddr = "0.0.0.0:9443"

	handler := NewHTTPSHandler(config)
	assert.Equal(t, "0.0.0.0:9443", handler.serverAddr)
}

func TestNewTCPHandler_InitializesFields(t *testing.T) {
	config := newTestConfig()
	handler := NewTCPHandler(config)

	assert.NotNil(t, handler.connections)
	assert.Empty(t, handler.connections)
	assert.NotNil(t, handler.shutdownChan)
}

func TestTCPHandler_StopClosesAllConnections(t *testing.T) {
	// Start a local TCP server
	echoListener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { echoListener.Close() })

	go func() {
		for {
			conn, err := echoListener.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				io.Copy(io.Discard, c)
			}(conn)
		}
	}()

	_, portStr, _ := net.SplitHostPort(echoListener.Addr().String())
	var echoPort int
	fmt.Sscanf(portStr, "%d", &echoPort)

	config := newTestConfig()
	config.LocalPort = echoPort
	config.RemoteAddr = "127.0.0.1:0"

	handler := NewTCPHandler(config)
	require.NoError(t, handler.Start())

	// Open multiple connections
	var conns []net.Conn
	for i := 0; i < 3; i++ {
		conn, err := net.Dial("tcp", handler.listener.Addr().String())
		require.NoError(t, err)
		conns = append(conns, conn)
	}

	// Give time for connections to be accepted and tracked
	time.Sleep(200 * time.Millisecond)

	// Stop the handler - should close all connections
	require.NoError(t, handler.Stop())

	// Verify all connections are closed by trying to read
	for _, conn := range conns {
		conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		buf := make([]byte, 1)
		_, err := conn.Read(buf)
		// Should get an error (EOF or closed)
		assert.Error(t, err)
		conn.Close()
	}
}

func TestHTTPHandler_ProxyErrorHandler_Returns502(t *testing.T) {
	config := newTestConfig()
	handler := NewHTTPHandler(config)
	require.NoError(t, handler.setupProxy())

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/data", strings.NewReader("body"))
	handler.proxy.ErrorHandler(rec, req, fmt.Errorf("dial tcp: connection refused"))

	assert.Equal(t, http.StatusBadGateway, rec.Code)
	assert.Contains(t, rec.Body.String(), "Error connecting to the local service")
}

func TestHTTPSHandler_ProxyErrorHandler_Returns502(t *testing.T) {
	config := newTestConfig()
	config.TLSConfig = &tls.Config{}
	handler := NewHTTPSHandler(config)
	require.NoError(t, handler.setupProxy())

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.proxy.ErrorHandler(rec, req, fmt.Errorf("connection refused"))

	assert.Equal(t, http.StatusBadGateway, rec.Code)
	assert.Contains(t, rec.Body.String(), "Error connecting to the local service")
}

func TestHTTPHandler_SetupProxy_DebugLogging(t *testing.T) {
	config := newTestConfig()
	config.Logger.SetLevel(logrus.DebugLevel)

	handler := NewHTTPHandler(config)
	require.NoError(t, handler.setupProxy())

	// Ensure proxy was set up with debug-level director - should not panic
	req := httptest.NewRequest(http.MethodGet, "http://example.com/debug", nil)
	req.RemoteAddr = "1.2.3.4:5678"
	handler.proxy.Director(req)

	// Just verify it doesn't panic with debug logging enabled
	assert.Equal(t, "http", req.Header.Get("X-Forwarded-Proto"))
}
