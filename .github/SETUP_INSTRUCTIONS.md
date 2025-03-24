# GitHub Actions Setup for NXpose Packages

This repository includes a GitHub Actions workflow for building and uploading NXpose packages to DigitalOcean Spaces (S3-compatible storage).

## Required Secrets

You need to set up the following secrets in your GitHub repository to enable the workflow:

1. `DO_SPACE_KEY`: Your DigitalOcean Spaces API key
2. `DO_SPACE_SECRET`: Your DigitalOcean Spaces API secret
3. `DO_SPACE_ENDPOINT`: The endpoint for your DigitalOcean space (e.g., `nyc3.digitaloceanspaces.com`)
4. `DO_SPACE_BUCKET`: Your DigitalOcean space name (bucket name)

## Setting Up Secrets

1. Go to your GitHub repository
2. Navigate to "Settings" > "Secrets and variables" > "Actions"
3. Click "New repository secret"
4. Add each of the secrets listed above with their respective values

## How the Workflow Works

The workflow will:

1. Trigger automatically when a tag starting with 'v' is pushed (e.g., v1.0.0)
2. Can also be triggered manually through the "workflow_dispatch" event
3. Build packages for all supported platforms using the Makefile
4. Upload the packages to your DigitalOcean Space
5. Create an index.html file with links to all packages
6. If triggered by a tag, create a GitHub release with links to the packages

## Testing the Workflow

To test the workflow manually:

1. Go to the "Actions" tab in your repository
2. Select the "Build and Upload Packages" workflow
3. Click "Run workflow"
4. Optionally provide a version number or leave blank to use the git-derived version
5. Click "Run workflow"

## Package URLs

After the workflow completes, packages will be available at:
```
https://{DO_SPACE_BUCKET}.{DO_SPACE_ENDPOINT}/nxpose/{VERSION}/{FILENAME}
```

And the index page will be at:
```
https://{DO_SPACE_BUCKET}.{DO_SPACE_ENDPOINT}/nxpose/{VERSION}/index.html
``` 