// internal/tunnel/tunnel_manager.go
// Complete implementation of the tunnel manager

package tunnel

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

// TunnelManager maintains and controls multiple active tunnels
type TunnelManager struct {
	tunnels           map[string]*ManagedTunnel
	userTunnels       map[string]map[string]struct{} // Maps user ID to set of tunnel IDs
	mu                sync.RWMutex
	log               *logrus.Logger
	configDir         string
	maxTunnels        int
	maxTunnelsPerUser int
	maxConnectionTime time.Duration
	ctx               context.Context
	cancel            context.CancelFunc
	stopWait          sync.WaitGroup
	redisClient       interface{} // Redis client for tracking user tunnels, if available
}

// ManagedTunnel represents a tunnel being managed
type ManagedTunnel struct {
	ID            string
	UserID        string // User who created the tunnel
	Protocol      string
	LocalPort     int
	PublicURL     string
	ServerHost    string
	ServerPort    int
	Created       time.Time
	LastActive    time.Time
	Active        bool
	ExpiresAt     time.Time // Time when the tunnel will expire
	CertData      []byte
	tunnel        *Tunnel
	reconnectChan chan struct{}
	stopChan      chan struct{}
}

// TunnelInfo provides public information about a managed tunnel
type TunnelInfo struct {
	ID         string    `json:"id"`
	UserID     string    `json:"user_id"`
	Protocol   string    `json:"protocol"`
	LocalPort  int       `json:"local_port"`
	PublicURL  string    `json:"public_url"`
	ServerHost string    `json:"server_host"`
	ServerPort int       `json:"server_port"`
	Created    time.Time `json:"created"`
	LastActive time.Time `json:"last_active"`
	Active     bool      `json:"active"`
	ExpiresAt  time.Time `json:"expires_at,omitempty"`
}

// TunnelSaveData represents the data saved to disk for persistence
type TunnelSaveData struct {
	Tunnels []ManagedTunnelData `json:"tunnels"`
}

// ManagedTunnelData represents the serializable data for a managed tunnel
type ManagedTunnelData struct {
	ID         string    `json:"id"`
	UserID     string    `json:"user_id"`
	Protocol   string    `json:"protocol"`
	LocalPort  int       `json:"local_port"`
	PublicURL  string    `json:"public_url"`
	ServerHost string    `json:"server_host"`
	ServerPort int       `json:"server_port"`
	Created    time.Time `json:"created"`
	LastActive time.Time `json:"last_active"`
	ExpiresAt  time.Time `json:"expires_at,omitempty"`
	CertPath   string    `json:"cert_path,omitempty"`
}

// NewTunnelManager creates a new tunnel manager
func NewTunnelManager(configDir string, maxTunnels int, maxTunnelsPerUser int, maxConnectionTime string) *TunnelManager {
	ctx, cancel := context.WithCancel(context.Background())

	// Parse max connection time
	var duration time.Duration
	if maxConnectionTime != "" {
		var err error
		duration, err = time.ParseDuration(maxConnectionTime)
		if err != nil {
			// Log error but continue with zero duration (no limit)
			logrus.Warnf("Invalid max connection time format: %s, using no limit", maxConnectionTime)
			duration = 0
		}
	}

	manager := &TunnelManager{
		tunnels:           make(map[string]*ManagedTunnel),
		userTunnels:       make(map[string]map[string]struct{}),
		log:               logrus.New(),
		configDir:         configDir,
		maxTunnels:        maxTunnels,
		maxTunnelsPerUser: maxTunnelsPerUser,
		maxConnectionTime: duration,
		ctx:               ctx,
		cancel:            cancel,
	}

	// Configure logger
	manager.log.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})

	if os.Getenv("NXPOSE_DEBUG") == "1" {
		manager.log.SetLevel(logrus.DebugLevel)
	} else {
		manager.log.SetLevel(logrus.InfoLevel)
	}

	// Ensure config directory exists
	if configDir != "" {
		if err := os.MkdirAll(configDir, 0755); err != nil {
			manager.log.Warnf("Failed to create config directory: %v", err)
		}
	}

	// Start background tasks
	manager.startBackgroundTasks()

	return manager
}

