package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

// TCPTunnel manages TCP connections for a tunnel
type TCPTunnel struct {
	TunnelID    string
	Port        int
	listener    net.Listener
	connections map[string]*TCPConnection
	server      *Server
	mu          sync.Mutex
	closed      bool
}

// TCPConnection represents a single TCP connection through the tunnel
type TCPConnection struct {
	ID         string
	TunnelID   string
	ClientID   string
	conn       net.Conn
	wsConn     *WebSocketTunnel
	createTime time.Time
	lastActive time.Time
	closed     bool
	mu         sync.Mutex
}

// TCPMessage represents a TCP data message tunneled over WebSocket
type TCPMessage struct {
	ConnectionID string `json:"connection_id"`
	Data         []byte `json:"data,omitempty"`
}

// StartTCPListener starts a TCP listener for the given tunnel
func (s *Server) StartTCPListener(tunnelID string, port int) (*TCPTunnel, error) {
	// Check if tunnel exists
	s.tunnels.mu.RLock()
	_, exists := s.tunnels.tunnels[tunnelID]
	s.tunnels.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("tunnel not found: %s", tunnelID)
	}

	// Create TCP listener
	addr := fmt.Sprintf("%s:%d", s.config.BindAddress, port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("failed to start TCP listener: %w", err)
	}

	s.log.WithFields(logrus.Fields{
		"tunnel_id": tunnelID,
		"port":      port,
		"address":   addr,
	}).Info("Started TCP listener for tunnel")

	// Create TCP tunnel
	tcpTunnel := &TCPTunnel{
		TunnelID:    tunnelID,
		Port:        port,
		listener:    listener,
		connections: make(map[string]*TCPConnection),
		server:      s,
	}

	// Start accepting connections
	go tcpTunnel.acceptConnections()

	return tcpTunnel, nil
}

// acceptConnections accepts incoming TCP connections
func (t *TCPTunnel) acceptConnections() {
	for {
		// Accept connection
		conn, err := t.listener.Accept()
		if err != nil {
			// Check if the listener was closed
			t.mu.Lock()
			if t.closed {
				t.mu.Unlock()
				return
			}
			t.mu.Unlock()

			t.server.log.WithError(err).WithField("tunnel_id", t.TunnelID).Error("Error accepting TCP connection")
			continue
		}

		// Create a new connection handler
		connID := uuid.New().String()
		tcpConn := &TCPConnection{
			ID:         connID,
			TunnelID:   t.TunnelID,
			conn:       conn,
			createTime: time.Now(),
			lastActive: time.Now(),
		}

		// Store the connection
		t.mu.Lock()
		t.connections[connID] = tcpConn
		t.mu.Unlock()

		t.server.log.WithFields(logrus.Fields{
			"tunnel_id":     t.TunnelID,
			"connection_id": connID,
			"remote_addr":   conn.RemoteAddr().String(),
		}).Info("Accepted new TCP connection")

		// Handle the connection in a goroutine
		go t.handleConnection(tcpConn)
	}
}

// handleConnection processes data from a TCP connection
func (t *TCPTunnel) handleConnection(conn *TCPConnection) {
	// Buffer for reading from TCP connection
	buffer := make([]byte, 4096)

	// Get the tunnel
	t.server.tunnels.mu.RLock()
	tunnel := t.server.tunnels.tunnels[t.TunnelID]
	t.server.tunnels.mu.RUnlock()

	if tunnel == nil {
		t.server.log.WithField("tunnel_id", t.TunnelID).Error("Tunnel not found")
		conn.Close()
		return
	}

	// In a real implementation, we would find the WebSocket connection for this tunnel
	// For demonstration, we'll simulate it

	// Read from the TCP connection and forward to WebSocket
	for {
		n, err := conn.conn.Read(buffer)
		if err != nil {
			if err != io.EOF {
				t.server.log.WithError(err).WithFields(logrus.Fields{
					"tunnel_id":     t.TunnelID,
					"connection_id": conn.ID,
				}).Error("Error reading from TCP connection")
			}
			break
		}

		// Update last active time
		conn.mu.Lock()
		conn.lastActive = time.Now()
		conn.mu.Unlock()

		// Copy data to prevent buffer reuse issues
		data := make([]byte, n)
		copy(data, buffer[:n])

		// Forward data to WebSocket (if connected)
		if conn.wsConn != nil {
			// Create TCP data message
			tcpMessage := TCPMessage{
				ConnectionID: conn.ID,
				Data:         data,
			}

			// Marshal message data
			tcpData, err := json.Marshal(tcpMessage)
			if err != nil {
				t.server.log.WithError(err).Error("Failed to marshal TCP message")
				continue
			}

			// Create tunnel message
			message := TunnelMessage{
				Type:     "tcp_data",
				TunnelID: t.TunnelID,
				Data:     tcpData,
			}

			// Send message
			if err := conn.wsConn.sendMessage(message); err != nil {
				t.server.log.WithError(err).Error("Failed to send TCP data to client")
				break
			}

			t.server.log.WithFields(logrus.Fields{
				"tunnel_id":     t.TunnelID,
				"connection_id": conn.ID,
				"data_size":     n,
			}).Debug("Forwarded TCP data to client")
		} else {
			// WebSocket not connected, buffer data or close connection
			t.server.log.WithField("tunnel_id", t.TunnelID).Warn("No WebSocket connection available for TCP tunnel")
			break
		}
	}

	// Close the connection
	conn.Close()

	// Remove from active connections
	t.mu.Lock()
	delete(t.connections, conn.ID)
	t.mu.Unlock()

	t.server.log.WithFields(logrus.Fields{
		"tunnel_id":     t.TunnelID,
		"connection_id": conn.ID,
	}).Info("TCP connection closed")
}

// Close closes the TCP tunnel and all its connections
func (t *TCPTunnel) Close() error {
	t.mu.Lock()

	if t.closed {
		t.mu.Unlock()
		return nil
	}

	// Mark as closed
	t.closed = true

	// Close listener
	if t.listener != nil {
		t.listener.Close()
	}

	// Collect connections and clear the map under the lock,
	// then close connections outside the lock to avoid potential
	// deadlock with handleConnection which may hold conn.mu then acquire t.mu.
	conns := make([]*TCPConnection, 0, len(t.connections))
	for _, conn := range t.connections {
		conns = append(conns, conn)
	}
	t.connections = make(map[string]*TCPConnection)
	t.mu.Unlock()

	for _, conn := range conns {
		conn.Close()
	}

	t.server.log.WithField("tunnel_id", t.TunnelID).Info("TCP tunnel closed")

	return nil
}

// Close closes a TCP connection
func (c *TCPConnection) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}

	// Mark as closed
	c.closed = true

	// Close the connection
	if c.conn != nil {
		c.conn.Close()
	}

	return nil
}
