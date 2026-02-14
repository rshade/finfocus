# Quickstart: Analyzer Install/Uninstall

**Feature**: 590-analyzer-install
**Date**: 2026-02-13

## Overview

The `finfocus analyzer install` command replaces the 4-step manual process for setting up the Pulumi Analyzer with a single command.

## Before (Manual Process)

```bash
# Step 1: Find the finfocus binary
BINARY=$(which finfocus)

# Step 2: Determine version
VERSION=$(finfocus version 2>&1 | grep -oP 'v[\d.]+' || echo "v0.1.0")

# Step 3: Create plugin directory
PLUGIN_DIR=~/.pulumi/plugins/analyzer-finfocus-${VERSION}
mkdir -p "$PLUGIN_DIR"

# Step 4: Create symlink
ln -sf "$BINARY" "$PLUGIN_DIR/pulumi-analyzer-finfocus"
```

## After (Single Command)

```bash
# Install
finfocus analyzer install

# Output:
# Analyzer installed successfully
#   Version: v0.2.0
#   Path: /home/user/.pulumi/plugins/analyzer-finfocus-v0.2.0/pulumi-analyzer-finfocus
#   Method: symlink
```

## Common Workflows

### First-Time Setup

```bash
# Install the analyzer
finfocus analyzer install

# Configure Pulumi to use it (add to Pulumi.yaml)
# analyzers:
#   - finfocus

# Run preview with cost estimation
pulumi preview
```

### Upgrade After Updating finfocus

```bash
# After installing a new version of finfocus
finfocus analyzer install --force

# Output:
# Analyzer upgraded successfully
#   Previous version: v0.1.0
#   New version: v0.2.0
#   Path: /home/user/.pulumi/plugins/analyzer-finfocus-v0.2.0/pulumi-analyzer-finfocus
```

### Check Status

```bash
# Without installing, check current state
finfocus analyzer install
# If already installed:
# Analyzer already installed at v0.2.0
#   Path: /home/user/.pulumi/plugins/analyzer-finfocus-v0.2.0/pulumi-analyzer-finfocus
#   Use --force to reinstall
```

### Uninstall

```bash
finfocus analyzer uninstall

# Output:
# Analyzer uninstalled successfully
#   Removed: /home/user/.pulumi/plugins/analyzer-finfocus-v0.2.0/
```

### Custom Plugin Directory

```bash
# Install to a non-default location
finfocus analyzer install --target-dir /opt/pulumi/plugins
```

## Error Messages

### Pulumi Not Found

```text
Warning: Pulumi CLI not found in PATH. The analyzer will be installed but may
not be discovered automatically. Ensure your Pulumi plugin directory is correct
or use --target-dir to specify the location.
```

### Already Installed (No --force)

```text
Analyzer already installed at v0.1.0
  Path: /home/user/.pulumi/plugins/analyzer-finfocus-v0.1.0/pulumi-analyzer-finfocus
  Current finfocus version: v0.2.0
  Use --force to upgrade
```

### Permission Denied

```text
Error: permission denied writing to /home/user/.pulumi/plugins/analyzer-finfocus-v0.2.0/
  Ensure you have write access to the Pulumi plugin directory.
  Alternatively, use --target-dir to install to a writable location.
```
