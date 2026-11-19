---
title: Docker Deployment
---

This guide describes the Docker image for running FinFocus in a container.
The image is built from `docker/Dockerfile` in the repository. The
`Docker Build and Push` workflow (`.github/workflows/docker.yml`) builds it for
`linux/amd64` and `linux/arm64` on every `v*` release tag and publishes it to
the GitHub Container Registry as `ghcr.io/rshade/finfocus`.

## Quick Start

### Pull from GitHub Container Registry

```bash
# Pull the most recent release
docker pull ghcr.io/rshade/finfocus:latest

# Pull a specific version: <version> without a leading "v", for example 0.5.8
docker pull ghcr.io/rshade/finfocus:<version>
```

Each release tag `vX.Y.Z` publishes the image tags `X.Y.Z`, `X.Y`, `X`, and
`latest`.

### Build Locally

Run the build from the repository root, because the build context is the whole
repository. The Dockerfile uses BuildKit cache mounts, so it needs BuildKit,
the default builder since Docker Engine 23.0.

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

- **Base Image**: Alpine Linux (`alpine:latest`)
- **Go Version**: 1.27.4 (golang:1.27.4-alpine)
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

### Version Information

The Dockerfile doesn't declare build arguments. The builder stage sets the
version from `git describe`, the commit from `git rev-parse HEAD`, and the
build date from `date`, and passes them to the binary through `-ldflags`.
`.dockerignore` excludes `.git/`, so the build has no Git metadata: the
binary reports version `docker` and commit `unknown`. Published images do
the same (`finfocus --version` prints `finfocus version docker`), tracked in
[#2237](https://github.com/rshade/finfocus/issues/2237). The image tag is the
release version.

### Health Check

The image includes a health check that runs `finfocus --help`:

```bash
# Check container health
docker inspect --format='{{.State.Health.Status}}' <container_id>
```

## Troubleshooting

### Permission Issues

If you encounter permission issues with volume mounts:

```bash
# Ensure proper ownership of plugin directories
sudo chown -R 1001:1001 ~/.finfocus
```

### Plugin Installation

The image has no plugins. Install them with `finfocus plugin install`, or
copy each plugin into `~/.finfocus/plugins/<name>/<version>/`. Mount that
directory as shown in [Plugin Management](#plugin-management) to keep plugins
between runs.
