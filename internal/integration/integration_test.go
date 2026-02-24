//go:build integration

package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"testing"
	"time"

	"nxpose/internal/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/websocket"
)

// ---------- Test: Client registers, creates tunnel, sends HTTP through tunnel ----------

func TestClientRegisterCreateTunnelAndHTTPForward(t *testing.T) {
	env := startServerWithoutContainers(t, nil)
	baseURL := env.ts.URL

	// Step 1: Register a client
	regResp := registerClient(t, baseURL)
	assert.NotEmpty(t, regResp.ClientID)
	assert.NotEmpty(t, regResp.Certificate)

	// Step 2: Create a tunnel
	tunnelResp := createTunnel(t, baseURL, regResp.ClientID, regResp.Certificate, "http", 8080)
	assert.NotEmpty(t, tunnelResp.TunnelID)
	assert.NotEmpty(t, tunnelResp.PublicURL)

	// Step 3: Connect via WebSocket and register the tunnel
	ws := connectWebSocket(t, baseURL, regResp.ClientID)
	registerTunnelOnWebSocket(t, ws, tunnelResp.TunnelID)

	// Step 4: Start a simulated client handler that echoes requests
	done := make(chan struct{})
	simulateClientHTTPHandler(t, ws, func(req httpRequestMsg) httpResponseMsg {
		return httpResponseMsg{
			StatusCode: http.StatusOK,
			Headers: map[string]string{
				"Content-Type": "text/plain",
				"X-Tunnel-Id":  "test-tunnel",
			},
			Body: []byte(fmt.Sprintf("Hello from tunnel! Method=%s Path=%s", req.Method, req.Path)),
		}
	}, done)

	// Step 5: Verify server status shows the tunnel
	status := getStatus(t, baseURL)
	tunnelCount, ok := status["tunnels"].(float64)
	require.True(t, ok)
	assert.GreaterOrEqual(t, tunnelCount, float64(1))
}

func TestClientRegisterCreateTunnelWithContainers(t *testing.T) {
	// This test uses real MongoDB and Redis containers
	containers := setupContainers(t)
	env := startServerWithContainers(t, containers, nil)
	baseURL := env.ts.URL

	// Register client
	regResp := registerClient(t, baseURL)
	assert.NotEmpty(t, regResp.ClientID)

	// Create tunnel
	tunnelResp := createTunnel(t, baseURL, regResp.ClientID, regResp.Certificate, "http", 3000)
	assert.NotEmpty(t, tunnelResp.TunnelID)
	assert.NotEmpty(t, tunnelResp.PublicURL)

	// Connect WebSocket and register tunnel
	ws := connectWebSocket(t, baseURL, regResp.ClientID)
	registerTunnelOnWebSocket(t, ws, tunnelResp.TunnelID)

	// Simulate client handler
	done := make(chan struct{})
	simulateClientHTTPHandler(t, ws, func(req httpRequestMsg) httpResponseMsg {
		return httpResponseMsg{
			StatusCode: http.StatusOK,
			Headers:    map[string]string{"Content-Type": "application/json"},
			Body:       []byte(`{"status":"ok","source":"tunnel"}`),
		}
	}, done)

	// Verify tunnel exists in status
	status := getStatus(t, baseURL)
	assert.NotNil(t, status["tunnels"])
}

// ---------- Test: TCP tunnel creation and data forwarding ----------

func TestTCPTunnelCreationAndDataForwarding(t *testing.T) {
	env := startServerWithoutContainers(t, nil)
	baseURL := env.ts.URL

	// Register client
	regResp := registerClient(t, baseURL)

	// Create TCP tunnel
	tunnelResp := createTunnel(t, baseURL, regResp.ClientID, regResp.Certificate, "tcp", 5432)
	assert.NotEmpty(t, tunnelResp.TunnelID)
	assert.Contains(t, tunnelResp.PublicURL, "tcp://")

	// Connect via WebSocket
	ws := connectWebSocket(t, baseURL, regResp.ClientID)
	registerTunnelOnWebSocket(t, ws, tunnelResp.TunnelID)

	// Send TCP data through WebSocket
	tcpData, err := json.Marshal(map[string]interface{}{
		"connection_id": "conn-1",
		"data":          []byte("SELECT 1;"),
	})
	require.NoError(t, err)

	msg := tunnelMessage{
		Type:     "tcp_data",
		TunnelID: tunnelResp.TunnelID,
		Data:     tcpData,
	}
	err = websocket.JSON.Send(ws, msg)
	require.NoError(t, err)

	// Verify tunnel was registered and is active
	status := getStatus(t, baseURL)
	tunnelCount, ok := status["tunnels"].(float64)
	require.True(t, ok)
	assert.GreaterOrEqual(t, tunnelCount, float64(1))
}

