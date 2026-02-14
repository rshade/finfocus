# Quickstart: finfocus setup — One-Command Bootstrap

**Date**: 2026-02-14
**Branch**: `591-setup-command`

## Overview

The `finfocus setup` command bootstraps the entire FinFocus environment in a
single invocation. It creates directories, writes default configuration,
installs the Pulumi analyzer, and installs default plugins.

## Usage

### Basic Setup

```bash
finfocus setup
```

### CI/CD Setup (no TTY-dependent output)

```bash
finfocus setup --non-interactive
```

### Minimal Setup (directories and config only)

```bash
finfocus setup --skip-analyzer --skip-plugins
```

### Custom Home Directory

```bash
export FINFOCUS_HOME=/opt/finfocus
finfocus setup
```

## Flags

| Flag               | Description                              |
|--------------------|------------------------------------------|
| `--non-interactive`| Disable TTY-dependent output (ASCII markers, no color) |
| `--skip-analyzer`  | Skip Pulumi analyzer installation        |
| `--skip-plugins`   | Skip default plugin installation         |

## Expected Output (TTY)

```text
FinFocus v0.3.0 (go1.25.7)
✓ Pulumi CLI detected (v3.210.0)
✓ Created ~/.finfocus/
✓ Created ~/.finfocus/plugins/
✓ Created ~/.finfocus/cache/
✓ Created ~/.finfocus/logs/
✓ Initialized config (~/.finfocus/config.yaml)
✓ Installed Pulumi analyzer (v0.3.0, symlink)
✓ Installed plugin: aws-public (latest)

Setup complete! Run 'finfocus cost projected --pulumi-json plan.json' to get started.
```

## Expected Output (non-interactive)

```text
FinFocus v0.3.0 (go1.25.7)
[OK] Pulumi CLI detected (v3.210.0)
[OK] Created /home/ci/.finfocus/
[OK] Created /home/ci/.finfocus/plugins/
[OK] Created /home/ci/.finfocus/cache/
[OK] Created /home/ci/.finfocus/logs/
[OK] Initialized config (/home/ci/.finfocus/config.yaml)
[OK] Installed Pulumi analyzer (v0.3.0, symlink)
[OK] Installed plugin: aws-public (latest)

Setup complete! Run 'finfocus cost projected --pulumi-json plan.json' to get started.
```

## Idempotent Re-Run Output

```text
FinFocus v0.3.0 (go1.25.7)
✓ Pulumi CLI detected (v3.210.0)
✓ Directory exists: ~/.finfocus/
✓ Directory exists: ~/.finfocus/plugins/
✓ Directory exists: ~/.finfocus/cache/
✓ Directory exists: ~/.finfocus/logs/
✓ Config already exists (~/.finfocus/config.yaml)
✓ Pulumi analyzer already current (v0.3.0)
✓ Plugin already installed: aws-public

Setup complete! Run 'finfocus cost projected --pulumi-json plan.json' to get started.
```

## Error Handling Examples

### Missing Pulumi (warning, non-fatal)

```text
! Pulumi CLI not found on PATH. Install from https://www.pulumi.com/docs/install/
```

### Permission Denied (critical error)

```text
✗ Failed to create /root/.finfocus/: permission denied
  Try: export FINFOCUS_HOME=/path/to/writable/directory
```

### Plugin Download Failure (warning, non-fatal)

```text
! Failed to install plugin aws-public: network timeout
  Try later: finfocus plugin install aws-public
```

## Development

### Build and Test

```bash
make build
make test
make lint
```

### Run Setup Locally

```bash
./bin/finfocus setup
```

### Test with Temporary Home

```bash
FINFOCUS_HOME=$(mktemp -d) ./bin/finfocus setup
```
