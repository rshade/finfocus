---
title: Docker Deployment
layout: default
---

This guide describes the Docker configuration for building and running
FinFocus in a containerized environment.

## Quick Start

### Pull from GitHub Container Registry

```bash
# Pull the latest release
docker pull ghcr.io/rshade/finfocus:latest

# Pull a specific version
docker pull ghcr.io/rshade/finfocus:v1.0.0
```

### Build Locally

```bash
# Build the image
docker build -f docker/Dockerfile -t finfocus:local .

# Run the help command
docker run --rm finfocus:local --help
```

## Usage Examples

### Basic Commands

```bash
# Show help
docker run --rm ghcr.io/rshade/finfocus:latest --help

# List plugins
docker run --rm ghcr.io/rshade/finfocus:latest plugin list

# Validate plugins
docker run --rm ghcr.io/rshade/finfocus:latest plugin validate
```

### Cost Analysis with Volume Mounts

```bash
# Calculate projected costs from a local Pulumi plan
docker run --rm \
  -v $(pwd):/workspace \
  ghcr.io/rshade/finfocus:latest \
  cost projected --pulumi-json /workspace/plan.json

# Get actual costs with configuration
docker run --rm \
  -v $(pwd):/workspace \
  -v ~/.finfocus:/home/finfocus/.finfocus \
  ghcr.io/rshade/finfocus:latest \
  cost actual --from 2024-01-01 --to 2024-01-31
```

### Plugin Management

```bash
# Mount plugin directory to persist plugins
docker run --rm \
  -v ~/.finfocus/plugins:/home/finfocus/.finfocus/plugins \
  ghcr.io/rshade/finfocus:latest \
  plugin list
```

## Image Details

- **Base Image**: Alpine Linux (latest)
- **Go Version**: 1.25.7 (golang:1.25.7-alpine)
- **User**: Non-root user `finfocus` (UID: 1001, GID: 1001)
- **Working Directory**: `/home/finfocus`
- **Plugin Directory**: `/home/finfocus/.finfocus/plugins`
- **Specs Directory**: `/home/finfocus/.finfocus/specs`

## Security Features

- Runs as non-root user for enhanced security
- Multi-stage build to minimize image size
- Health check included for container monitoring
- SBOM (Software Bill of Materials) generated during CI builds
- Vulnerability scanning with Trivy

## Environment Variables

The container respects the following environment variables:

- `HOME`: Set to `/home/finfocus`
- `PATH`: Includes `/usr/local/bin` for the finfocus binary

## Persistent Data

To persist plugins and configuration between container runs:

```bash
# Create local directories
mkdir -p ~/.finfocus/{plugins,specs}

# Run with persistent volumes
docker run --rm \
  -v ~/.finfocus:/home/finfocus/.finfocus \
  -v $(pwd):/workspace \
  ghcr.io/rshade/finfocus:latest \
  cost projected --pulumi-json /workspace/plan.json
```

## Development

### Build Arguments

The Dockerfile supports the following build-time variables:

- Git version information is automatically embedded during build
- Build date and commit information included in the binary

### Health Check

The image includes a health check that runs `finfocus --help`:

<!-- markdownlint-disable MD031 -->

{% raw %}

```bash
# Check container health
docker inspect --format='{{.State.Health.Status}}' <container_id>
```

{% endraw %}

<!-- markdownlint-enable MD031 -->

## Troubleshooting

### Permission Issues

If you encounter permission issues with volume mounts:

```bash
# Ensure proper ownership of plugin directories
sudo chown -R 1001:1001 ~/.finfocus
```

### Plugin Installation

Currently, plugins must be manually installed in the `~/.finfocus/plugins`
directory. Future versions will include automated plugin downloading capabilities.