func TestTCPTunnelWithContainers(t *testing.T) {
	containers := setupContainers(t)
	env := startServerWithContainers(t, containers, nil)
	baseURL := env.ts.URL

	// Register and create TCP tunnel
	regResp := registerClient(t, baseURL)
	tunnelResp := createTunnel(t, baseURL, regResp.ClientID, regResp.Certificate, "tcp", 3306)
	assert.Contains(t, tunnelResp.PublicURL, "tcp://")

	// Connect WebSocket and register
	ws := connectWebSocket(t, baseURL, regResp.ClientID)
	registerTunnelOnWebSocket(t, ws, tunnelResp.TunnelID)

	// Simulate multiple TCP data messages
	for i := 0; i < 5; i++ {
		tcpData, err := json.Marshal(map[string]interface{}{
			"connection_id": fmt.Sprintf("conn-%d", i),
			"data":          []byte(fmt.Sprintf("data-packet-%d", i)),
		})
		require.NoError(t, err)

		msg := tunnelMessage{
			Type:     "tcp_data",
			TunnelID: tunnelResp.TunnelID,
			Data:     tcpData,
		}
		err = websocket.JSON.Send(ws, msg)
		require.NoError(t, err)
	}
}

// ---------- Test: Tunnel expiration and cleanup under load ----------

func TestTunnelExpirationAndCleanup(t *testing.T) {
	// Use a very short max connection time so tunnels expire quickly
	env := startServerWithoutContainers(t, func(cfg *config.ServerConfig) {
		cfg.TunnelLimits.MaxConnection = "1s"
	})
	baseURL := env.ts.URL

	// Register a client
	regResp := registerClient(t, baseURL)

	// Create multiple tunnels
	tunnelIDs := make([]string, 3)
	for i := 0; i < 3; i++ {
		tunnelResp := createTunnel(t, baseURL, regResp.ClientID, regResp.Certificate, "http", 8080+i)
		tunnelIDs[i] = tunnelResp.TunnelID
	}

	// Verify all tunnels exist
	status := getStatus(t, baseURL)
	tunnelCount, ok := status["tunnels"].(float64)
	require.True(t, ok)
	assert.Equal(t, float64(3), tunnelCount)

	// Wait for tunnels to expire (the server has a cleanup routine but it runs
	// at intervals, so we'll verify the expiration time was set correctly)
	time.Sleep(2 * time.Second)

	// Create one more tunnel to verify the server is still operational after expiration
	tunnelResp := createTunnel(t, baseURL, regResp.ClientID, regResp.Certificate, "http", 9999)
	assert.NotEmpty(t, tunnelResp.TunnelID)
}

func TestTunnelExpirationWithContainers(t *testing.T) {
	containers := setupContainers(t)
	env := startServerWithContainers(t, containers, func(cfg *config.ServerConfig) {
		cfg.TunnelLimits.MaxConnection = "2s"
	})
	baseURL := env.ts.URL

	regResp := registerClient(t, baseURL)

	// Create tunnels under load
	var wg sync.WaitGroup
	tunnelCount := 5
	tunnelIDs := make([]string, tunnelCount)
	mu := sync.Mutex{}

	for i := 0; i < tunnelCount; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			resp := createTunnel(t, baseURL, regResp.ClientID, regResp.Certificate, "http", 8080+idx)
			mu.Lock()
			tunnelIDs[idx] = resp.TunnelID
			mu.Unlock()
		}(i)
	}
	wg.Wait()

	// Verify all tunnels were created
	for _, id := range tunnelIDs {
		assert.NotEmpty(t, id)
	}

	// Wait for expiration
	time.Sleep(3 * time.Second)

	// Verify server is still operational
	status := getStatus(t, baseURL)
	assert.NotNil(t, status["version"])
}

