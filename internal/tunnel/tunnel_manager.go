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
	tunnels    map[string]*ManagedTunnel
	mu         sync.RWMutex
	log        *logrus.Logger
	configDir  string
	maxTunnels int
	ctx        context.Context
	cancel     context.CancelFunc
	stopWait   sync.WaitGroup
}

// ManagedTunnel represents a tunnel being managed
type ManagedTunnel struct {
	ID            string
	Protocol      string
	LocalPort     int
	PublicURL     string
	ServerHost    string
	ServerPort    int
	Created       time.Time
	LastActive    time.Time
	Active        bool
	CertData      []byte
	tunnel        *Tunnel
	reconnectChan chan struct{}
	stopChan      chan struct{}
}

// TunnelInfo provides public information about a managed tunnel
type TunnelInfo struct {
	ID         string    `json:"id"`
	Protocol   string    `json:"protocol"`
	LocalPort  int       `json:"local_port"`
	PublicURL  string    `json:"public_url"`
	ServerHost string    `json:"server_host"`
	ServerPort int       `json:"server_port"`
	Created    time.Time `json:"created"`
	LastActive time.Time `json:"last_active"`
	Active     bool      `json:"active"`
}

// TunnelSaveData represents the data saved to disk for persistence
type TunnelSaveData struct {
	Tunnels []ManagedTunnelData `json:"tunnels"`
}

// ManagedTunnelData represents the serializable data for a managed tunnel
type ManagedTunnelData struct {
	ID         string    `json:"id"`
	Protocol   string    `json:"protocol"`
	LocalPort  int       `json:"local_port"`
	PublicURL  string    `json:"public_url"`
	ServerHost string    `json:"server_host"`
	ServerPort int       `json:"server_port"`
	Created    time.Time `json:"created"`
	LastActive time.Time `json:"last_active"`
	CertPath   string    `json:"cert_path,omitempty"`
}

// NewTunnelManager creates a new tunnel manager
func NewTunnelManager(configDir string, maxTunnels int) *TunnelManager {
	ctx, cancel := context.WithCancel(context.Background())

	manager := &TunnelManager{
		tunnels:    make(map[string]*ManagedTunnel),
		log:        logrus.New(),
		configDir:  configDir,
		maxTunnels: maxTunnels,
		ctx:        ctx,
		cancel:     cancel,
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
	now := time.Now()
	inactiveThreshold := 30 * time.Minute

	tm.mu.Lock()
	defer tm.mu.Unlock()

	for id, tunnel := range tm.tunnels {
		// Skip active tunnels
		if tunnel.Active {
			continue
		}

		// If the tunnel has been inactive for too long, remove it
		if now.Sub(tunnel.LastActive) > inactiveThreshold {
			tm.log.Infof("Removing stale tunnel %s (inactive for %v)", id, now.Sub(tunnel.LastActive))
			delete(tm.tunnels, id)
			close(tunnel.stopChan)
		}
	}
}

// CreateTunnel creates a new tunnel and adds it to the manager
func (tm *TunnelManager) CreateTunnel(protocol string, localPort int, serverHost string, serverPort int, certData []byte) (*TunnelInfo, error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	// Check if we have reached the maximum number of tunnels
	if tm.maxTunnels > 0 && len(tm.tunnels) >= tm.maxTunnels {
		return nil, fmt.Errorf("maximum number of tunnels reached (%d)", tm.maxTunnels)
	}

	// Check if a tunnel for this port already exists
	for _, t := range tm.tunnels {
		if t.LocalPort == localPort && t.Protocol == protocol {
			return nil, fmt.Errorf("tunnel for %s port %d already exists", protocol, localPort)
		}
	}

	// Generate tunnel ID
	id := uuid.New().String()

	// Create the tunnel
	managedTunnel := &ManagedTunnel{
		ID:            id,
		Protocol:      protocol,
		LocalPort:     localPort,
		ServerHost:    serverHost,
		ServerPort:    serverPort,
		Created:       time.Now(),
		LastActive:    time.Now(),
		Active:        false,
		CertData:      certData,
		reconnectChan: make(chan struct{}, 1),
		stopChan:      make(chan struct{}),
	}

	// Try to expose the service with retries
	maxRetries := 3
	var publicURL, tunnelID string
	var err error

	for i := 0; i < maxRetries; i++ {
		tm.log.Infof("Attempting to expose local service (attempt %d/%d)...", i+1, maxRetries)

		publicURL, tunnelID, err = ExposeLocalService(protocol, localPort, certData, serverHost, serverPort)
		if err == nil {
			break // Success, exit retry loop
		}

		tm.log.Warnf("Failed to expose service (attempt %d/%d): %v", i+1, maxRetries, err)

		if i < maxRetries-1 {
			// Wait before retrying with exponential backoff
			backoff := time.Duration((i+1)*(i+1)) * time.Second
			tm.log.Infof("Retrying in %v...", backoff)
			time.Sleep(backoff)
		}
	}

	// If all retries failed
	if err != nil {
		return nil, fmt.Errorf("failed to expose local service after %d attempts: %w", maxRetries, err)
	}

	// Validate the response
	if tunnelID == "" || publicURL == "" {
		return nil, fmt.Errorf("server returned invalid tunnel data: missing tunnel ID or public URL")
	}

	// Update with server-provided ID and URL
	tm.log.Infof("Successfully created tunnel with ID %s and URL %s", tunnelID, publicURL)
	managedTunnel.ID = tunnelID // Use the server-provided tunnelID
	managedTunnel.PublicURL = publicURL
	tm.tunnels[tunnelID] = managedTunnel // Store with tunnelID from server

	// Start the tunnel asynchronously
	go tm.startTunnel(managedTunnel)

	// Save tunnels configuration
	go tm.SaveTunnels()

	// Return tunnel info
	return tm.getTunnelInfo(managedTunnel), nil
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
	tunnel, exists := tm.tunnels[id]
	if !exists {
		tm.mu.Unlock()
		return false
	}

	// Signal the tunnel to stop
	close(tunnel.stopChan)

	// Clean up resources
	if tunnel.tunnel != nil {
		tunnel.tunnel.Stop()
	}

	// Remove from manager
	delete(tm.tunnels, id)
	tm.mu.Unlock()

	tm.log.Infof("Removed tunnel %s", id)

	// Save updated configuration
	go tm.SaveTunnels()

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
		Protocol:   tunnel.Protocol,
		LocalPort:  tunnel.LocalPort,
		PublicURL:  tunnel.PublicURL,
		ServerHost: tunnel.ServerHost,
		ServerPort: tunnel.ServerPort,
		Created:    tunnel.Created,
		LastActive: tunnel.LastActive,
		Active:     tunnel.Active,
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
			Protocol:   tunnel.Protocol,
			LocalPort:  tunnel.LocalPort,
			PublicURL:  tunnel.PublicURL,
			ServerHost: tunnel.ServerHost,
			ServerPort: tunnel.ServerPort,
			Created:    tunnel.Created,
			LastActive: tunnel.LastActive,
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
			Protocol:      td.Protocol,
			LocalPort:     td.LocalPort,
			PublicURL:     td.PublicURL,
			ServerHost:    td.ServerHost,
			ServerPort:    td.ServerPort,
			Created:       td.Created,
			LastActive:    td.LastActive,
			Active:        false,
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
