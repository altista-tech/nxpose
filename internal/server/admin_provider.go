package server

import (
	"fmt"
	"sync/atomic"
	"time"

	"nxpose/internal/admin"
)

// GetTunnels returns all active tunnels for the admin panel
func (s *Server) GetTunnels() []admin.TunnelInfo {
	s.tunnels.mu.RLock()
	defer s.tunnels.mu.RUnlock()

	tunnels := make([]admin.TunnelInfo, 0, len(s.tunnels.tunnels))
	for _, t := range s.tunnels.tunnels {
		_, connected := s.wsManager.GetWebSocketTunnel(t.ID)
		tunnels = append(tunnels, toAdminTunnelInfo(t, connected))
	}
	return tunnels
}

// GetClients returns a list of unique clients and their tunnels
func (s *Server) GetClients() []admin.ClientInfo {
	s.tunnels.mu.RLock()
	defer s.tunnels.mu.RUnlock()

	clientMap := make(map[string]*admin.ClientInfo)
	for _, t := range s.tunnels.tunnels {
		_, connected := s.wsManager.GetWebSocketTunnel(t.ID)
		ti := toAdminTunnelInfo(t, connected)

		lastActive := t.GetLastActive()
		if client, ok := clientMap[t.ClientID]; ok {
			client.TunnelCount++
			client.Tunnels = append(client.Tunnels, ti)
			if lastActive.After(client.LastActive) {
				client.LastActive = lastActive
			}
		} else {
			clientMap[t.ClientID] = &admin.ClientInfo{
				ID:          t.ClientID,
				TunnelCount: 1,
				Tunnels:     []admin.TunnelInfo{ti},
				LastActive:  lastActive,
			}
		}
	}

	clients := make([]admin.ClientInfo, 0, len(clientMap))
	for _, c := range clientMap {
		clients = append(clients, *c)
	}
	return clients
}

// GetStats returns server statistics for the admin dashboard
func (s *Server) GetStats() admin.ServerStats {
	s.tunnels.mu.RLock()
	tunnelCount := len(s.tunnels.tunnels)

	var totalConns int64
	connectedClientIDs := make(map[string]bool)
	for _, t := range s.tunnels.tunnels {
		totalConns += atomic.LoadInt64(&t.connections)
		// Only count clients that have at least one active WebSocket tunnel
		if _, connected := s.wsManager.GetWebSocketTunnel(t.ID); connected {
			connectedClientIDs[t.ClientID] = true
		}
	}
	s.tunnels.mu.RUnlock()

	uptime := time.Since(s.startTime)

	return admin.ServerStats{
		ActiveTunnels:    tunnelCount,
		ConnectedClients: len(connectedClientIDs),
		TotalConnections: totalConns,
		Uptime:           uptime,
		UptimeStr:        admin.FormatDuration(uptime),
		MaintenanceMode:  s.GetMaintenanceMode(),
	}
}

// KillTunnel removes a tunnel by ID
func (s *Server) KillTunnel(tunnelID string) error {
	// Extract tunnel data under lock, then release before external calls
	s.tunnels.mu.Lock()
	tunnel, ok := s.tunnels.tunnels[tunnelID]
	if !ok {
		s.tunnels.mu.Unlock()
		return fmt.Errorf("tunnel %s not found", tunnelID)
	}
	clientID := tunnel.ClientID
	delete(s.tunnels.tunnels, tunnelID)
	s.tunnels.mu.Unlock()

	// Perform external operations without holding the tunnel lock
	if s.redis != nil && clientID != "" {
		if _, err := s.redis.DecrementTunnelCount(clientID); err != nil {
			s.log.Warnf("Failed to decrement tunnel count in Redis for killed tunnel: %v", err)
		}
	}

	s.wsManager.UnregisterWebSocketTunnel(tunnelID)
	return nil
}

// GetMaintenanceMode returns the current maintenance mode state
func (s *Server) GetMaintenanceMode() bool {
	s.maintenanceMu.RLock()
	defer s.maintenanceMu.RUnlock()
	return s.maintenanceMode
}

// SetMaintenanceMode sets the maintenance mode state
func (s *Server) SetMaintenanceMode(enabled bool) {
	s.maintenanceMu.Lock()
	defer s.maintenanceMu.Unlock()
	s.maintenanceMode = enabled
}

// ToggleMaintenanceMode atomically toggles maintenance mode and returns the new state
func (s *Server) ToggleMaintenanceMode() bool {
	s.maintenanceMu.Lock()
	defer s.maintenanceMu.Unlock()
	s.maintenanceMode = !s.maintenanceMode
	return s.maintenanceMode
}

// toAdminTunnelInfo converts a server Tunnel to an admin TunnelInfo
func toAdminTunnelInfo(t *Tunnel, connected bool) admin.TunnelInfo {
	return admin.TunnelInfo{
		ID:          t.ID,
		ClientID:    t.ClientID,
		Protocol:    t.Protocol,
		Subdomain:   t.Subdomain,
		TargetPort:  t.TargetPort,
		CreateTime:  t.CreateTime,
		LastActive:  t.GetLastActive(),
		ExpiresAt:   t.ExpiresAt,
		Connections: atomic.LoadInt64(&t.connections),
		Connected:   connected,
	}
}
