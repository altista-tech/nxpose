//go:build integration

package integration

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"nxpose/internal/config"
	"nxpose/internal/logger"
	"nxpose/internal/server"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcmongo "github.com/testcontainers/testcontainers-go/modules/mongodb"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
	"golang.org/x/net/websocket"
)

// testEnv holds references to containers and the server for integration tests.
type testEnv struct {
	mongoContainer *tcmongo.MongoDBContainer
	redisContainer *tcredis.RedisContainer
	mongoURI       string
	redisHost      string
	redisPort      int
	srv            *server.Server
	ts             *httptest.Server
	cfg            *config.ServerConfig
}

// setupContainers starts MongoDB and Redis containers and returns a testEnv.
func setupContainers(t *testing.T) *testEnv {
	t.Helper()
	ctx := context.Background()

	// Start MongoDB container
	mongoC, err := tcmongo.Run(ctx, "mongo:7")
	require.NoError(t, err, "failed to start MongoDB container")

	mongoURI, err := mongoC.ConnectionString(ctx)
	require.NoError(t, err, "failed to get MongoDB connection string")

	// Start Redis container
	redisC, err := tcredis.Run(ctx, "redis:7-alpine")
	require.NoError(t, err, "failed to start Redis container")

	redisEndpoint, err := redisC.Endpoint(ctx, "")
	require.NoError(t, err, "failed to get Redis endpoint")

	// Parse Redis host:port
	parts := strings.SplitN(redisEndpoint, ":", 2)
	require.Len(t, parts, 2, "expected host:port format for Redis endpoint: %s", redisEndpoint)
	redisHost := parts[0]
	redisPort, err := strconv.Atoi(parts[1])
	require.NoError(t, err, "failed to parse Redis port from endpoint: %s", redisEndpoint)

	env := &testEnv{
		mongoContainer: mongoC,
		redisContainer: redisC,
		mongoURI:       mongoURI,
		redisHost:      redisHost,
		redisPort:      redisPort,
	}

	t.Cleanup(func() {
		if mongoC != nil {
			testcontainers.CleanupContainer(t, mongoC)
		}
		if redisC != nil {
			testcontainers.CleanupContainer(t, redisC)
		}
	})

	return env
}

// startServerWithContainers creates a server connected to container services and
// starts it as an httptest server. Returns the testEnv with the running server.
func startServerWithContainers(t *testing.T, env *testEnv, cfgModifier func(*config.ServerConfig)) *testEnv {
	t.Helper()

	cfg := config.DefaultServerConfig()
	cfg.BindAddress = "127.0.0.1"
	cfg.Port = 0
	cfg.BaseDomain = "localhost"

	cfg.MongoDB.Enabled = true
	cfg.MongoDB.URI = env.mongoURI
	cfg.MongoDB.Database = fmt.Sprintf("nxpose_test_%d", time.Now().UnixNano())
	cfg.MongoDB.Timeout = 10 * time.Second

	cfg.Redis.Enabled = true
	cfg.Redis.Host = env.redisHost
	cfg.Redis.Port = env.redisPort
	cfg.Redis.KeyPrefix = fmt.Sprintf("nxpose_test_%d:", time.Now().UnixNano())
	cfg.Redis.Timeout = 10 * time.Second

	cfg.OAuth2.Enabled = false
	cfg.LetsEncrypt.Enabled = false
	cfg.TunnelLimits.MaxPerUser = 10
	cfg.TunnelLimits.MaxConnection = ""

	if cfgModifier != nil {
		cfgModifier(cfg)
	}

	env.cfg = cfg

	log := logger.New(true)
	tlsConfig := &tls.Config{
		InsecureSkipVerify: true,
	}

	srv, err := server.NewServer(cfg, tlsConfig, log)
	require.NoError(t, err, "failed to create server")
	env.srv = srv

	// Start an httptest server using the server's Handler
	ts := httptest.NewServer(srv.Handler())
	env.ts = ts

	t.Cleanup(func() {
		ts.Close()
	})

	return env
}

// startServerWithoutContainers creates a minimal server without external services
// for tests that only need core server functionality (no MongoDB/Redis).
func startServerWithoutContainers(t *testing.T, cfgModifier func(*config.ServerConfig)) *testEnv {
	t.Helper()

	cfg := config.DefaultServerConfig()
	cfg.BindAddress = "127.0.0.1"
	cfg.Port = 0
	cfg.BaseDomain = "localhost"
	cfg.MongoDB.Enabled = false
	cfg.Redis.Enabled = false
	cfg.OAuth2.Enabled = false
	cfg.LetsEncrypt.Enabled = false
	cfg.TunnelLimits.MaxPerUser = 10
	cfg.TunnelLimits.MaxConnection = ""

	if cfgModifier != nil {
		cfgModifier(cfg)
	}

	log := logger.New(true)
	tlsConfig := &tls.Config{
		InsecureSkipVerify: true,
	}

	srv, err := server.NewServer(cfg, tlsConfig, log)
	require.NoError(t, err, "failed to create server")

	ts := httptest.NewServer(srv.Handler())

	t.Cleanup(func() {
		ts.Close()
	})

	return &testEnv{
		srv: srv,
		ts:  ts,
		cfg: cfg,
	}
}

// registrationResponse mirrors server.RegistrationResponse for test deserialization.
type registrationResponse struct {
	Success     bool      `json:"success"`
	Message     string    `json:"message,omitempty"`
	ClientID    string    `json:"client_id"`
	Certificate string    `json:"certificate"`
	ExpiresAt   time.Time `json:"expires_at"`
}

