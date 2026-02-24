package admin

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/mux"

	"nxpose/internal/config"
)

// TunnelInfo represents tunnel data exposed to the admin panel
type TunnelInfo struct {
	ID          string    `json:"id"`
	ClientID    string    `json:"client_id"`
	Protocol    string    `json:"protocol"`
	Subdomain   string    `json:"subdomain"`
	TargetPort  int       `json:"target_port"`
	CreateTime  time.Time `json:"create_time"`
	LastActive  time.Time `json:"last_active"`
	ExpiresAt   time.Time `json:"expires_at"`
	Connections int64     `json:"connections"`
	Connected   bool      `json:"connected"`
}

// ClientInfo represents client data exposed to the admin panel
type ClientInfo struct {
	ID          string       `json:"id"`
	TunnelCount int          `json:"tunnel_count"`
	Tunnels     []TunnelInfo `json:"tunnels"`
	LastActive  time.Time    `json:"last_active"`
}

// ServerStats represents server statistics for the dashboard
type ServerStats struct {
	ActiveTunnels    int           `json:"active_tunnels"`
	ConnectedClients int          `json:"connected_clients"`
	TotalConnections int64         `json:"total_connections"`
	Uptime           time.Duration `json:"uptime"`
	UptimeStr        string        `json:"uptime_str"`
	MaintenanceMode  bool          `json:"maintenance_mode"`
}

// DataProvider is an interface that the server implements to provide data to the admin panel
type DataProvider interface {
	GetTunnels() []TunnelInfo
	GetClients() []ClientInfo
	GetStats() ServerStats
	KillTunnel(tunnelID string) error
	GetMaintenanceMode() bool
	SetMaintenanceMode(enabled bool)
}

// Handler implements the admin panel HTTP handlers
type Handler struct {
	config       *config.AdminConfig
	serverConfig *config.ServerConfig
	provider     DataProvider
	templates    *template.Template
	startTime    time.Time
	mu           sync.RWMutex
}

// NewHandler creates a new admin panel handler
func NewHandler(adminConfig *config.AdminConfig, serverConfig *config.ServerConfig, provider DataProvider) (*Handler, error) {
	h := &Handler{
		config:       adminConfig,
		serverConfig: serverConfig,
		provider:     provider,
		startTime:    time.Now(),
	}

	if err := h.loadTemplates(); err != nil {
		return nil, fmt.Errorf("failed to load admin templates: %w", err)
	}

	return h, nil
}

// RegisterRoutes registers admin panel routes on the given router
func (h *Handler) RegisterRoutes(router *mux.Router) {
	prefix := h.config.PathPrefix
	if prefix == "" {
		prefix = "/admin"
	}

	sub := router.PathPrefix(prefix).Subrouter()

	// Apply auth middleware
	sub.Use(h.authMiddleware)

	// Page routes
	sub.HandleFunc("", h.handleDashboard).Methods("GET")
	sub.HandleFunc("/", h.handleDashboard).Methods("GET")
	sub.HandleFunc("/tunnels", h.handleTunnels).Methods("GET")
	sub.HandleFunc("/clients", h.handleClients).Methods("GET")
	sub.HandleFunc("/settings", h.handleSettings).Methods("GET")

	// API routes for HTMX
	sub.HandleFunc("/api/stats", h.handleAPIStats).Methods("GET")
	sub.HandleFunc("/api/tunnels", h.handleAPITunnels).Methods("GET")
	sub.HandleFunc("/api/tunnels/{id}/kill", h.handleAPIKillTunnel).Methods("POST")
	sub.HandleFunc("/api/clients", h.handleAPIClients).Methods("GET")
	sub.HandleFunc("/api/settings/maintenance", h.handleAPIToggleMaintenance).Methods("POST")

	// Static assets
	sub.HandleFunc("/static/style.css", h.handleCSS).Methods("GET")
	sub.HandleFunc("/static/app.js", h.handleJS).Methods("GET")
}

// authMiddleware provides authentication for admin routes
func (h *Handler) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch h.config.AuthMethod {
		case "none":
			next.ServeHTTP(w, r)
			return
		case "basic":
			if h.config.Username == "" || h.config.Password == "" {
				next.ServeHTTP(w, r)
				return
			}
			user, pass, ok := r.BasicAuth()
			if !ok ||
				subtle.ConstantTimeCompare([]byte(user), []byte(h.config.Username)) != 1 ||
				subtle.ConstantTimeCompare([]byte(pass), []byte(h.config.Password)) != 1 {
				w.Header().Set("WWW-Authenticate", `Basic realm="nxpose admin"`)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		default:
			// Default to no auth if unrecognized method
			next.ServeHTTP(w, r)
		}
	})
}

// handleDashboard renders the main dashboard page
func (h *Handler) handleDashboard(w http.ResponseWriter, r *http.Request) {
	stats := h.provider.GetStats()
	data := map[string]interface{}{
		"Page":       "dashboard",
		"PathPrefix": h.config.PathPrefix,
		"Stats":      stats,
		"Config":     h.serverConfig,
	}
	h.renderTemplate(w, "layout", data)
}

