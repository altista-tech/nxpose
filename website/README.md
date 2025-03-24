# NXpose Website

This directory contains the website for the NXpose project, a secure tunneling service that allows exposing local services to the internet through encrypted tunnels.

## Directory Structure

- `index.html` - The main landing page
- `server.html` - Server configuration documentation
- `docs.html` - Comprehensive documentation
- `assets/css/` - CSS stylesheets
- `assets/js/` - JavaScript files
- `assets/img/` - Images and icons

## Features

- Responsive design using Bootstrap 5
- Light and dark theme support with auto-detection of system preferences
- Interactive components (tabs, accordions, tooltips)
- Mobile-friendly navigation
- Syntax highlighting for code examples

## Development

The website is built with plain HTML, CSS, and JavaScript using the following libraries:

- Bootstrap 5 for layout and components
- Font Awesome for icons
- Custom styles for theme support

### Running Locally

The website can be served using any static file server. For example:

```bash
# Using Python
python -m http.server

# Using Node.js
npx serve

# Using PHP
php -S localhost:8000
```

### Customization

To customize the website:

1. Modify the CSS in `assets/css/style.css`
2. Update theme colors in both light and dark mode sections
3. Add or remove pages as needed, maintaining the navigation structure

## Deployment

To deploy the website:

1. Upload all files to your web server
2. Ensure proper permissions are set
3. Point your domain to the directory containing these files

## License

This website is part of the NXpose project and is licensed under the same terms as the main project.

## Contact

For questions or support, visit [GitHub repository](https://github.com/nxrvl/nxpose). 