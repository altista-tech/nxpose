package server

import (
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- WebSocketManager tests ---

func TestWebSocketManager_New(t *testing.T) {
	mgr := NewWebSocketManager()
	assert.NotNil(t, mgr)
	assert.NotNil(t, mgr.tunnels)
	assert.NotNil(t, mgr.requests)
	assert.Empty(t, mgr.tunnels)
	assert.Empty(t, mgr.requests)
}

func TestWebSocketManager_RegisterAndGet(t *testing.T) {
	mgr := NewWebSocketManager()

	wsTunnel := &WebSocketTunnel{
		ID:          "ws-1",
		ClientID:    "client-1",
		TunnelID:    "tunnel-1",
		Connected:   true,
		ConnectedAt: time.Now(),
	}
	wsTunnel.SetLastActive(time.Now())

	mgr.RegisterWebSocketTunnel("tunnel-1", wsTunnel)

	got, exists := mgr.GetWebSocketTunnel("tunnel-1")
	assert.True(t, exists)
	assert.Equal(t, wsTunnel, got)
}

func TestWebSocketManager_GetNonExistent(t *testing.T) {
	mgr := NewWebSocketManager()

	got, exists := mgr.GetWebSocketTunnel("nonexistent")
	assert.False(t, exists)
	assert.Nil(t, got)
}

func TestWebSocketManager_Unregister(t *testing.T) {
	mgr := NewWebSocketManager()

	wsTunnel := &WebSocketTunnel{
		ID:       "ws-1",
		TunnelID: "tunnel-1",
	}

	mgr.RegisterWebSocketTunnel("tunnel-1", wsTunnel)
	mgr.UnregisterWebSocketTunnel("tunnel-1")

	_, exists := mgr.GetWebSocketTunnel("tunnel-1")
	assert.False(t, exists)
}

func TestWebSocketManager_UnregisterNonExistent(t *testing.T) {
	mgr := NewWebSocketManager()
	// Should not panic
	mgr.UnregisterWebSocketTunnel("nonexistent")
}

func TestWebSocketManager_RegisterOverwrite(t *testing.T) {
	mgr := NewWebSocketManager()

	wsTunnel1 := &WebSocketTunnel{ID: "ws-1", TunnelID: "tunnel-1"}
	wsTunnel2 := &WebSocketTunnel{ID: "ws-2", TunnelID: "tunnel-1"}

	mgr.RegisterWebSocketTunnel("tunnel-1", wsTunnel1)
	mgr.RegisterWebSocketTunnel("tunnel-1", wsTunnel2)

	got, exists := mgr.GetWebSocketTunnel("tunnel-1")
	assert.True(t, exists)
	assert.Equal(t, "ws-2", got.ID, "should use the latest registered tunnel")
}

// --- Request/Response routing tests ---

func TestWebSocketManager_RegisterRequest(t *testing.T) {
	mgr := NewWebSocketManager()

	ch := mgr.RegisterRequest("req-1")
	assert.NotNil(t, ch)

	mgr.mu.RLock()
	_, exists := mgr.requests["req-1"]
	mgr.mu.RUnlock()
	assert.True(t, exists)
}

func TestWebSocketManager_UnregisterRequest(t *testing.T) {
	mgr := NewWebSocketManager()

	mgr.RegisterRequest("req-1")
	mgr.UnregisterRequest("req-1")

	mgr.mu.RLock()
	_, exists := mgr.requests["req-1"]
	mgr.mu.RUnlock()
	assert.False(t, exists)
}

func TestWebSocketManager_HandleResponse_Success(t *testing.T) {
	mgr := NewWebSocketManager()

	ch := mgr.RegisterRequest("req-1")

	response := &HTTPResponse{
		StatusCode: 200,
		Headers:    map[string]string{"Content-Type": "text/html"},
		Body:       []byte("Hello World"),
	}

	handled := mgr.HandleResponse("req-1", response)
	assert.True(t, handled)

	// Response should be available on the channel
	select {
	case got := <-ch:
		assert.Equal(t, 200, got.StatusCode)
		assert.Equal(t, "Hello World", string(got.Body))
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for response")
	}
}

func TestWebSocketManager_HandleResponse_NoWaitingRequest(t *testing.T) {
	mgr := NewWebSocketManager()

	response := &HTTPResponse{StatusCode: 200}
	handled := mgr.HandleResponse("nonexistent", response)
	assert.False(t, handled)
}

func TestWebSocketManager_HandleResponse_ChannelFull(t *testing.T) {
	mgr := NewWebSocketManager()

	ch := mgr.RegisterRequest("req-1")

	// Fill the channel (buffered with size 1)
	response1 := &HTTPResponse{StatusCode: 200}
	handled := mgr.HandleResponse("req-1", response1)
	assert.True(t, handled)

	// Re-register since HandleResponse unregisters on success
	mgr.mu.Lock()
	mgr.requests["req-1"] = ch
	mgr.mu.Unlock()

	// Second send to same channel should fail (already full)
	response2 := &HTTPResponse{StatusCode: 404}
	handled = mgr.HandleResponse("req-1", response2)
	assert.False(t, handled)
}

func TestWebSocketManager_ConcurrentOperations(t *testing.T) {
	mgr := NewWebSocketManager()

	var wg sync.WaitGroup

	// Concurrent register/unregister of tunnels
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			tunnelID := "tunnel-" + string(rune('A'+id%26))
			wsTunnel := &WebSocketTunnel{
				ID:       "ws-" + tunnelID,
				TunnelID: tunnelID,
			}

			mgr.RegisterWebSocketTunnel(tunnelID, wsTunnel)
			mgr.GetWebSocketTunnel(tunnelID)
			mgr.UnregisterWebSocketTunnel(tunnelID)
		}(i)
	}

	// Concurrent register/handle requests
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			reqID := "req-" + string(rune('A'+id%26))
			mgr.RegisterRequest(reqID)
			mgr.HandleResponse(reqID, &HTTPResponse{StatusCode: 200})
			mgr.UnregisterRequest(reqID)
		}(i)
	}

	wg.Wait()
}

