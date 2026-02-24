package admin

import (
	"html/template"
	"time"
)

func (h *Handler) loadTemplates() error {
	funcMap := template.FuncMap{
		"formatDuration": FormatDuration,
		"formatTime":     FormatTime,
		"timeSince":      TimeSince,
		"timeNow": func() time.Time {
			return time.Now()
		},
	}

	t := template.New("").Funcs(funcMap)

	// Parse all templates
	templates := []struct {
		name    string
		content string
	}{
		{"layout", layoutTemplate},
		{"dashboard_content", dashboardTemplate},
		{"tunnels_content", tunnelsTemplate},
		{"clients_content", clientsTemplate},
		{"settings_content", settingsTemplate},
		{"stats_fragment", statsFragmentTemplate},
		{"tunnels_fragment", tunnelsFragmentTemplate},
		{"clients_fragment", clientsFragmentTemplate},
		{"maintenance_fragment", maintenanceFragmentTemplate},
	}

	for _, tmpl := range templates {
		if _, err := t.New(tmpl.name).Parse(tmpl.content); err != nil {
			return err
		}
	}

	h.templates = t
	return nil
}

const layoutTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>NXpose Admin</title>
    <link rel="stylesheet" href="{{.PathPrefix}}/static/style.css">
    <script src="https://unpkg.com/htmx.org@1.9.10"></script>
</head>
<body>
    <div class="admin-layout">
        <nav class="sidebar">
            <div class="sidebar-header">
                <h2>NXpose</h2>
                <span class="sidebar-badge">Admin</span>
            </div>
            <ul class="nav-list">
                <li><a href="{{.PathPrefix}}/" class="nav-link{{if eq .Page "dashboard"}} active{{end}}">
                    <svg class="nav-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="3" y="3" width="7" height="7"/><rect x="14" y="3" width="7" height="7"/><rect x="3" y="14" width="7" height="7"/><rect x="14" y="14" width="7" height="7"/></svg>
                    Dashboard
                </a></li>
                <li><a href="{{.PathPrefix}}/tunnels" class="nav-link{{if eq .Page "tunnels"}} active{{end}}">
                    <svg class="nav-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M4 14a1 1 0 0 1-.78-1.63l9.9-10.2a.5.5 0 0 1 .86.46l-1.92 6.02A1 1 0 0 0 13 10h7a1 1 0 0 1 .78 1.63l-9.9 10.2a.5.5 0 0 1-.86-.46l1.92-6.02A1 1 0 0 0 11 14z"/></svg>
                    Tunnels
                </a></li>
                <li><a href="{{.PathPrefix}}/clients" class="nav-link{{if eq .Page "clients"}} active{{end}}">
                    <svg class="nav-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2"/><circle cx="9" cy="7" r="4"/><path d="M22 21v-2a4 4 0 0 0-3-3.87"/><path d="M16 3.13a4 4 0 0 1 0 7.75"/></svg>
                    Clients
                </a></li>
                <li><a href="{{.PathPrefix}}/settings" class="nav-link{{if eq .Page "settings"}} active{{end}}">
                    <svg class="nav-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1 0 2.83 2 2 0 0 1-2.83 0l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-2 2 2 2 0 0 1-2-2v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83 0 2 2 0 0 1 0-2.83l.06-.06A1.65 1.65 0 0 0 4.68 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1-2-2 2 2 0 0 1 2-2h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 0-2.83 2 2 0 0 1 2.83 0l.06.06A1.65 1.65 0 0 0 9 4.68a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 2-2 2 2 0 0 1 2 2v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 0 2 2 0 0 1 0 2.83l-.06.06A1.65 1.65 0 0 0 19.4 9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 2 2 2 2 0 0 1-2 2h-.09a1.65 1.65 0 0 0-1.51 1z"/></svg>
                    Settings
                </a></li>
            </ul>
        </nav>
        <main class="main-content">
            {{if eq .Page "dashboard"}}{{template "dashboard_content" .}}{{end}}
            {{if eq .Page "tunnels"}}{{template "tunnels_content" .}}{{end}}
            {{if eq .Page "clients"}}{{template "clients_content" .}}{{end}}
            {{if eq .Page "settings"}}{{template "settings_content" .}}{{end}}
        </main>
    </div>
    <script src="{{.PathPrefix}}/static/app.js"></script>
</body>
</html>`

const dashboardTemplate = `<div class="page-header">
    <h1>Dashboard</h1>
</div>
<div id="stats-container" hx-get="{{.PathPrefix}}/api/stats" hx-trigger="every 5s" hx-swap="innerHTML">
    {{template "stats_fragment" .}}
</div>`

const statsFragmentTemplate = `<div class="stats-grid">
    <div class="stat-card">
        <div class="stat-label">Active Tunnels</div>
        <div class="stat-value">{{.Stats.ActiveTunnels}}</div>
    </div>
    <div class="stat-card">
        <div class="stat-label">Connected Clients</div>
        <div class="stat-value">{{.Stats.ConnectedClients}}</div>
    </div>
    <div class="stat-card">
        <div class="stat-label">Total Connections</div>
        <div class="stat-value">{{.Stats.TotalConnections}}</div>
    </div>
    <div class="stat-card">
        <div class="stat-label">Uptime</div>
        <div class="stat-value">{{.Stats.UptimeStr}}</div>
    </div>
