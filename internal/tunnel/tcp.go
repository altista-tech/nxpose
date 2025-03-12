// internal/tunnel/tcp.go
// Implementation of TCP tunneling functionality

package tunnel

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

// TCPTunnel manages TCP connections for tunneling
type TCPTunnel struct {
	ID             string
	LocalPort      int
	RemoteAddr     string
	MaxConnections int
	connections    map[string]*TCPConnection
	tunnel         *Tunnel
	listener       net.Listener
	log            *logrus.Logger
	mu             sync.RWMutex
	stopCh         chan struct{}
	stopped        bool
}

// TCPConnection represents a single TCP connection being tunneled
type TCPConnection struct {
	ID            string
	TunnelID      string
	LocalConn     net.Conn
	RemoteAddr    string
	CreatedAt     time.Time
	LastActivity  time.Time
	BytesSent     int64
	BytesReceived int64
	mu            sync.Mutex
	closed        bool
}

// NewTCPTunnel creates a new TCP tunnel
func NewTCPTunnel(tunnelID string, localPort int, remoteAddr string, tunnel *Tunnel, logger *logrus.Logger) *TCPTunnel {
	return &TCPTunnel{
		ID:             tunnelID,
		LocalPort:      localPort,
		RemoteAddr:     remoteAddr,
		MaxConnections: 100, // Default max connections
		connections:    make(map[string]*TCPConnection),
		tunnel:         tunnel,
		log:            logger,
		stopCh:         make(chan struct{}),
	}
}

// Start starts the TCP tunnel
func (t *TCPTunnel) Start() error {
	// Check if already running
	t.mu.Lock()
	if t.stopped {
		t.mu.Unlock()
		return fmt.Errorf("tunnel is stopped")
	}

	if t.listener != nil {
		t.mu.Unlock()
		return fmt.Errorf("tunnel is already running")
	}

	// Start the local listener
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", t.LocalPort))
	if err != nil {
		t.mu.Unlock()
		return fmt.Errorf("failed to start TCP listener: %w", err)
	}

	t.listener = listener
	t.mu.Unlock()

	t.log.Infof("TCP tunnel listening on localhost:%d", t.LocalPort)

	// Accept connections in a goroutine
	go t.acceptConnections()

	return nil
}

// Stop stops the TCP tunnel
func (t *TCPTunnel) Stop() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.stopped {
		return nil
	}

	// Mark as stopped
	t.stopped = true

	// Close listener
	if t.listener != nil {
		t.listener.Close()
	}

	// Close stop channel
	close(t.stopCh)

	// Close all connections
	for id, conn := range t.connections {
		t.closeConnection(id, conn)
	}

	t.log.Info("TCP tunnel stopped")
	return nil
}

// acceptConnections accepts incoming TCP connections
func (t *TCPTunnel) acceptConnections() {
	for {
		// Check if stopped
		select {
		case <-t.stopCh:
			return
		default:
		}

		// Accept a connection
		conn, err := t.listener.Accept()
		if err != nil {
			if t.isStopped() {
				return
			}
			t.log.Errorf("Error accepting TCP connection: %v", err)
			time.Sleep(100 * time.Millisecond)
			continue
		}

		// Handle connection in a goroutine
		go t.handleConnection(conn)
	}
}

// handleConnection handles a new TCP connection
func (t *TCPTunnel) handleConnection(conn net.Conn) {
	// Create a new connection ID
	connID := uuid.New().String()

	// Create a connection object
	tcpConn := &TCPConnection{
		ID:           connID,
		TunnelID:     t.ID,
		LocalConn:    conn,
		RemoteAddr:   conn.RemoteAddr().String(),
		CreatedAt:    time.Now(),
		LastActivity: time.Now(),
	}

	// Register the connection
	t.mu.Lock()

	// Check max connections
	if len(t.connections) >= t.MaxConnections {
		t.mu.Unlock()
		conn.Close()
		t.log.Warnf("Rejected TCP connection: maximum connections reached (%d)", t.MaxConnections)
		return
	}

	t.connections[connID] = tcpConn
	t.mu.Unlock()

	t.log.Infof("Accepted TCP connection from %s (id: %s)", conn.RemoteAddr(), connID)

	// Start forwarding data
	go t.forwardLocalToRemote(tcpConn)
}

