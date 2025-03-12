# NXpose Documentation Guide

Complete guide for building, running, and deploying the nxpose documentation site.

## Quick Start

The fastest way to run the documentation site locally:

```bash
# Start the site
docker-compose -f docker-compose.site.yml up -d

# View at http://localhost:8080
```

## Table of Contents

- [Running Locally](#running-locally)
- [Development Mode](#development-mode)
- [Building](#building)
- [Deployment](#deployment)
- [Customization](#customization)

## Running Locally

### Method 1: Docker Compose (Recommended)

**Production mode** - Serves the built static site with nginx:

```bash
# Start
docker-compose -f docker-compose.site.yml up -d

# View logs
docker-compose -f docker-compose.site.yml logs -f

# Stop
docker-compose -f docker-compose.site.yml down

# Rebuild after changes
docker-compose -f docker-compose.site.yml up -d --build
```

Access at: **http://localhost:8080**

**Development mode** - Live reload on file changes:

```bash
# Start dev server
docker-compose -f docker-compose.site.yml --profile dev up nxpose-site-dev

# Edit files in site/docs/ and see changes immediately
# Press Ctrl+C to stop
```

Access at: **http://localhost:8000**

### Method 2: Make Commands

```bash
# Build and run with Docker
make site

# Serve locally (requires mkdocs installed)
make site-serve
```

### Method 3: MkDocs Directly

Install dependencies first:

```bash
pip install mkdocs-material mkdocs-minify-plugin
```

Then:

```bash
cd site

# Development server with live reload
mkdocs serve
# Access at http://127.0.0.1:8000

# Build static files
mkdocs build
# Output in site/site/
```

## Development Mode

For active documentation development, use the live reload server:

```bash
# Using Docker Compose
docker-compose -f docker-compose.site.yml --profile dev up nxpose-site-dev

# OR using MkDocs directly
cd site && mkdocs serve
```

**Features:**
- Instant reload on file changes
- No need to rebuild
- Perfect for writing/editing documentation

**Edit these files:**
- `README.md` (becomes home page)
- `site/docs/*.md` (all documentation pages)
- `site/docs/stylesheets/extra.css` (styling)
- `site/mkdocs.yml` (configuration)

## Building

### Docker Build

```bash
# Using Make
make site

# Using Docker directly
docker build -f Dockerfile.site -t nxpose-site .

# Using Docker Compose
docker-compose -f docker-compose.site.yml build
```

### Static Build

```bash
cd site
mkdocs build

# Output will be in site/site/
# Copy these files to any static hosting
```

## Deployment

### 1. GitHub Pages

Automatic deployment to GitHub Pages:

```bash
cd site
mkdocs gh-deploy
```

Your site will be available at `https://yourusername.github.io/nxpose/`

### 2. Netlify

**Option A: Netlify CLI**

```bash
# Install
npm install -g netlify-cli

# Deploy
cd site && mkdocs build
netlify deploy --prod --dir=site
```

**Option B: Netlify UI**

1. Connect your GitHub repository
2. Configure build settings:
   - **Build command:** `cd site && mkdocs build`
   - **Publish directory:** `site/site`
3. Deploy

### 3. Docker Deployment

**Build and push:**

```bash
# Build
docker build -f Dockerfile.site -t your-registry/nxpose-site:latest .

# Push to registry
docker push your-registry/nxpose-site:latest
```

**Deploy on server:**

```bash
# Pull and run
docker pull your-registry/nxpose-site:latest
docker run -d -p 80:80 --name nxpose-site your-registry/nxpose-site:latest

# Or with compose
# Create docker-compose.yml on server:
version: '3.8'
services:
  site:
    image: your-registry/nxpose-site:latest
    ports:
      - "80:80"
    restart: always

# Deploy
docker-compose up -d
```

### 4. Static Hosting (S3, Cloudflare Pages, Vercel)

```bash
# Build
cd site && mkdocs build

# Upload site/site/ directory to:
# - AWS S3 + CloudFront
# - Cloudflare Pages
# - Vercel
# - Any static hosting service
```

**Example: AWS S3**

```bash
cd site
mkdocs build
aws s3 sync site/ s3://your-bucket/ --delete
aws cloudfront create-invalidation --distribution-id YOUR_DIST_ID --paths "/*"
```

### 5. Kubernetes

Create a deployment:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nxpose-site
spec:
  replicas: 2
  selector:
    matchLabels:
      app: nxpose-site
  template:
    metadata:
      labels:
        app: nxpose-site
    spec:
      containers:
      - name: site
        image: your-registry/nxpose-site:latest
        ports:
        - containerPort: 80
---
apiVersion: v1
kind: Service
metadata:
  name: nxpose-site
spec:
  selector:
    app: nxpose-site
  ports:
  - port: 80
    targetPort: 80
  type: LoadBalancer
```

## Customization

### Theme Colors

Edit `site/mkdocs.yml`:

```yaml
theme:
  palette:
    - scheme: slate
      primary: indigo    # Change this
      accent: indigo     # Change this
```

### Custom CSS

Edit `site/docs/stylesheets/extra.css`:

```css
:root {
  --md-primary-fg-color: #4F46E5;  /* Primary color */
  --md-accent-fg-color: #818CF8;   /* Accent color */
}
```

### Navigation

Edit `site/mkdocs.yml`:

```yaml
nav:
  - Home: index.md
  - Getting Started:
      - Installation: installation.md
      - Quick Start: quickstart.md
  # Add more sections here
```

### Logo and Favicon

Replace these files:
- `site/docs/icon.png` - Site logo
- `site/docs/favicon.svg` - Browser favicon

### Add New Pages

1. Create new `.md` file in `site/docs/`
2. Add to navigation in `site/mkdocs.yml`
3. Rebuild

## File Structure

```
nxpose/
├── README.md                      # Main docs → becomes index.md
├── DOCUMENTATION.md               # This file
├── docker-compose.site.yml        # Docker Compose config
├── Dockerfile.site                # Docker build for site
├── Makefile                       # Build targets
│
└── site/                          # Documentation site
    ├── mkdocs.yml                 # MkDocs configuration
    ├── README.md                  # Site README
    │
    ├── docs/                      # Documentation content
    │   ├── index.md               # Auto-generated from README.md
    │   ├── installation.md
    │   ├── quickstart.md
    │   ├── use-cases.md
    │   ├── config-reference.md
    │   ├── security.md
    │   ├── contributing.md
    │   ├── faq.md
    │   │
    │   ├── client/                # Client documentation
    │   │   ├── registration.md
    │   │   ├── exposing-services.md
    │   │   ├── tcp-tunnels.md
    │   │   └── configuration.md
    │   │
    │   ├── server/                # Server documentation
    │   │   ├── setup.md
    │   │   ├── configuration.md
    │   │   ├── tls.md
    │   │   ├── oauth2.md
    │   │   └── database.md
    │   │
    │   ├── architecture/          # Architecture docs
    │   │   ├── overview.md
    │   │   ├── components.md
    │   │   ├── request-flow.md
    │   │   └── security.md
    │   │
    │   └── stylesheets/           # Custom CSS
    │       └── extra.css
    │
    └── overrides/                 # Theme overrides
        └── partials/
```

## Troubleshooting

### Port Conflict

If port 8080 is in use:

```bash
# Check what's using the port
lsof -i :8080

# Change port in docker-compose.site.yml
ports:
  - "8081:80"  # Use 8081 instead
```

### Build Failures

```bash
# Clean rebuild
docker-compose -f docker-compose.site.yml down
docker-compose -f docker-compose.site.yml build --no-cache
docker-compose -f docker-compose.site.yml up -d
```

### Changes Not Appearing

**Production mode:**
```bash
# Rebuild the image
docker-compose -f docker-compose.site.yml up -d --build --force-recreate
```

**Development mode:**
- Changes should auto-reload
- Check that you're editing files in `site/docs/`
- Check terminal for errors

### MkDocs Not Found

```bash
# Install MkDocs and plugins
pip install mkdocs-material mkdocs-minify-plugin

# Verify installation
mkdocs --version
```

## CI/CD Integration

### GitHub Actions

Create `.github/workflows/docs.yml`:

```yaml
name: Deploy Documentation

on:
  push:
    branches: [main]
  workflow_dispatch:

jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3

      - name: Setup Python
        uses: actions/setup-python@v4
        with:
          python-version: 3.x

      - name: Install dependencies
        run: |
          pip install mkdocs-material mkdocs-minify-plugin

      - name: Deploy to GitHub Pages
        run: |
          cd site
          mkdocs gh-deploy --force
```

## Resources

- [MkDocs Documentation](https://www.mkdocs.org/)
- [Material for MkDocs](https://squidfunk.github.io/mkdocs-material/)
- [Material Icons](https://squidfunk.github.io/mkdocs-material/reference/icons-emojis/)
- [Markdown Extensions](https://squidfunk.github.io/mkdocs-material/setup/extensions/)
- [Docker Documentation](https://docs.docker.com/)

---

For more details, see `site/README.md` and `docs/SITE.md`.
