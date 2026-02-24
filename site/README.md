# NXpose Documentation Site

This directory contains the MkDocs-based documentation website for NXpose.

## Structure

```
site/
├── docs/                    # Documentation content
│   ├── index.md            # Home page (generated from README.md)
│   ├── installation.md     # Installation guide
│   ├── quickstart.md       # Quick start guide
│   ├── client/             # Client documentation
│   ├── server/             # Server documentation
│   └── architecture/       # Architecture documentation
├── overrides/              # Theme customizations
├── mkdocs.yml              # MkDocs configuration
└── README.md               # This file
```

## Building the Site

### Using Docker Compose (Recommended)

The easiest way to run the documentation site locally:

```bash
# From the project root - build and run production site
docker-compose -f docker-compose.site.yml up -d

# Open http://localhost:8080 in your browser

# View logs
docker-compose -f docker-compose.site.yml logs -f

# Stop the site
docker-compose -f docker-compose.site.yml down
```

For development with live reload:

```bash
# Run development server with live reload
docker-compose -f docker-compose.site.yml --profile dev up nxpose-site-dev

# Open http://localhost:8000 in your browser
# Changes to markdown files will auto-reload
```

### Using Docker (Alternative)

Build the documentation site using the provided Dockerfile:

```bash
# From the project root
make site

# Or directly with docker
docker build -f Dockerfile.site -t nxpose-site .
```

Run the site locally:

```bash
docker run -d -p 8080:80 --name nxpose-site nxpose-site
# Open http://localhost:8080 in your browser

# Stop the container
docker stop nxpose-site && docker rm nxpose-site
```

### Using MkDocs Directly

If you have MkDocs installed locally:

```bash
# Install dependencies
pip install mkdocs-material mkdocs-minify-plugin

# Serve locally with live reload
cd site
mkdocs serve
# Open http://127.0.0.1:8000 in your browser

# Build static site
mkdocs build
# Output will be in site/site/
```

### Using Make

```bash
# Build with Docker
make site

# Serve locally (requires mkdocs installed)
make site-serve
```

## Customization

### Theme

The site uses [MkDocs Material](https://squidfunk.github.io/mkdocs-material/) theme with custom styling defined in:

- `docs/stylesheets/extra.css` - Custom CSS
- `overrides/` - Theme template overrides

### Navigation

Edit the `nav` section in `mkdocs.yml` to modify the site navigation structure.

### Content

- Main content is generated from the root `README.md` file
- Additional pages are in the `docs/` directory
- To add new pages:
  1. Create a new `.md` file in `docs/` or a subdirectory
  2. Add it to the `nav` section in `mkdocs.yml`

## Deployment

The site can be deployed to various hosting services:

### GitHub Pages

```bash
cd site
mkdocs gh-deploy
```

### Netlify

1. Connect your repository to Netlify
2. Set build command: `cd site && mkdocs build`
3. Set publish directory: `site/site`

### Docker/Kubernetes

Use the generated Docker image:

```bash
docker build -f Dockerfile.site -t your-registry/nxpose-site:latest .
docker push your-registry/nxpose-site:latest
```

### Static Hosting

Build and deploy the static files:

```bash
cd site
mkdocs build
# Upload the contents of site/site/ to your hosting service
```

## References

- [MkDocs](https://www.mkdocs.org/)
- [MkDocs Material](https://squidfunk.github.io/mkdocs-material/)
- [MkDocs Material Extensions](https://squidfunk.github.io/mkdocs-material/reference/)