func TestWebSocketManager_CloseAll(t *testing.T) {
	mgr := NewWebSocketManager()

	// Register some tunnels (without real connections - just to test the map clearing)
	mgr.mu.Lock()
	mgr.tunnels["t1"] = &WebSocketTunnel{ID: "ws-1", TunnelID: "t1", closed: true}
	mgr.tunnels["t2"] = &WebSocketTunnel{ID: "ws-2", TunnelID: "t2", closed: true}
	mgr.mu.Unlock()

	mgr.CloseAll()

	mgr.mu.RLock()
	count := len(mgr.tunnels)
	mgr.mu.RUnlock()
	assert.Equal(t, 0, count)
}

// --- TunnelMessage serialization tests ---

func TestTunnelMessage_Serialization(t *testing.T) {
	msg := TunnelMessage{
		Type:      "http_request",
		RequestID: "req-123",
		TunnelID:  "tun-456",
		Data:      json.RawMessage(`{"method":"GET","path":"/"}`),
	}

	data, err := json.Marshal(msg)
	require.NoError(t, err)

	var decoded TunnelMessage
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, msg.Type, decoded.Type)
	assert.Equal(t, msg.RequestID, decoded.RequestID)
	assert.Equal(t, msg.TunnelID, decoded.TunnelID)
}

func TestHTTPRequest_Serialization(t *testing.T) {
	req := HTTPRequest{
		Method:  "POST",
		Path:    "/api/data",
		Query:   "page=1&size=10",
		Headers: map[string]string{"Content-Type": "application/json"},
		Body:    []byte(`{"key":"value"}`),
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
		StatusCode: 201,
		Headers:    map[string]string{"Location": "/api/data/1"},
		Body:       []byte(`{"id":1}`),
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

func TestHTTPRequest_EmptyFields(t *testing.T) {
	req := HTTPRequest{
		Method: "GET",
		Path:   "/",
	}

	data, err := json.Marshal(req)
	require.NoError(t, err)

	var decoded HTTPRequest
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, "GET", decoded.Method)
	assert.Equal(t, "/", decoded.Path)
	assert.Empty(t, decoded.Query)
	assert.Nil(t, decoded.Headers)
	assert.Nil(t, decoded.Body)
}

func TestTunnelMessage_EmptyData(t *testing.T) {
	msg := TunnelMessage{
		Type: "ping",
	}

	data, err := json.Marshal(msg)
	require.NoError(t, err)

	var decoded TunnelMessage
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, "ping", decoded.Type)
	assert.Empty(t, decoded.RequestID)
	assert.Empty(t, decoded.TunnelID)
}

// --- TCPMessage serialization tests ---