// startBackgroundTasks starts background maintenance tasks
func (tm *TunnelManager) startBackgroundTasks() {
	// Start cleanup routine
	tm.stopWait.Add(1)
	go func() {
		defer tm.stopWait.Done()
		tm.cleanupRoutine()
	}()

	// Start reconnect routine
	tm.stopWait.Add(1)
	go func() {
		defer tm.stopWait.Done()
		tm.reconnectRoutine()
	}()

	// Start autosave routine
	if tm.configDir != "" {
		tm.stopWait.Add(1)
		go func() {
			defer tm.stopWait.Done()
			tm.autosaveRoutine()
		}()
	}
}

// cleanupRoutine periodically cleans up stale tunnels
func (tm *TunnelManager) cleanupRoutine() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			tm.cleanupStaleTunnels()
		case <-tm.ctx.Done():
			return
		}
	}
}

// reconnectRoutine attempts to reconnect failed tunnels
func (tm *TunnelManager) reconnectRoutine() {
	for {
		select {
		case <-tm.ctx.Done():
			return
		default:
			// Check for tunnels needing reconnection
			tm.mu.RLock()
			var reconnectChans []chan struct{}
			for _, tunnel := range tm.tunnels {
				if tunnel.Active {
					continue
				}
				reconnectChans = append(reconnectChans, tunnel.reconnectChan)
			}
			tm.mu.RUnlock()

			// Wait for a reconnect signal or context cancellation
			if len(reconnectChans) == 0 {
				// If no tunnels need reconnection, sleep briefly
				select {
				case <-tm.ctx.Done():
					return
				case <-time.After(5 * time.Second):
					continue
				}
			}

			// Create a select case for each reconnect channel
			cases := make([]chan struct{}, len(reconnectChans)+1)
			cases[0] = make(chan struct{})

			// Add context done channel
			go func() {
				<-tm.ctx.Done()
				close(cases[0])
			}()

			// Add reconnect channels
			for i, ch := range reconnectChans {
				cases[i+1] = ch
			}

			// Wait for any signal
			var triggered bool
			for i, ch := range cases {
				select {
				case <-ch:
					triggered = true
					if i == 0 {
						return // context cancelled
					}
					// Otherwise, a tunnel needs reconnection
					break
				default:
				}
				if triggered {
					break
				}
			}

			// If no signal, sleep briefly
			if !triggered {
				time.Sleep(1 * time.Second)
			}
		}
	}
}

// autosaveRoutine periodically saves tunnel configurations
func (tm *TunnelManager) autosaveRoutine() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := tm.SaveTunnels(); err != nil {
				tm.log.Errorf("Failed to save tunnels: %v", err)
			}
		case <-tm.ctx.Done():
			// One final save on shutdown
			if err := tm.SaveTunnels(); err != nil {
				tm.log.Errorf("Failed to save tunnels on shutdown: %v", err)
			}
			return
		}
	}
}

// cleanupStaleTunnels removes or reconnects inactive tunnels
func (tm *TunnelManager) cleanupStaleTunnels() {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	now := time.Now()
	for id, tunnel := range tm.tunnels {
		// Check if tunnel is expired based on max connection time
		if !tunnel.ExpiresAt.IsZero() && now.After(tunnel.ExpiresAt) {
			tm.log.Infof("Removing expired tunnel %s", id)

			// Close the tunnel if it's active
			if tunnel.Active {
				close(tunnel.stopChan)
				if tunnel.tunnel != nil {
					_ = tunnel.tunnel.Stop()
				}
			}

			// Remove from userTunnels map
			if tunnel.UserID != "" {
				if tunnels, exists := tm.userTunnels[tunnel.UserID]; exists {
					delete(tunnels, id)

					// Remove user entry if they have no more tunnels
					if len(tunnels) == 0 {
						delete(tm.userTunnels, tunnel.UserID)
					}
				}

				// If Redis client is available, decrement the user's tunnel count
				if tm.redisClient != nil {
					if redisClient, ok := tm.redisClient.(interface {
						DecrementTunnelCount(userID string) (int, error)
					}); ok {
						_, err := redisClient.DecrementTunnelCount(tunnel.UserID)
						if err != nil {
							tm.log.Warnf("Failed to decrement tunnel count in Redis: %v", err)
						}
					}
				}
			}

			// Remove from tunnels map
			delete(tm.tunnels, id)
		}
	}
}