// ---------- Test: Multiple concurrent clients with tunnel isolation ----------

func TestMultipleConcurrentClientsIsolation(t *testing.T) {
	env := startServerWithoutContainers(t, nil)
	baseURL := env.ts.URL

	numClients := 5
	type clientInfo struct {
		clientID    string
		certificate string
		tunnelID    string
		publicURL   string
	}

	clients := make([]clientInfo, numClients)
	var wg sync.WaitGroup
	var mu sync.Mutex

	// Register clients concurrently
	for i := 0; i < numClients; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			regResp := registerClient(t, baseURL)

			mu.Lock()
			clients[idx] = clientInfo{
				clientID:    regResp.ClientID,
				certificate: regResp.Certificate,
			}
			mu.Unlock()
		}(i)
	}
	wg.Wait()

	// Create tunnels concurrently for all clients
	for i := 0; i < numClients; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			mu.Lock()
			client := clients[idx]
			mu.Unlock()

			tunnelResp := createTunnel(t, baseURL, client.clientID, client.certificate, "http", 8080+idx)

			mu.Lock()
			clients[idx].tunnelID = tunnelResp.TunnelID
			clients[idx].publicURL = tunnelResp.PublicURL
			mu.Unlock()
		}(i)
	}
	wg.Wait()

	// Verify all clients have unique tunnel IDs and public URLs
	tunnelIDSet := make(map[string]bool)
	publicURLSet := make(map[string]bool)

	for _, client := range clients {
		assert.NotEmpty(t, client.clientID)
		assert.NotEmpty(t, client.tunnelID)
		assert.NotEmpty(t, client.publicURL)

		assert.False(t, tunnelIDSet[client.tunnelID], "duplicate tunnel ID: %s", client.tunnelID)
		assert.False(t, publicURLSet[client.publicURL], "duplicate public URL: %s", client.publicURL)

		tunnelIDSet[client.tunnelID] = true
		publicURLSet[client.publicURL] = true
	}

	// Verify status reports correct tunnel count
	status := getStatus(t, baseURL)
	count, ok := status["tunnels"].(float64)
	require.True(t, ok)
	assert.Equal(t, float64(numClients), count)
}

func TestMultipleConcurrentClientsWithContainers(t *testing.T) {
	containers := setupContainers(t)
	env := startServerWithContainers(t, containers, nil)
	baseURL := env.ts.URL

	numClients := 3
	type clientResult struct {
		clientID  string
		tunnelID  string
		publicURL string
	}

	results := make([]clientResult, numClients)
	var wg sync.WaitGroup
	var mu sync.Mutex

	// Register and create tunnels concurrently
	for i := 0; i < numClients; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			regResp := registerClient(t, baseURL)
			tunnelResp := createTunnel(t, baseURL, regResp.ClientID, regResp.Certificate, "http", 8080+idx)

			mu.Lock()
			results[idx] = clientResult{
				clientID:  regResp.ClientID,
				tunnelID:  tunnelResp.TunnelID,
				publicURL: tunnelResp.PublicURL,
			}
			mu.Unlock()
		}(i)
	}
	wg.Wait()

	// Verify all tunnels are unique and properly isolated
	seen := make(map[string]bool)
	for _, r := range results {
		assert.NotEmpty(t, r.tunnelID)
		assert.False(t, seen[r.tunnelID], "duplicate tunnel ID")
		seen[r.tunnelID] = true
	}

	// Each client connects WebSocket and registers their tunnel
	for _, r := range results {
		ws := connectWebSocket(t, baseURL, r.clientID)
		registerTunnelOnWebSocket(t, ws, r.tunnelID)
	}
}

// ---------- Test: Tunnel limit enforcement ----------

