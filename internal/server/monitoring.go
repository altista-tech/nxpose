package server

import (
	"time"

	"github.com/sirupsen/logrus"
)

// startMonitoring starts a background routine to monitor tunnels and connections
func (s *Server) startMonitoring() {
	// Run tunnel cleanup every minute
	go s.tunnelCleanupRoutine()

	// Log statistics every 5 minutes
	go s.statsRoutine()

	s.log.Info("Monitoring routines started")
}

// tunnelCleanupRoutine periodically checks for and removes inactive tunnels
func (s *Server) tunnelCleanupRoutine() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.cleanupInactiveTunnels()
		case <-s.shutdownCh:
			s.log.Info("Tunnel cleanup routine shutting down")
			return
		}
	}
}

// cleanupInactiveTunnels removes tunnels that have been inactive for too long
func (s *Server) cleanupInactiveTunnels() {
	now := time.Now()
	inactiveThreshold := 1 * time.Hour
	expiredThreshold := 24 * time.Hour

	// Get tunnels to clean up
	var tunnelsToRemove []string

	s.tunnels.mu.RLock()
	for id, tunnel := range s.tunnels.tunnels {
		// Check if the tunnel has a websocket connection
		wsTunnel, connected := s.wsManager.GetWebSocketTunnel(id)

		// If there's no active WebSocket connection and it's been inactive for too long, remove it
		if !connected {
			if now.Sub(tunnel.GetLastActive()) > inactiveThreshold {
				tunnelsToRemove = append(tunnelsToRemove, id)
				s.log.WithFields(logrus.Fields{
					"tunnel_id":   id,
					"subdomain":   tunnel.Subdomain,
					"last_active": tunnel.GetLastActive(),
				}).Info("Removing inactive tunnel (no WebSocket connection)")
				continue
			}
		} else if wsTunnel != nil {
			// Update the tunnel's last activity time based on the WebSocket connection
			tunnel.SetLastActive(wsTunnel.GetLastActive())
		}

		// Check if the tunnel has expired (based on creation time)
		if now.Sub(tunnel.CreateTime) > expiredThreshold {
			tunnelsToRemove = append(tunnelsToRemove, id)
			s.log.WithFields(logrus.Fields{
				"tunnel_id":  id,
				"subdomain":  tunnel.Subdomain,
				"created_at": tunnel.CreateTime,
			}).Info("Removing expired tunnel")
			continue
		}
	}
	s.tunnels.mu.RUnlock()

	// Remove the tunnels
	if len(tunnelsToRemove) > 0 {
		// Collect client IDs under the lock, delete from map, then release
		// before Redis I/O and WebSocket close to avoid holding the lock
		// during network operations.
		type removedTunnel struct {
			id       string
			clientID string
		}
		var removed []removedTunnel

		s.tunnels.mu.Lock()
		for _, id := range tunnelsToRemove {
			tunnel, exists := s.tunnels.tunnels[id]
			if !exists {
				continue // Already removed by another goroutine
			}
			clientID := ""
			if tunnel.ClientID != "" {
				clientID = tunnel.ClientID
			}
			removed = append(removed, removedTunnel{id: id, clientID: clientID})
			delete(s.tunnels.tunnels, id)
		}
		s.tunnels.mu.Unlock()

		// Perform I/O operations outside the lock
		for _, rt := range removed {
			if s.redis != nil && rt.clientID != "" {
				if _, err := s.redis.DecrementTunnelCount(rt.clientID); err != nil {
					s.log.Warnf("Failed to decrement tunnel count in Redis: %v", err)
				}
			}
			s.wsManager.UnregisterWebSocketTunnel(rt.id)
		}

		s.log.WithField("count", len(removed)).Info("Removed inactive tunnels")
	}
}

// statsRoutine periodically logs server statistics
func (s *Server) statsRoutine() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.logServerStats()
		case <-s.shutdownCh:
			s.log.Info("Statistics routine shutting down")
			return
		}
	}
}

// logServerStats logs information about active tunnels and connections
func (s *Server) logServerStats() {
	s.tunnels.mu.RLock()
	tunnelCount := len(s.tunnels.tunnels)
	s.tunnels.mu.RUnlock()

	s.wsManager.mu.RLock()
	wsConnCount := len(s.wsManager.tunnels)
	s.wsManager.mu.RUnlock()

	s.log.WithFields(logrus.Fields{
		"active_tunnels":           tunnelCount,
		"active_websocket_tunnels": wsConnCount,
	}).Info("Server statistics")
}