// CreateTunnel creates a new tunnel and adds it to the manager
func (tm *TunnelManager) CreateTunnel(protocol string, localPort int, serverHost string, serverPort int, certData []byte, userID string) (*TunnelInfo, error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	// Check if we have reached the maximum number of tunnels
	if tm.maxTunnels > 0 && len(tm.tunnels) >= tm.maxTunnels {
		return nil, fmt.Errorf("maximum number of tunnels reached (%d)", tm.maxTunnels)
	}

	// Check if user has reached their tunnel limit
	if tm.maxTunnelsPerUser > 0 && userID != "" {
		userTunnelCount := 0

		// If Redis client is available, use it to get the user's tunnel count
		if tm.redisClient != nil {
			if redisClient, ok := tm.redisClient.(interface {
				GetTunnelCount(userID string) (int, error)
			}); ok {
				count, err := redisClient.GetTunnelCount(userID)
				if err != nil {
					tm.log.Warnf("Failed to get tunnel count from Redis: %v", err)
				} else {
					userTunnelCount = count
				}
			}
		} else {
			// Otherwise use in-memory tracking
			if tunnels, exists := tm.userTunnels[userID]; exists {
				userTunnelCount = len(tunnels)
			}
		}

		if userTunnelCount >= tm.maxTunnelsPerUser {
			return nil, fmt.Errorf("maximum number of tunnels per user reached (%d)", tm.maxTunnelsPerUser)
		}
	}

	// Check if tunnel already exists for this port
	for _, t := range tm.tunnels {
		if t.Protocol == protocol && t.LocalPort == localPort {
			return nil, fmt.Errorf("tunnel already exists for %s port %d", protocol, localPort)
		}
	}

	// Set up expiration time if max connection time is set
	var expiresAt time.Time
	if tm.maxConnectionTime > 0 {
		expiresAt = time.Now().Add(tm.maxConnectionTime)
	}

	// Create a new tunnel entry
	id := uuid.New().String()
	mt := &ManagedTunnel{
		ID:            id,
		UserID:        userID,
		Protocol:      protocol,
		LocalPort:     localPort,
		ServerHost:    serverHost,
		ServerPort:    serverPort,
		Created:       time.Now(),
		LastActive:    time.Now(),
		Active:        false,
		ExpiresAt:     expiresAt,
		CertData:      certData,
		reconnectChan: make(chan struct{}, 1),
		stopChan:      make(chan struct{}),
	}

	// Set the tunnel public URL
	switch protocol {
	case "http", "https":
		host := id
		if protocol == "https" {
			host += "-s"
		}
		mt.PublicURL = fmt.Sprintf("%s://%s.%s", protocol, host, serverHost)
	case "tcp":
		// For TCP, the public URL is the server host and port
		mt.PublicURL = fmt.Sprintf("tcp://%s:%d", serverHost, serverPort)
	default:
		return nil, fmt.Errorf("unsupported protocol: %s", protocol)
	}

	// Add the tunnel to our maps
	tm.tunnels[id] = mt

	// Track tunnel by user ID if provided
	if userID != "" {
		if _, exists := tm.userTunnels[userID]; !exists {
			tm.userTunnels[userID] = make(map[string]struct{})
		}
		tm.userTunnels[userID][id] = struct{}{}

		// If Redis client is available, increment the user's tunnel count
		if tm.redisClient != nil {
			if redisClient, ok := tm.redisClient.(interface {
				IncrementTunnelCount(userID string) (int, error)
				SetTunnelExpiry(tunnelID string, duration time.Duration) error
			}); ok {
				_, err := redisClient.IncrementTunnelCount(userID)
				if err != nil {
					tm.log.Warnf("Failed to increment tunnel count in Redis: %v", err)
				}

				// Set tunnel expiry in Redis if max connection time is set
				if !expiresAt.IsZero() {
					duration := time.Until(expiresAt)
					err := redisClient.SetTunnelExpiry(id, duration)
					if err != nil {
						tm.log.Warnf("Failed to set tunnel expiry in Redis: %v", err)
					}
				}
			}
		}
	}

	// Start the tunnel
	go tm.startTunnel(mt)

	return tm.getTunnelInfo(mt), nil
}