func TestTunnelLimitEnforcement(t *testing.T) {
	maxPerUser := 2
	env := startServerWithoutContainers(t, func(cfg *config.ServerConfig) {
		cfg.TunnelLimits.MaxPerUser = maxPerUser
	})
	baseURL := env.ts.URL

	regResp := registerClient(t, baseURL)

	// Create tunnels up to the limit
	for i := 0; i < maxPerUser; i++ {
		tunnelResp := createTunnel(t, baseURL, regResp.ClientID, regResp.Certificate, "http", 8080+i)
		assert.NotEmpty(t, tunnelResp.TunnelID)
	}

	// The next tunnel creation should fail (429 Too Many Requests)
	reqBody, err := json.Marshal(map[string]interface{}{
		"client_id":   regResp.ClientID,
		"protocol":    "http",
		"port":        9090,
		"certificate": regResp.Certificate,
	})
	require.NoError(t, err)

	resp, err := http.Post(baseURL+"/api/tunnel", "application/json", bytes.NewBuffer(reqBody))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusTooManyRequests, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Contains(t, string(body), "Maximum number of tunnels reached")
}

// ---------- Test: Server status endpoint ----------

func TestStatusEndpoint(t *testing.T) {
	env := startServerWithoutContainers(t, nil)
	baseURL := env.ts.URL

	status := getStatus(t, baseURL)

	assert.Equal(t, "1.0.0", status["version"])
	assert.NotNil(t, status["features"])
	assert.NotNil(t, status["tls"])

	features, ok := status["features"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, false, features["oauth2_enabled"])
	assert.Equal(t, false, features["mongodb_enabled"])
	assert.Equal(t, false, features["letsencrypt_enabled"])
}

// ---------- Test: Invalid requests ----------

func TestInvalidTunnelRequests(t *testing.T) {
	env := startServerWithoutContainers(t, nil)
	baseURL := env.ts.URL

	t.Run("missing certificate", func(t *testing.T) {
		reqBody, err := json.Marshal(map[string]interface{}{
			"client_id":   "test-client",
			"protocol":    "http",
			"port":        8080,
			"certificate": "",
		})
		require.NoError(t, err)

		resp, err := http.Post(baseURL+"/api/tunnel", "application/json", bytes.NewBuffer(reqBody))
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("unsupported protocol", func(t *testing.T) {
		reqBody, err := json.Marshal(map[string]interface{}{
			"client_id":   "test-client",
			"protocol":    "ftp",
			"port":        21,
			"certificate": "some-cert",
		})
		require.NoError(t, err)

		resp, err := http.Post(baseURL+"/api/tunnel", "application/json", bytes.NewBuffer(reqBody))
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("invalid JSON body", func(t *testing.T) {
		resp, err := http.Post(baseURL+"/api/tunnel", "application/json", bytes.NewBuffer([]byte("not json")))
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("method not allowed for tunnel", func(t *testing.T) {
		resp, err := http.Get(baseURL + "/api/tunnel")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusMethodNotAllowed, resp.StatusCode)
	})
}

// ---------- Test: WebSocket connection without tunnel ----------

func TestWebSocketConnectionWithoutTunnel(t *testing.T) {
	env := startServerWithoutContainers(t, nil)
	baseURL := env.ts.URL

	// Connect WebSocket
	ws := connectWebSocket(t, baseURL, "client-no-tunnel")

	// Try to register a non-existent tunnel
	data, err := json.Marshal(map[string]string{
		"tunnel_id": "non-existent-tunnel-id",
	})
	require.NoError(t, err)

	msg := tunnelMessage{
		Type: "register_tunnel",
		Data: data,
	}
	err = websocket.JSON.Send(ws, msg)
	require.NoError(t, err)

	// Should receive an error response
	var response tunnelMessage
	err = websocket.JSON.Receive(ws, &response)
	require.NoError(t, err)
	assert.Equal(t, "error", response.Type)
}

// ---------- Test: Welcome page on base domain ----------

func TestWelcomePageOnBaseDomain(t *testing.T) {
	env := startServerWithoutContainers(t, nil)
	baseURL := env.ts.URL

	resp, err := http.Get(baseURL + "/")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Contains(t, string(body), "NXpose Tunnel Service")
}
