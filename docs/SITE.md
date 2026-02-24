# Documentation Site

This document explains how to build and run the nxpose documentation site.

## Quick Start

```bash
# Build and run the site
docker-compose -f docker-compose.site.yml up -d

# View at http://localhost:8080
```

## Available Methods

### 1. Docker Compose (Recommended)

#### Production Site

```bash
# Start the site
docker-compose -f docker-compose.site.yml up -d

# View logs
docker-compose -f docker-compose.site.yml logs -f nxpose-site

# Stop the site
docker-compose -f docker-compose.site.yml down

# Rebuild after changes
docker-compose -f docker-compose.site.yml up -d --build
```

Access at: http://localhost:8080

#### Development Mode (Live Reload)

```bash
# Start development server
docker-compose -f docker-compose.site.yml --profile dev up nxpose-site-dev

# Make changes to .md files and see them reload automatically
# Ctrl+C to stop
```

Access at: http://localhost:8000

### 2. Makefile

```bash
# Build site
make site

# Serve locally (requires mkdocs installed)
make site-serve
```

### 3. MkDocs Directly

Requires: `pip install mkdocs-material mkdocs-minify-plugin`

```bash
cd site

# Serve with live reload
mkdocs serve

# Build static files
mkdocs build
```

## File Structure

```
nxpose/
├── README.md                      # Main documentation (becomes index.md)
├── docker-compose.site.yml        # Docker Compose for site
├── Dockerfile.site                # Dockerfile for building site
├── Makefile                       # Build targets including 'site'
└── site/
    ├── mkdocs.yml                 # MkDocs configuration
    ├── docs/                      # Documentation pages
    │   ├── index.md               # Generated from README.md
    │   ├── installation.md
    │   ├── quickstart.md
    │   ├── client/                # Client docs
    │   ├── server/                # Server docs
    │   ├── architecture/          # Architecture docs
    │   └── stylesheets/
    │       └── extra.css          # Custom styling
    └── overrides/                 # Theme customizations
```

## Deployment Options

### GitHub Pages

```bash
cd site
mkdocs gh-deploy
```

### Netlify

1. Connect repository to Netlify
2. Build command: `cd site && mkdocs build`
3. Publish directory: `site/site`

### Docker Registry

```bash
# Build
docker build -f Dockerfile.site -t your-registry/nxpose-site:latest .

# Push
docker push your-registry/nxpose-site:latest

# Deploy
docker run -d -p 80:80 your-registry/nxpose-site:latest
```

### Static Hosting

```bash
cd site
mkdocs build
# Upload site/site/ contents to your hosting provider
```

## Customization

### Styling

Edit `site/docs/stylesheets/extra.css` for custom CSS.

### Theme

Modify `site/mkdocs.yml` to customize:
- Colors (`theme.palette`)
- Features (`theme.features`)
- Navigation (`nav`)
- Markdown extensions

### Content

- Edit pages in `site/docs/`
- Update navigation in `site/mkdocs.yml`
- Main content comes from root `README.md`

## Troubleshooting

### Port Already in Use

```bash
# Change port in docker-compose.site.yml
# Or stop conflicting service
sudo lsof -i :8080
```

### Build Errors

```bash
# Rebuild from scratch
docker-compose -f docker-compose.site.yml down
docker-compose -f docker-compose.site.yml up -d --build --force-recreate
```

### Changes Not Showing

```bash
# Rebuild the image
docker-compose -f docker-compose.site.yml build --no-cache
docker-compose -f docker-compose.site.yml up -d
```

## Resources

- [MkDocs](https://www.mkdocs.org/)
- [MkDocs Material](https://squidfunk.github.io/mkdocs-material/)
- [Docker Compose](https://docs.docker.com/compose/)