// startTunnel starts a managed tunnel
func (tm *TunnelManager) startTunnel(mt *ManagedTunnel) {
	tm.log.Infof("Starting tunnel %s for %s port %d to %s",
		mt.ID, mt.Protocol, mt.LocalPort, mt.PublicURL)

	// Create tunnel instance with server-provided ID
	tunnel := &Tunnel{
		ID:         mt.ID,
		Protocol:   mt.Protocol,
		LocalPort:  mt.LocalPort,
		PublicURL:  mt.PublicURL,
		CertData:   mt.CertData,
		ServerHost: mt.ServerHost,
		ServerPort: mt.ServerPort,
		stopCh:     make(chan struct{}),
		log:        tm.log,
	}

	// Store tunnel in managed tunnel
	mt.tunnel = tunnel

	// Start tunnel with the server-provided ID
	if err := tunnel.Start(); err != nil {
		tm.log.Errorf("Failed to start tunnel %s: %v", mt.ID, err)

		// Mark as inactive
		tm.mu.Lock()
		mt.Active = false
		tm.mu.Unlock()

		// Trigger reconnect after delay
		go func() {
			select {
			case <-time.After(10 * time.Second):
				select {
				case mt.reconnectChan <- struct{}{}:
					// Signal sent successfully
				default:
					// Channel already has a pending signal
				}
			case <-mt.stopChan:
				// Tunnel was removed, no need to reconnect
			}
		}()

		return
	}

	// Mark as active
	tm.mu.Lock()
	mt.Active = true
	tm.mu.Unlock()

	// Monitor tunnel until it stops or is removed
	select {
	case <-tunnel.stopCh:
		// Tunnel stopped itself
		tm.log.Infof("Tunnel %s stopped", mt.ID)
	case <-mt.stopChan:
		// Tunnel was removed from the manager
		tm.log.Infof("Tunnel %s removed", mt.ID)
		tunnel.Stop()
	}

	// Mark as inactive
	tm.mu.Lock()
	mt.Active = false
	mt.LastActive = time.Now()
	tm.mu.Unlock()
}

// GetTunnel retrieves a tunnel by ID
func (tm *TunnelManager) GetTunnel(id string) (*TunnelInfo, bool) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	tunnel, exists := tm.tunnels[id]
	if !exists {
		return nil, false
	}

	return tm.getTunnelInfo(tunnel), true
}

// GetTunnelByPort finds a tunnel by protocol and local port
func (tm *TunnelManager) GetTunnelByPort(protocol string, port int) (*TunnelInfo, bool) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	for _, tunnel := range tm.tunnels {
		if tunnel.Protocol == protocol && tunnel.LocalPort == port {
			return tm.getTunnelInfo(tunnel), true
		}
	}

	return nil, false
}

// RemoveTunnel stops and removes a tunnel from the manager
func (tm *TunnelManager) RemoveTunnel(id string) bool {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	tunnel, exists := tm.tunnels[id]
	if !exists {
		return false
	}

	// Close the tunnel if it's active
	if tunnel.Active {
		close(tunnel.stopChan)
		if tunnel.tunnel != nil {
			_ = tunnel.tunnel.Stop()
		}
	}

	// Remove from userTunnels map
	if tunnel.UserID != "" {
		if tunnels, exists := tm.userTunnels[tunnel.UserID]; exists {
			delete(tunnels, id)

			// Remove user entry if they have no more tunnels
			if len(tunnels) == 0 {
				delete(tm.userTunnels, tunnel.UserID)
			}
		}

		// If Redis client is available, decrement the user's tunnel count
		if tm.redisClient != nil {
			if redisClient, ok := tm.redisClient.(interface {
				DecrementTunnelCount(userID string) (int, error)
			}); ok {
				_, err := redisClient.DecrementTunnelCount(tunnel.UserID)
				if err != nil {
					tm.log.Warnf("Failed to decrement tunnel count in Redis: %v", err)
				}
			}
		}
	}

	// Remove from tunnels map
	delete(tm.tunnels, id)

	return true
}

