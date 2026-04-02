---
layout: default
title: Plugin Registry Submission
description: How to add your plugin to the FinFocus registry
---

This guide explains how to submit your plugin to the FinFocus plugin registry.

## Prerequisites

Before submitting, ensure your plugin:

1. **Implements the FinFocus plugin protocol** - Uses gRPC with the finfocus-spec protobuf definitions
2. **Has GitHub releases** - Uses GoReleaser or similar to create releases with platform-specific binaries
3. **Follows naming conventions** - Binary naming must match GoReleaser v2 format

## Binary Naming Convention

Your release assets must follow this naming pattern:

```text
{plugin-name}_{version}_{os}_{arch}.{format}
```

Examples:

- `kubecost_v1.0.0_linux_amd64.tar.gz`
- `kubecost_v1.0.0_darwin_arm64.tar.gz`
- `kubecost_v1.0.0_windows_amd64.zip`

**Supported platforms:**

- Linux: amd64, arm64
- macOS (darwin): amd64, arm64
- Windows: amd64 (zip format required)

## Registry Entry Format

Add your plugin to `internal/registry/registry.json`:

```json
{
  "schema_version": "1.0.0",
  "plugins": {
    "your-plugin": {
      "name": "your-plugin",
      "description": "Brief description of what your plugin does",
      "repository": "owner/repo-name",
      "author": "Your Name or Organization",
      "license": "Apache-2.0",
      "homepage": "https://github.com/owner/repo-name",
      "supported_providers": ["aws", "gcp", "azure"],
      "capabilities": ["projected", "actual"],
      "security_level": "community",
      "min_spec_version": "0.1.0"
    }
  }
}
```

### Required Fields

| Field         | Description                                            |
| ------------- | ------------------------------------------------------ |
| `name`        | Plugin name (used in `plugin install <name>`)          |
| `description` | Brief description (shown in `plugin list --available`) |
| `repository`  | GitHub repository in `owner/repo` format               |

### Optional Fields

| Field                 | Description                                                         |
| --------------------- | ------------------------------------------------------------------- |
| `author`              | Plugin author or organization                                       |
| `license`             | SPDX license identifier (e.g., "Apache-2.0", "MIT")                 |
| `homepage`            | URL to documentation or homepage                                    |
| `supported_providers` | Cloud providers supported (aws, gcp, azure, kubernetes)             |
| `capabilities`        | Features: "projected" (cost estimates), "actual" (historical costs) |
| `security_level`      | Trust level (see below)                                             |
| `min_spec_version`    | Minimum finfocus-spec version required                              |

### Security Levels

| Level          | Description                                     |
| -------------- | ----------------------------------------------- |
| `official`     | Maintained by the FinFocus team, fully reviewed |
| `community`    | Community-maintained, basic review completed    |
| `experimental` | New or untested, use with caution               |

## Submission Process

### 1. Prepare Your Plugin

Ensure your plugin repository has:

- [ ] Working GitHub releases with proper binary naming
- [ ] README with usage instructions
- [ ] LICENSE file
- [ ] At least one release tag (e.g., `v0.1.0`)

### 2. Test Installation

Test that your plugin installs correctly:

```bash
# Test direct URL installation
finfocus plugin install github.com/your-org/your-plugin

# Verify it works
finfocus plugin list
finfocus plugin validate
```

### 3. Submit Pull Request

1. Fork the `finfocus` repository
2. Add your plugin entry to `internal/registry/registry.json`
3. Submit a pull request with:
   - Plugin entry in registry.json
   - Brief description of the plugin's purpose
   - Link to plugin repository

### 4. Review Process

The FinFocus team will:

1. Verify the plugin builds and installs correctly
2. Review basic security considerations
3. Test plugin functionality
4. Assign appropriate security level

## GoReleaser Configuration

Example `.goreleaser.yml` for proper binary naming:

{% raw %}

```yaml
version: 2

builds:
  - id: plugin
    main: ./cmd/plugin
    binary: your-plugin
    goos:
      - linux
      - darwin
      - windows
    goarch:
      - amd64
      - arm64
    ignore:
      - goos: windows
        goarch: arm64

archives:
  - id: plugin
    builds:
      - plugin
    name_template: '{{ .ProjectName }}_{{ .Tag }}_{{ .Os }}_{{ .Arch }}'
    format_overrides:
      - goos: windows
        format: zip

checksum:
  name_template: 'checksums.txt'

release:
  github:
    owner: your-org
    name: your-plugin
```

{% endraw %}

## Example Plugins

Reference these existing plugins:

- **kubecost** - Kubernetes cost analysis via Kubecost API
- **aws-public** - AWS public pricing data

## Questions?

- Open an issue on the [finfocus repository](https://github.com/rshade/finfocus/issues)
- Check existing plugins for implementation examples