</div>
<div class="card" style="margin-top: 1.5rem;">
    <div class="card-header">
        <h3>Server Status</h3>
        {{if .Stats.MaintenanceMode}}<span class="badge badge-warning">Maintenance Mode</span>{{else}}<span class="badge badge-success">Operational</span>{{end}}
    </div>
    <div class="card-body">
        <p>The server is {{if .Stats.MaintenanceMode}}in maintenance mode. New tunnel connections are paused.{{else}}running normally and accepting connections.{{end}}</p>
    </div>
</div>`

const tunnelsTemplate = `<div class="page-header">
    <h1>Tunnel Management</h1>
</div>
<div id="tunnels-container" hx-get="{{.PathPrefix}}/api/tunnels" hx-trigger="every 5s" hx-swap="innerHTML">
    {{template "tunnels_fragment" .}}
</div>`

const tunnelsFragmentTemplate = `{{if .Tunnels}}
<div class="table-container">
    <table class="data-table">
        <thead>
            <tr>
                <th>ID</th>
                <th>Subdomain</th>
                <th>Protocol</th>
                <th>Client</th>
                <th>Status</th>
                <th>Created</th>
                <th>Last Active</th>
                <th>Actions</th>
            </tr>
        </thead>
        <tbody>
            {{range .Tunnels}}
            <tr>
                <td><code>{{.ID}}</code></td>
                <td>{{.Subdomain}}</td>
                <td><span class="badge badge-info">{{.Protocol}}</span></td>
                <td><code>{{.ClientID}}</code></td>
                <td>{{if .Connected}}<span class="badge badge-success">Connected</span>{{else}}<span class="badge badge-warning">Disconnected</span>{{end}}</td>
                <td>{{formatTime .CreateTime}}</td>
                <td>{{timeSince .LastActive}}</td>
                <td>
                    <button class="btn btn-danger btn-sm"
                            hx-post="{{$.PathPrefix}}/api/tunnels/{{.ID}}/kill"
                            hx-target="#tunnels-container"
                            hx-confirm="Kill tunnel {{.ID}}?">
                        Kill
                    </button>
                </td>
            </tr>
            {{end}}
        </tbody>
    </table>
</div>
{{else}}
<div class="empty-state">
    <p>No active tunnels</p>
</div>
{{end}}`

const clientsTemplate = `<div class="page-header">
    <h1>Connected Clients</h1>
</div>
<div id="clients-container" hx-get="{{.PathPrefix}}/api/clients" hx-trigger="every 5s" hx-swap="innerHTML">
    {{template "clients_fragment" .}}
</div>`

const clientsFragmentTemplate = `{{if .Clients}}
<div class="table-container">
    <table class="data-table">
        <thead>
            <tr>
                <th>Client ID</th>
                <th>Tunnels</th>
                <th>Last Active</th>
            </tr>
        </thead>
        <tbody>
            {{range .Clients}}
            <tr>
                <td><code>{{.ID}}</code></td>
                <td>{{.TunnelCount}}</td>
                <td>{{timeSince .LastActive}}</td>
            </tr>
            {{end}}
        </tbody>
    </table>
</div>
{{else}}
<div class="empty-state">
    <p>No connected clients</p>
</div>
{{end}}`

const settingsTemplate = `<div class="page-header">
    <h1>Server Settings</h1>
</div>
<div class="card">
    <div class="card-header"><h3>Server Configuration</h3></div>
    <div class="card-body">
        <table class="config-table">
            <tr><td class="config-label">Bind Address</td><td>{{.Config.BindAddress}}</td></tr>
            <tr><td class="config-label">Port</td><td>{{.Config.Port}}</td></tr>
            <tr><td class="config-label">Base Domain</td><td>{{.Config.BaseDomain}}</td></tr>
            <tr><td class="config-label">OAuth2 Enabled</td><td>{{.Config.OAuth2.Enabled}}</td></tr>
            <tr><td class="config-label">MongoDB Enabled</td><td>{{.Config.MongoDB.Enabled}}</td></tr>
            <tr><td class="config-label">Redis Enabled</td><td>{{.Config.Redis.Enabled}}</td></tr>
            <tr><td class="config-label">Let's Encrypt</td><td>{{.Config.LetsEncrypt.Enabled}}</td></tr>
            <tr><td class="config-label">Max Tunnels/User</td><td>{{.Config.TunnelLimits.MaxPerUser}}</td></tr>
        </table>
    </div>
</div>
<div class="card" style="margin-top: 1.5rem;">
    <div class="card-header"><h3>Maintenance Mode</h3></div>
    <div class="card-body" id="maintenance-container">
        {{template "maintenance_fragment" .}}
    </div>
</div>`

const maintenanceFragmentTemplate = `<p>Maintenance mode is currently <strong>{{if .MaintenanceMode}}enabled{{else}}disabled{{end}}</strong>.</p>
<button class="btn {{if .MaintenanceMode}}btn-success{{else}}btn-warning{{end}}"
        hx-post="{{.PathPrefix}}/api/settings/maintenance"
        hx-target="#maintenance-container"
        hx-swap="innerHTML">
    {{if .MaintenanceMode}}Disable Maintenance Mode{{else}}Enable Maintenance Mode{{end}}
</button>`
