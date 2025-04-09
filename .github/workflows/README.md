# GitHub Workflow for Building nxpose Packages

This workflow builds and packages the nxpose server for different platforms and architectures, and uploads the packages to MinIO S3 storage.

## Workflow Overview

The workflow consists of two main jobs:

1. **Build**: Builds the nxpose server packages for:
   - Architectures: `amd64` and `arm64`
   - Package formats: `deb` (Ubuntu/Debian) and `rpm` (RHEL/CentOS/Fedora)

2. **Upload to MinIO**: Uploads the built packages to a MinIO S3 bucket.

## Workflow Triggers

The workflow runs on:
- Push to `main` branch
- New tag creation (tags starting with `v`)
- Pull requests to `main` branch
- Manual trigger via GitHub UI

## Version Handling

- When triggered by a tag (e.g., `v1.2.3`), the version in the package will match the tag (without the 'v' prefix).
- For other triggers, a development version is used with a timestamp: `1.0.0-dev.YYYYMMDDHHMMSS`.

## Required Secrets

To enable uploading to MinIO S3 storage, you must set up the following secrets in your GitHub repository:

- `MINIO_ENDPOINT`: The URL of your MinIO server (e.g., `https://minio.example.com`)
- `MINIO_ACCESS_KEY`: The access key for your MinIO server
- `MINIO_SECRET_KEY`: The secret key for your MinIO server

## Setting Up Secrets

1. Go to your GitHub repository settings
2. Navigate to Secrets and Variables > Actions
3. Click "New repository secret"
4. Add each of the required secrets mentioned above

## Package Storage Structure

Packages are stored in the `nxpose-packages` bucket with the following structure:

- For tag releases (e.g., `v1.2.3`):
  - `1.2.3/nxpose_1.2.3_amd64.deb`
  - `1.2.3/nxpose_1.2.3_arm64.deb`
  - `1.2.3/nxpose_1.2.3_amd64.rpm`
  - `1.2.3/nxpose_1.2.3_arm64.rpm`
  - Also copied to `latest/` directory

- For non-tag releases:
  - `latest/nxpose_1.0.0-dev.YYYYMMDDHHMMSS_amd64.deb`
  - And similar for other architectures/formats

## Accessing the Packages

You can access the packages using the MinIO client or any S3-compatible tool:

```bash
# Using MinIO client
mc cp minio/nxpose-packages/latest/nxpose_1.0.0_amd64.deb .

# Using AWS CLI (if configured to work with your MinIO)
aws s3 cp s3://nxpose-packages/latest/nxpose_1.0.0_amd64.deb .
``` 