// forwardLocalToRemote forwards data from the local connection to the remote endpoint
func (t *TCPTunnel) forwardLocalToRemote(conn *TCPConnection) {
	defer t.closeConnection(conn.ID, conn)

	buffer := make([]byte, 4096)

	for {
		// Check if stopped
		select {
		case <-t.stopCh:
			return
		default:
		}

		// Read from local connection
		n, err := conn.LocalConn.Read(buffer)
		if err != nil {
			if err != io.EOF {
				t.log.Errorf("Error reading from local connection: %v", err)
			}
			return
		}

		// Update activity timestamp
		conn.mu.Lock()
		conn.LastActivity = time.Now()
		conn.BytesReceived += int64(n)
		conn.mu.Unlock()

		// Copy data to send
		data := make([]byte, n)
		copy(data, buffer[:n])

		// Create TCP message
		tcpMsg := TCPMessage{
			ConnectionID: conn.ID,
			Data:         data,
		}

		// Marshal message
		msgData, err := json.Marshal(tcpMsg)
		if err != nil {
			t.log.Errorf("Error marshaling TCP message: %v", err)
			return
		}

		// Create tunnel message
		tunnelMsg := TunnelMessage{
			Type:      "tcp_data",
			RequestID: uuid.New().String(),
			TunnelID:  t.ID,
			Data:      msgData,
		}

		// Send to server via the WebSocket tunnel
		err = t.tunnel.sendMessage(tunnelMsg)
		if err != nil {
			t.log.Errorf("Error sending TCP data to server: %v", err)
			return
		}

		// Update bytes sent
		conn.mu.Lock()
		conn.BytesSent += int64(n)
		conn.mu.Unlock()

		t.log.Debugf("Forwarded %d bytes from local to remote (conn: %s)", n, conn.ID)
	}
}

// HandleRemoteData handles data received from the remote endpoint
func (t *TCPTunnel) HandleRemoteData(connectionID string, data []byte) error {
	// Find the connection
	t.mu.RLock()
	conn, exists := t.connections[connectionID]
	t.mu.RUnlock()

	if !exists {
		return fmt.Errorf("TCP connection not found: %s", connectionID)
	}

	// Check if connection is closed
	conn.mu.Lock()
	if conn.closed {
		conn.mu.Unlock()
		return fmt.Errorf("TCP connection is closed")
	}

	// Update activity timestamp
	conn.LastActivity = time.Now()
	conn.mu.Unlock()

	// Write data to local connection
	n, err := conn.LocalConn.Write(data)
	if err != nil {
		t.log.Errorf("Error writing to local connection: %v", err)
		t.closeConnection(connectionID, conn)
		return err
	}

	// Update bytes received
	conn.mu.Lock()
	conn.BytesReceived += int64(n)
	conn.mu.Unlock()

	t.log.Debugf("Forwarded %d bytes from remote to local (conn: %s)", n, connectionID)
	return nil
}

// closeConnection closes and removes a TCP connection
func (t *TCPTunnel) closeConnection(id string, conn *TCPConnection) {
	conn.mu.Lock()
	if conn.closed {
		conn.mu.Unlock()
		return
	}
	conn.closed = true
	conn.mu.Unlock()

	// Close local connection
	conn.LocalConn.Close()

	// Remove from the connections map
	t.mu.Lock()
	delete(t.connections, id)
	t.mu.Unlock()

	t.log.Infof("Closed TCP connection %s (remote: %s, bytes sent: %d, bytes received: %d)",
		id, conn.RemoteAddr, conn.BytesSent, conn.BytesReceived)
}

// isStopped checks if the tunnel is stopped
func (t *TCPTunnel) isStopped() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.stopped
}

// GetConnections returns information about active connections
func (t *TCPTunnel) GetConnections() []map[string]interface{} {
	t.mu.RLock()
	defer t.mu.RUnlock()

	connections := make([]map[string]interface{}, 0, len(t.connections))

	for id, conn := range t.connections {
		conn.mu.Lock()
		connInfo := map[string]interface{}{
			"id":             id,
			"remote_addr":    conn.RemoteAddr,
			"created_at":     conn.CreatedAt,
			"last_activity":  conn.LastActivity,
			"bytes_sent":     conn.BytesSent,
			"bytes_received": conn.BytesReceived,
			"idle_time":      time.Since(conn.LastActivity).String(),
		}
		conn.mu.Unlock()

		connections = append(connections, connInfo)
	}

	return connections
}