// tunnelResponse mirrors server.TunnelResponse for test deserialization.
type tunnelResponse struct {
	Success   bool   `json:"success"`
	Message   string `json:"message,omitempty"`
	TunnelID  string `json:"tunnel_id"`
	PublicURL string `json:"public_url"`
}

// tunnelMessage mirrors server.TunnelMessage for WebSocket communication.
type tunnelMessage struct {
	Type      string          `json:"type"`
	RequestID string          `json:"request_id,omitempty"`
	TunnelID  string          `json:"tunnel_id,omitempty"`
	Data      json.RawMessage `json:"data,omitempty"`
}

// httpRequestMsg mirrors server.HTTPRequest for WebSocket communication.
type httpRequestMsg struct {
	Method  string            `json:"method"`
	Path    string            `json:"path"`
	Query   string            `json:"query,omitempty"`
	Headers map[string]string `json:"headers"`
	Body    []byte            `json:"body,omitempty"`
}

// httpResponseMsg mirrors server.HTTPResponse for WebSocket communication.
type httpResponseMsg struct {
	StatusCode int               `json:"status_code"`
	Headers    map[string]string `json:"headers"`
	Body       []byte            `json:"body,omitempty"`
}

// registerClient sends a registration request and returns the response.
func registerClient(t *testing.T, baseURL string) registrationResponse {
	t.Helper()

	reqBody, err := json.Marshal(map[string]string{
		"client_name":   "test-client",
		"client_region": "us-west",
	})
	require.NoError(t, err)

	resp, err := http.Post(baseURL+"/api/register", "application/json", bytes.NewBuffer(reqBody))
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	var regResp registrationResponse
	err = json.NewDecoder(resp.Body).Decode(&regResp)
	require.NoError(t, err)
	require.True(t, regResp.Success)

	return regResp
}

// createTunnel sends a tunnel creation request and returns the response.
func createTunnel(t *testing.T, baseURL, clientID, certificate, protocol string, port int) tunnelResponse {
	t.Helper()

	reqBody, err := json.Marshal(map[string]interface{}{
		"client_id":   clientID,
		"protocol":    protocol,
		"port":        port,
		"certificate": certificate,
	})
	require.NoError(t, err)

	resp, err := http.Post(baseURL+"/api/tunnel", "application/json", bytes.NewBuffer(reqBody))
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	var tunnelResp tunnelResponse
	err = json.NewDecoder(resp.Body).Decode(&tunnelResp)
	require.NoError(t, err)
	require.True(t, tunnelResp.Success)

	return tunnelResp
}

// connectWebSocket establishes a WebSocket connection to the server and returns the connection.
func connectWebSocket(t *testing.T, baseURL, clientID string) *websocket.Conn {
	t.Helper()

	// Convert http:// to ws://
	wsURL := "ws" + baseURL[4:] + "/api/ws?client_id=" + clientID

	origin := baseURL
	ws, err := websocket.Dial(wsURL, "", origin)
	require.NoError(t, err, "failed to connect websocket")

	// Read welcome message
	var welcome tunnelMessage
	err = websocket.JSON.Receive(ws, &welcome)
	require.NoError(t, err, "failed to receive welcome message")
	require.Equal(t, "welcome", welcome.Type)

	t.Cleanup(func() {
		ws.Close()
	})

	return ws
}

// registerTunnelOnWebSocket sends a register_tunnel message and waits for the response.
func registerTunnelOnWebSocket(t *testing.T, ws *websocket.Conn, tunnelID string) {
	t.Helper()

	data, err := json.Marshal(map[string]string{
		"tunnel_id": tunnelID,
	})
	require.NoError(t, err)

	msg := tunnelMessage{
		Type: "register_tunnel",
		Data: data,
	}

	err = websocket.JSON.Send(ws, msg)
	require.NoError(t, err, "failed to send register_tunnel")

	var response tunnelMessage
	err = websocket.JSON.Receive(ws, &response)
	require.NoError(t, err, "failed to receive tunnel_registered response")
	require.Equal(t, "tunnel_registered", response.Type)
}

// simulateClientHTTPHandler runs in a goroutine, reads HTTP requests from the WebSocket
// and sends back HTTP responses. It serves as the mock local service behind the tunnel.
func simulateClientHTTPHandler(t *testing.T, ws *websocket.Conn, handler func(httpRequestMsg) httpResponseMsg, done chan struct{}) {
	t.Helper()

	go func() {
		defer close(done)
		for {
			var msg tunnelMessage
			if err := websocket.JSON.Receive(ws, &msg); err != nil {
				if err == io.EOF {
					return
				}
				return
			}

			if msg.Type == "http_request" {
				var httpReq httpRequestMsg
				if err := json.Unmarshal(msg.Data, &httpReq); err != nil {
					continue
				}

				// Generate response using the handler
				httpResp := handler(httpReq)

				// Marshal the response
				respData, err := json.Marshal(httpResp)
				if err != nil {
					continue
				}

				// Send response back
				respMsg := tunnelMessage{
					Type:      "http_response",
					RequestID: msg.RequestID,
					TunnelID:  msg.TunnelID,
					Data:      respData,
				}

				if err := websocket.JSON.Send(ws, respMsg); err != nil {
					return
				}
			}
		}
	}()
}

// getStatus fetches the server status endpoint and returns the parsed response.
func getStatus(t *testing.T, baseURL string) map[string]interface{} {
	t.Helper()

	resp, err := http.Get(baseURL + "/api/status")
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	var status map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&status)
	require.NoError(t, err)

	return status
}