func TestTCPMessage_Serialization(t *testing.T) {
	msg := TCPMessage{
		ConnectionID: "conn-123",
		Data:         []byte("raw tcp data here"),
	}

	data, err := json.Marshal(msg)
	require.NoError(t, err)

	var decoded TCPMessage
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, msg.ConnectionID, decoded.ConnectionID)
	assert.Equal(t, msg.Data, decoded.Data)
}

// --- TunnelRegistry tests ---

func TestTunnelRegistry_AddAndLookup(t *testing.T) {
	registry := &TunnelRegistry{
		tunnels: make(map[string]*Tunnel),
	}

	tunnel := &Tunnel{
		ID:         "t-1",
		ClientID:   "client-1",
		Protocol:   "http",
		Subdomain:  "myapp",
		TargetPort: 8080,
		CreateTime: time.Now(),
	}
	tunnel.SetLastActive(time.Now())

	registry.mu.Lock()
	registry.tunnels["t-1"] = tunnel
	registry.mu.Unlock()

	registry.mu.RLock()
	got, exists := registry.tunnels["t-1"]
	registry.mu.RUnlock()

	assert.True(t, exists)
	assert.Equal(t, tunnel, got)
}

func TestTunnelRegistry_SubdomainLookup(t *testing.T) {
	registry := &TunnelRegistry{
		tunnels: make(map[string]*Tunnel),
	}

	registry.mu.Lock()
	registry.tunnels["t-1"] = &Tunnel{ID: "t-1", Subdomain: "app1"}
	registry.tunnels["t-2"] = &Tunnel{ID: "t-2", Subdomain: "app2"}
	registry.tunnels["t-3"] = &Tunnel{ID: "t-3", Subdomain: "app3"}
	registry.mu.Unlock()

	// Look up by subdomain (same pattern as handleTunnelRequest)
	registry.mu.RLock()
	var found *Tunnel
	for _, t := range registry.tunnels {
		if t.Subdomain == "app2" {
			found = t
			break
		}
	}
	registry.mu.RUnlock()

	require.NotNil(t, found)
	assert.Equal(t, "t-2", found.ID)
}

func TestTunnelRegistry_ConcurrentAccess(t *testing.T) {
	registry := &TunnelRegistry{
		tunnels: make(map[string]*Tunnel),
	}

	var wg sync.WaitGroup

	// Concurrent writes
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			tunnelID := "tunnel-" + string(rune('A'+id%26))
			registry.mu.Lock()
			registry.tunnels[tunnelID] = &Tunnel{
				ID:       tunnelID,
				ClientID: "client-" + tunnelID,
			}
			registry.mu.Unlock()
		}(i)
	}

	// Concurrent reads
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			tunnelID := "tunnel-" + string(rune('A'+id%26))
			registry.mu.RLock()
			_ = registry.tunnels[tunnelID]
			registry.mu.RUnlock()
		}(i)
	}

	wg.Wait()
}

// --- WebSocketTunnel sendMessage tests (nil conn) ---

func TestWebSocketTunnel_SendMessage_Closed(t *testing.T) {
	wsTunnel := &WebSocketTunnel{
		ID:     "ws-1",
		closed: true,
	}

	msg := TunnelMessage{Type: "test"}
	err := wsTunnel.sendMessage(msg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "closed")
}

func TestWebSocketTunnel_SendMessage_NilConn(t *testing.T) {
	wsTunnel := &WebSocketTunnel{
		ID:   "ws-1",
		Conn: nil,
	}

	msg := TunnelMessage{Type: "test"}
	err := wsTunnel.sendMessage(msg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "nil")
}

// --- TCPTunnel Close tests ---

func TestTCPTunnel_Close_AlreadyClosed(t *testing.T) {
	tcpTunnel := &TCPTunnel{
		TunnelID:    "t-1",
		connections: make(map[string]*TCPConnection),
		closed:      true,
	}

	err := tcpTunnel.Close()
	assert.NoError(t, err)
}

func TestTCPConnection_Close_AlreadyClosed(t *testing.T) {
	conn := &TCPConnection{
		ID:     "c-1",
		closed: true,
	}

	err := conn.Close()
	assert.NoError(t, err)
}

func TestTCPConnection_Close_NilConn(t *testing.T) {
	conn := &TCPConnection{
		ID:   "c-1",
		conn: nil,
	}

	err := conn.Close()
	assert.NoError(t, err)
	assert.True(t, conn.closed)
}