// RestartTunnel restarts a tunnel by ID
func (tm *TunnelManager) RestartTunnel(id string) error {
	tm.mu.RLock()
	tunnel, exists := tm.tunnels[id]
	tm.mu.RUnlock()

	if !exists {
		return fmt.Errorf("tunnel not found: %s", id)
	}

	// Signal the tunnel to stop
	if tunnel.tunnel != nil {
		tunnel.tunnel.Stop()
	}

	// Mark as inactive
	tm.mu.Lock()
	tunnel.Active = false
	tm.mu.Unlock()

	// Trigger reconnect
	select {
	case tunnel.reconnectChan <- struct{}{}:
		// Signal sent successfully
	default:
		// Channel already has a pending signal
	}

	return nil
}

// ListTunnels returns information about all tunnels
func (tm *TunnelManager) ListTunnels() []*TunnelInfo {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	tunnels := make([]*TunnelInfo, 0, len(tm.tunnels))
	for _, tunnel := range tm.tunnels {
		tunnels = append(tunnels, tm.getTunnelInfo(tunnel))
	}

	return tunnels
}

// getTunnelInfo returns public information about a managed tunnel
func (tm *TunnelManager) getTunnelInfo(tunnel *ManagedTunnel) *TunnelInfo {
	return &TunnelInfo{
		ID:         tunnel.ID,
		UserID:     tunnel.UserID,
		Protocol:   tunnel.Protocol,
		LocalPort:  tunnel.LocalPort,
		PublicURL:  tunnel.PublicURL,
		ServerHost: tunnel.ServerHost,
		ServerPort: tunnel.ServerPort,
		Created:    tunnel.Created,
		LastActive: tunnel.LastActive,
		Active:     tunnel.Active,
		ExpiresAt:  tunnel.ExpiresAt,
	}
}

// SaveTunnels saves all tunnel configurations to disk
func (tm *TunnelManager) SaveTunnels() error {
	if tm.configDir == "" {
		return fmt.Errorf("no config directory specified")
	}

	// Create config directory if it doesn't exist
	if err := os.MkdirAll(tm.configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Get tunnels to save
	tm.mu.RLock()
	tunnelData := TunnelSaveData{
		Tunnels: make([]ManagedTunnelData, 0, len(tm.tunnels)),
	}

	for _, tunnel := range tm.tunnels {
		// Save certificate to disk if needed
		certPath := ""
		if len(tunnel.CertData) > 0 {
			certPath = filepath.Join(tm.configDir, fmt.Sprintf("cert_%s.pem", tunnel.ID))
			if err := ioutil.WriteFile(certPath, tunnel.CertData, 0600); err != nil {
				tm.log.Warnf("Failed to save certificate for tunnel %s: %v", tunnel.ID, err)
				certPath = ""
			}
		}

		// Add tunnel data
		tunnelData.Tunnels = append(tunnelData.Tunnels, ManagedTunnelData{
			ID:         tunnel.ID,
			UserID:     tunnel.UserID,
			Protocol:   tunnel.Protocol,
			LocalPort:  tunnel.LocalPort,
			PublicURL:  tunnel.PublicURL,
			ServerHost: tunnel.ServerHost,
			ServerPort: tunnel.ServerPort,
			Created:    tunnel.Created,
			LastActive: tunnel.LastActive,
			ExpiresAt:  tunnel.ExpiresAt,
			CertPath:   certPath,
		})
	}
	tm.mu.RUnlock()

	// Marshal to JSON
	configPath := filepath.Join(tm.configDir, "tunnels.json")
	data, err := json.MarshalIndent(tunnelData, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal tunnel data: %w", err)
	}

	// Write to file
	if err := ioutil.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write tunnel data: %w", err)
	}

	tm.log.Infof("Saved %d tunnels to %s", len(tunnelData.Tunnels), configPath)
	return nil
}

