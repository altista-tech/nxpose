package admin

const adminCSS = `
:root {
    --background: #09090b;
    --foreground: #fafafa;
    --card: #0a0a0c;
    --card-foreground: #fafafa;
    --popover: #09090b;
    --popover-foreground: #fafafa;
    --primary: #fafafa;
    --primary-foreground: #18181b;
    --secondary: #27272a;
    --secondary-foreground: #fafafa;
    --muted: #27272a;
    --muted-foreground: #a1a1aa;
    --accent: #27272a;
    --accent-foreground: #fafafa;
    --destructive: #7f1d1d;
    --destructive-foreground: #fafafa;
    --border: #27272a;
    --input: #27272a;
    --ring: #d4d4d8;
    --radius: 0.5rem;
    --sidebar-width: 240px;
    --success: #166534;
    --warning: #854d0e;
    --info: #1e40af;
}

* {
    margin: 0;
    padding: 0;
    box-sizing: border-box;
}

body {
    font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
    background: var(--background);
    color: var(--foreground);
    line-height: 1.6;
}

.admin-layout {
    display: flex;
    min-height: 100vh;
}

/* Sidebar */
.sidebar {
    width: var(--sidebar-width);
    background: var(--card);
    border-right: 1px solid var(--border);
    padding: 1.5rem 0;
    position: fixed;
    height: 100vh;
    overflow-y: auto;
}

.sidebar-header {
    padding: 0 1.5rem 1.5rem;
    border-bottom: 1px solid var(--border);
    display: flex;
    align-items: center;
    gap: 0.5rem;
}

.sidebar-header h2 {
    font-size: 1.25rem;
    font-weight: 600;
}

.sidebar-badge {
    font-size: 0.7rem;
    background: var(--secondary);
    color: var(--muted-foreground);
    padding: 0.15rem 0.5rem;
    border-radius: var(--radius);
    text-transform: uppercase;
    letter-spacing: 0.05em;
}

.nav-list {
    list-style: none;
    padding: 1rem 0;
}

.nav-link {
    display: flex;
    align-items: center;
    gap: 0.75rem;
    padding: 0.625rem 1.5rem;
    color: var(--muted-foreground);
    text-decoration: none;
    font-size: 0.875rem;
    transition: all 0.15s ease;
}

.nav-link:hover {
    color: var(--foreground);
    background: var(--accent);
}

.nav-link.active {
    color: var(--foreground);
    background: var(--accent);
}

.nav-icon {
    width: 1rem;
    height: 1rem;
    flex-shrink: 0;
}

/* Main content */
.main-content {
    margin-left: var(--sidebar-width);
    flex: 1;
    padding: 2rem;
    max-width: calc(100% - var(--sidebar-width));
}

.page-header {
    margin-bottom: 2rem;
}

.page-header h1 {
    font-size: 1.75rem;
    font-weight: 600;
}

/* Stats grid */
.stats-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
    gap: 1rem;
}

.stat-card {
    background: var(--card);
    border: 1px solid var(--border);
    border-radius: var(--radius);
    padding: 1.5rem;
}

.stat-label {
    font-size: 0.8rem;
    color: var(--muted-foreground);
    text-transform: uppercase;
    letter-spacing: 0.05em;
    margin-bottom: 0.5rem;
}

.stat-value {
    font-size: 2rem;
    font-weight: 700;
}

/* Cards */
.card {
    background: var(--card);
    border: 1px solid var(--border);
    border-radius: var(--radius);
    overflow: hidden;
}

.card-header {
    padding: 1rem 1.5rem;
    border-bottom: 1px solid var(--border);
    display: flex;
    justify-content: space-between;
    align-items: center;
}

.card-header h3 {
    font-size: 1rem;
    font-weight: 600;
}

.card-body {
    padding: 1.5rem;
}

/* Tables */
.table-container {
    background: var(--card);
    border: 1px solid var(--border);
    border-radius: var(--radius);
    overflow: hidden;
}

.data-table {
    width: 100%;
    border-collapse: collapse;
}

.data-table th {
    text-align: left;
    padding: 0.75rem 1rem;
    font-size: 0.8rem;
    color: var(--muted-foreground);
    text-transform: uppercase;
    letter-spacing: 0.05em;
    border-bottom: 1px solid var(--border);
    background: var(--secondary);
}

.data-table td {
    padding: 0.75rem 1rem;
    border-bottom: 1px solid var(--border);
    font-size: 0.875rem;
}

.data-table tr:last-child td {
    border-bottom: none;
}

.data-table code {
    font-family: ui-monospace, SFMono-Regular, "SF Mono", Menlo, monospace;
    font-size: 0.8rem;
    background: var(--secondary);
    padding: 0.15rem 0.4rem;
    border-radius: 0.25rem;
}

/* Config table */
.config-table {
    width: 100%;
    border-collapse: collapse;
}

.config-table td {
    padding: 0.5rem 0;
    font-size: 0.875rem;
    border-bottom: 1px solid var(--border);
}

.config-table tr:last-child td {
    border-bottom: none;
}

.config-label {
    color: var(--muted-foreground);
    width: 200px;
    font-weight: 500;
}

/* Badges */
.badge {
    display: inline-block;
    font-size: 0.7rem;
    font-weight: 600;
    padding: 0.2rem 0.6rem;
    border-radius: 9999px;
    text-transform: uppercase;
    letter-spacing: 0.05em;
}

.badge-success {
    background: var(--success);
    color: #bbf7d0;
}

.badge-warning {
    background: var(--warning);
    color: #fef08a;
}

.badge-info {
    background: var(--info);
    color: #bfdbfe;
}

.badge-danger {
    background: var(--destructive);
    color: #fecaca;
}

/* Buttons */
.btn {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    border-radius: var(--radius);
    font-size: 0.875rem;
    font-weight: 500;
    padding: 0.5rem 1rem;
    cursor: pointer;
    border: 1px solid var(--border);
    background: var(--secondary);
    color: var(--foreground);
    transition: all 0.15s ease;
}

.btn:hover {
    background: var(--accent);
}

.btn-sm {
    font-size: 0.75rem;
    padding: 0.25rem 0.5rem;
}

.btn-danger {
    background: var(--destructive);
    border-color: var(--destructive);
    color: #fecaca;
}

.btn-danger:hover {
    opacity: 0.9;
}

.btn-success {
    background: var(--success);
    border-color: var(--success);
    color: #bbf7d0;
}

.btn-warning {
    background: var(--warning);
    border-color: var(--warning);
    color: #fef08a;
}

/* Empty state */
.empty-state {
    text-align: center;
    padding: 3rem;
    color: var(--muted-foreground);
    background: var(--card);
    border: 1px solid var(--border);
    border-radius: var(--radius);
}

/* Responsive */
@media (max-width: 768px) {
    .sidebar {
        width: 100%;
        height: auto;
        position: relative;
    }
    .main-content {
        margin-left: 0;
        max-width: 100%;
    }
    .admin-layout {
        flex-direction: column;
    }
    .stats-grid {
        grid-template-columns: repeat(2, 1fr);
    }
}
`

const adminJS = `
// Admin panel JavaScript
document.addEventListener('DOMContentLoaded', function() {
    // HTMX is loaded via CDN and handles all dynamic updates
    // This file is for any additional client-side behavior

    // Add loading indicator for HTMX requests
    document.body.addEventListener('htmx:beforeRequest', function(event) {
        var target = event.detail.target;
        if (target) {
            target.style.opacity = '0.6';
        }
    });

    document.body.addEventListener('htmx:afterRequest', function(event) {
        var target = event.detail.target;
        if (target) {
            target.style.opacity = '1';
        }
    });
});
`
