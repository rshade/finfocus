---
title: Deployment Configuration
layout: default
---

Use this guide to configure FinFocus across local, CI/CD, and containerized deployments.
Refer to the [Configuration Reference](../reference/config-reference.html) for the full schema.

## Configuration Sources

FinFocus loads configuration in this order:

1. CLI flags (highest priority)
2. Environment variables
3. `config.yaml` (default: `~/.finfocus/config.yaml`)

Use environment variables for secrets and pipeline-provided settings to avoid
committing sensitive data.

## File-Based Configuration

The default configuration file lives at `~/.finfocus/config.yaml`.

```yaml
output:
  default_format: table
  precision: 2

logging:
  level: info

plugins:
  dir: ~/.finfocus/plugins
```

### Recommended Defaults

- Keep `output.default_format` at `table` for human-readable local output
- Use `json` or `ndjson` formats in CI for structured logs
- Keep plugin directories in user home for easy caching

## Environment Variables

Environment variables are ideal for CI/CD and container deployments.

- `FINFOCUS_LOG_LEVEL`: Set logging verbosity (debug, info, warn, error)
- `FINFOCUS_CONFIG_FILE`: Path to a custom configuration file
- `FINFOCUS_PLUGIN_DIR`: Override the plugin directory

See the full list in [Environment Variables](../reference/environment-variables.html).

## Deployment Examples

### Local Workstation

```bash
# Use the default config file
echo "logging:\n  level: debug" > ~/.finfocus/config.yaml
finfocus cost projected --pulumi-json plan.json
```

### CI/CD Runner

```bash
export FINFOCUS_LOG_LEVEL=info
export FINFOCUS_PLUGIN_DIR="$HOME/.finfocus/plugins"
finfocus cost projected --pulumi-json plan.json
```

### Docker Container

```bash
docker run --rm \
  -e FINFOCUS_LOG_LEVEL=info \
  -v ~/.finfocus:/home/finfocus/.finfocus \
  -v $(pwd):/workspace \
  ghcr.io/rshade/finfocus:latest \
  cost projected --pulumi-json /workspace/plan.json
```

## Secret Management

- Use CI secret stores (GitHub Actions secrets, GitLab variables)
- For Kubernetes, map secrets to environment variables
- For Docker, prefer Docker secrets or mounted credential files

Pair this guide with [Security](security.html) for more credential handling details.