// LoadTunnels loads tunnel configurations from disk
func (tm *TunnelManager) LoadTunnels() error {
	if tm.configDir == "" {
		return fmt.Errorf("no config directory specified")
	}

	configPath := filepath.Join(tm.configDir, "tunnels.json")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// No saved tunnels, not an error
		return nil
	}

	// Read configuration file
	data, err := ioutil.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read tunnel data: %w", err)
	}

	// Unmarshal JSON
	var tunnelData TunnelSaveData
	if err := json.Unmarshal(data, &tunnelData); err != nil {
		return fmt.Errorf("failed to unmarshal tunnel data: %w", err)
	}

	// Process each tunnel
	for _, td := range tunnelData.Tunnels {
		// Load certificate if available
		var certData []byte
		if td.CertPath != "" {
			var err error
			certData, err = ioutil.ReadFile(td.CertPath)
			if err != nil {
				tm.log.Warnf("Failed to load certificate for tunnel %s: %v", td.ID, err)
			}
		}

		// Create managed tunnel
		managedTunnel := &ManagedTunnel{
			ID:            td.ID,
			UserID:        td.UserID,
			Protocol:      td.Protocol,
			LocalPort:     td.LocalPort,
			PublicURL:     td.PublicURL,
			ServerHost:    td.ServerHost,
			ServerPort:    td.ServerPort,
			Created:       td.Created,
			LastActive:    td.LastActive,
			Active:        false,
			ExpiresAt:     td.ExpiresAt,
			CertData:      certData,
			reconnectChan: make(chan struct{}, 1),
			stopChan:      make(chan struct{}),
		}

		// Add to tunnels map
		tm.mu.Lock()
		tm.tunnels[td.ID] = managedTunnel
		tm.mu.Unlock()

		// Start the tunnel asynchronously
		go tm.startTunnel(managedTunnel)
	}

	tm.log.Infof("Loaded %d tunnels from %s", len(tunnelData.Tunnels), configPath)
	return nil
}

// StartAllTunnels starts all tunnels that aren't currently active
func (tm *TunnelManager) StartAllTunnels() int {
	tm.mu.RLock()
	var inactiveTunnels []*ManagedTunnel
	for _, tunnel := range tm.tunnels {
		if !tunnel.Active {
			inactiveTunnels = append(inactiveTunnels, tunnel)
		}
	}
	tm.mu.RUnlock()

	// Start each inactive tunnel
	for _, tunnel := range inactiveTunnels {
		select {
		case tunnel.reconnectChan <- struct{}{}:
			// Signal sent successfully
		default:
			// Channel already has a pending signal
		}
	}

	return len(inactiveTunnels)
}

// StopAllTunnels stops all active tunnels
func (tm *TunnelManager) StopAllTunnels() int {
	tm.mu.RLock()
	var activeTunnels []*ManagedTunnel
	for _, tunnel := range tm.tunnels {
		if tunnel.Active {
			activeTunnels = append(activeTunnels, tunnel)
		}
	}
	tm.mu.RUnlock()

	// Stop each active tunnel
	for _, tunnel := range activeTunnels {
		if tunnel.tunnel != nil {
			tunnel.tunnel.Stop()
		}
	}

	return len(activeTunnels)
}

// Close shuts down all tunnels and the manager
func (tm *TunnelManager) Close() error {
	// Signal shutdown
	tm.cancel()

	// Create a copy of the tunnel map to avoid holding the lock during shutdown
	tunnelsCopy := make(map[string]*ManagedTunnel)

	tm.mu.RLock()
	for id, tunnel := range tm.tunnels {
		tunnelsCopy[id] = tunnel
	}
	tm.mu.RUnlock()

	// Stop all tunnels
	for id, tunnel := range tunnelsCopy {
		tm.log.Infof("Stopping tunnel %s", id)
		close(tunnel.stopChan)
		if tunnel.tunnel != nil {
			tunnel.tunnel.Stop()
		}
	}

	// Wait for background tasks to finish
	tm.stopWait.Wait()

	// Clear tunnels map
	tm.mu.Lock()
	tm.tunnels = make(map[string]*ManagedTunnel)
	tm.mu.Unlock()

	tm.log.Info("Tunnel manager shut down")
	return nil
}

// SetRedisClient sets the Redis client for the tunnel manager
func (tm *TunnelManager) SetRedisClient(client interface{}) {
	tm.redisClient = client
}

// GetTunnelsByUserID returns tunnels for a specific user
func (tm *TunnelManager) GetTunnelsByUserID(userID string) []*TunnelInfo {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	result := make([]*TunnelInfo, 0)

	if tunnels, exists := tm.userTunnels[userID]; exists {
		for id := range tunnels {
			if tunnel, exists := tm.tunnels[id]; exists {
				info := tm.getTunnelInfo(tunnel)
				result = append(result, info)
			}
		}
	}

	return result
}