// handleTunnels renders the tunnel management page
func (h *Handler) handleTunnels(w http.ResponseWriter, r *http.Request) {
	tunnels := h.provider.GetTunnels()
	data := map[string]interface{}{
		"Page":       "tunnels",
		"PathPrefix": h.config.PathPrefix,
		"Tunnels":    tunnels,
		"Config":     h.serverConfig,
	}
	h.renderTemplate(w, "layout", data)
}

// handleClients renders the client list page
func (h *Handler) handleClients(w http.ResponseWriter, r *http.Request) {
	clients := h.provider.GetClients()
	data := map[string]interface{}{
		"Page":       "clients",
		"PathPrefix": h.config.PathPrefix,
		"Clients":    clients,
		"Config":     h.serverConfig,
	}
	h.renderTemplate(w, "layout", data)
}

// handleSettings renders the server settings page
func (h *Handler) handleSettings(w http.ResponseWriter, r *http.Request) {
	stats := h.provider.GetStats()
	data := map[string]interface{}{
		"Page":            "settings",
		"PathPrefix":      h.config.PathPrefix,
		"Config":          h.serverConfig,
		"MaintenanceMode": stats.MaintenanceMode,
	}
	h.renderTemplate(w, "layout", data)
}

// handleAPIStats returns JSON stats for HTMX polling
func (h *Handler) handleAPIStats(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("HX-Request") == "true" {
		stats := h.provider.GetStats()
		data := map[string]interface{}{
			"Stats":      stats,
			"PathPrefix": h.config.PathPrefix,
		}
		h.renderTemplate(w, "stats_fragment", data)
		return
	}
	stats := h.provider.GetStats()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// handleAPITunnels returns tunnel list as HTML fragment for HTMX
func (h *Handler) handleAPITunnels(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("HX-Request") == "true" {
		tunnels := h.provider.GetTunnels()
		data := map[string]interface{}{
			"Tunnels":    tunnels,
			"PathPrefix": h.config.PathPrefix,
		}
		h.renderTemplate(w, "tunnels_fragment", data)
		return
	}
	tunnels := h.provider.GetTunnels()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tunnels)
}

// handleAPIKillTunnel kills a specific tunnel
func (h *Handler) handleAPIKillTunnel(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	tunnelID := vars["id"]
	if tunnelID == "" {
		http.Error(w, "tunnel ID required", http.StatusBadRequest)
		return
	}

	if err := h.provider.KillTunnel(tunnelID); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	if r.Header.Get("HX-Request") == "true" {
		// Return updated tunnel list
		tunnels := h.provider.GetTunnels()
		data := map[string]interface{}{
			"Tunnels":    tunnels,
			"PathPrefix": h.config.PathPrefix,
		}
		h.renderTemplate(w, "tunnels_fragment", data)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "killed"})
}

// handleAPIClients returns client list as HTML fragment for HTMX
func (h *Handler) handleAPIClients(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("HX-Request") == "true" {
		clients := h.provider.GetClients()
		data := map[string]interface{}{
			"Clients":    clients,
			"PathPrefix": h.config.PathPrefix,
		}
		h.renderTemplate(w, "clients_fragment", data)
		return
	}
	clients := h.provider.GetClients()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(clients)
}

// handleAPIToggleMaintenance toggles maintenance mode
func (h *Handler) handleAPIToggleMaintenance(w http.ResponseWriter, r *http.Request) {
	current := h.provider.GetMaintenanceMode()
	h.provider.SetMaintenanceMode(!current)

	if r.Header.Get("HX-Request") == "true" {
		data := map[string]interface{}{
			"MaintenanceMode": !current,
			"PathPrefix":      h.config.PathPrefix,
		}
		h.renderTemplate(w, "maintenance_fragment", data)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"maintenance_mode": !current})
}

// handleCSS serves the admin CSS
func (h *Handler) handleCSS(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/css")
	w.Write([]byte(adminCSS))
}

// handleJS serves the admin JS
func (h *Handler) handleJS(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/javascript")
	w.Write([]byte(adminJS))
}

// renderTemplate renders a named template with the given data
func (h *Handler) renderTemplate(w http.ResponseWriter, name string, data interface{}) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.templates.ExecuteTemplate(w, name, data); err != nil {
		http.Error(w, "template error: "+err.Error(), http.StatusInternalServerError)
	}
}

// FormatDuration formats a duration into a human-readable string
func FormatDuration(d time.Duration) string {
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60

	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	return fmt.Sprintf("%dm", minutes)
}

// FormatTime formats a time.Time for display
func FormatTime(t time.Time) string {
	if t.IsZero() {
		return "Never"
	}
	return t.Format("2006-01-02 15:04:05")
}

// TimeSince returns a human-readable string for time since t
func TimeSince(t time.Time) string {
	if t.IsZero() {
		return "Never"
	}
	d := time.Since(t)
	if d < time.Minute {
		return "just now"
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	}
	return fmt.Sprintf("%dd ago", int(d.Hours()/24))
}
