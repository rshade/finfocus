---
layout: default
title: Troubleshooting Guide
description: Common issues and solutions
---

This guide helps you diagnose and resolve common issues with FinFocus. Each
section covers a specific category of problems with step-by-step solutions.

## Table of Contents

- [Installation Issues](#installation-issues)
- [Cost Calculation Issues](#cost-calculation-issues)
- [Plugin Issues](#plugin-issues)
- [Performance Issues](#performance-issues)
- [File Issues](#file-issues)
- [Configuration Issues](#configuration-issues)
- [Getting More Help](#getting-more-help)

## Installation Issues

### "finfocus: command not found"

Solution:

```bash
# Build from source
make build

# Add to PATH
export PATH="$PWD/bin:$PATH"

# Or verify installation
ls -la bin/finfocus
```

### "make: command not found"

Solution:

```bash
# Install make
# macOS
brew install make

# Ubuntu/Debian
sudo apt-get install make

# Check
make --version
```

## Cost Calculation Issues

### "No cost data available"

Causes:

1. Plugin not installed
2. Plugin not configured
3. Resource type not supported
4. API credentials invalid

Solution:

```bash
# Check plugins
finfocus plugin list
finfocus plugin validate

# See plugin documentation
# docs/plugins/vantage/setup.md
```

### "Invalid date format"

Solution:

```bash
# Use YYYY-MM-DD
finfocus cost actual --from 2024-01-01

# Or RFC3339
finfocus cost actual --from 2024-01-01T00:00:00Z
```

### "Filter not working"

Solution:

```bash
# Check filter syntax
finfocus cost actual --filter "tag:env=prod"

# Multiple conditions
finfocus cost actual --filter "tag:env=prod AND tag:team=platform"
```

## Plugin Issues

### "Plugin validation failed"

```bash
# Debug plugin
finfocus plugin validate

# Check plugin binary
ls -la ~/.finfocus/plugins/*/*/
chmod +x ~/.finfocus/plugins/*/*/finfocus-*
```

### "Plugin timeout"

Solution:

1. Check network connectivity
2. Verify API credentials
3. Check plugin logs
4. Increase timeout (if available)

### "API authentication failed"

Solution:

```bash
# Verify credentials
echo $VANTAGE_API_KEY  # For Vantage

# Or check config
cat ~/.finfocus/config.yaml
```

## Performance Issues

### "CLI is slow"

Solution:

```bash
# Filter to smaller dataset
finfocus cost actual --from 2024-01-31 --to 2024-01-31

# Use NDJSON for large output
finfocus cost actual --output ndjson
```

### "Plugin is timing out"

Solution:

1. Check plugin is running
2. Verify network connectivity
3. Check plugin logs
4. Try again (may be API issue)

## File Issues

### "Pulumi JSON file not found"

Solution:

```bash
# Generate plan
pulumi preview --json > plan.json

# Verify
cat plan.json | head
```

### "Permission denied"

Solution:

```bash
# Fix permissions
chmod +x ~/.finfocus/plugins/*/*/finfocus-*

# Or rebuild
make build
chmod +x bin/finfocus
```

## Configuration Issues

### "Configuration not found"

Solution:

```bash
# Vantage requires config
# See: docs/plugins/vantage/setup.md

# Or use local specs
mkdir -p ~/.finfocus/specs
# Add YAML spec files
```

### "Default currency error"

Solution:

```bash
# Ensure specs have currency
cat ~/.finfocus/specs/*.yaml | grep -i currency

# See spec documentation
# docs/deployment/configuration.md
```

## Getting More Help

1. **Check logs:**

   ```bash
   finfocus cost actual --debug
   ```

2. **Read relevant guide:**
   - [User Guide](../guides/user-guide.md)
   - [Plugin Documentation](../plugins/)
   - [Configuration Guide](../deployment/configuration.md)

3. **Report issue:**
   - [GitHub Issues](https://github.com/rshade/finfocus/issues)
