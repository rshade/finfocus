---
layout: default
title: Vantage Plugin Setup Guide
description: Step-by-step installation and configuration guide for the FinFocus Vantage plugin
---

This guide walks you through installing and configuring the Vantage plugin for
FinFocus. Follow these steps to enable multi-cloud cost aggregation using
Vantage's cost visibility platform.

## Table of Contents

1. [Prerequisites](#prerequisites)
2. [Installation](#installation)
3. [Configuration](#configuration)
4. [Verification](#verification)
5. [Initial Sync](#initial-sync)
6. [Common Setup Issues](#common-setup-issues)

---

## Prerequisites

Before installing the Vantage plugin, ensure you have:

### Required

- **FinFocus Core** installed (v0.1.0 or later)
- **Vantage Account** with API access enabled
- **Vantage API Token** (see [Authentication Guide](authentication.md))
- **Cost Report Token** or **Workspace Token** from Vantage

### System Requirements

- Go 1.25.6 or later
- `make` (for building from source)
- Docker (optional, for running mock tests)

### Vantage Configuration

- At least one Cost Report configured in your Vantage workspace
- API access enabled for your account
- Cost data available for your cloud providers (AWS, Azure, GCP, etc.)

---

## Installation

### Option 1: Install Pre-Built Binary (Recommended)

Download the latest release for your platform:

```bash
# Linux (amd64)
curl -Lo finfocus-vantage https://github.com/rshade/finfocus-plugin-vantage/releases/latest/download/finfocus-vantage-linux-amd64
chmod +x finfocus-vantage
sudo mv finfocus-vantage /usr/local/bin/

# Linux (arm64)
curl -Lo finfocus-vantage https://github.com/rshade/finfocus-plugin-vantage/releases/latest/download/finfocus-vantage-linux-arm64
chmod +x finfocus-vantage
sudo mv finfocus-vantage /usr/local/bin/

# macOS (amd64)
curl -Lo finfocus-vantage https://github.com/rshade/finfocus-plugin-vantage/releases/latest/download/finfocus-vantage-darwin-amd64
chmod +x finfocus-vantage
sudo mv finfocus-vantage /usr/local/bin/
```

### Option 2: Install via FinFocus Plugin Manager

Install through FinFocus's plugin system:

```bash
# Install plugin (coming soon)
finfocus plugin install vantage

# List installed plugins
finfocus plugin list

# Verify installation
finfocus plugin validate vantage
```

### Option 3: Build from Source

For development or customization:

```bash
# Clone repository
git clone https://github.com/rshade/finfocus-plugin-vantage.git
cd finfocus-plugin-vantage

# Build binary
make build

# Binary created at: bin/finfocus-vantage
./bin/finfocus-vantage --version
```

### Verify Installation

Confirm the plugin is installed correctly:

```bash
# Check version
finfocus-vantage --version
# Expected output: finfocus-vantage version 0.1.0

# View help
finfocus-vantage --help
```

---

## Configuration

### Step 1: Create Configuration Directory

Create a directory for FinFocus configuration:

```bash
mkdir -p ~/.finfocus/plugins/vantage
cd ~/.finfocus/plugins/vantage
```

### Step 2: Create Configuration File

Create `config.yaml` with your Vantage settings:

```yaml
version: 0.1
source: vantage

credentials:
  token: ${FINFOCUS_VANTAGE_TOKEN}

params:
  # Use Cost Report Token (preferred) or Workspace Token
  cost_report_token: 'cr_your_report_token_here'

  # Date range (ISO 8601 format)
  start_date: '2024-01-01'

  # Granularity: "day" or "month"
  granularity: 'day'

  # Dimensions to group by
  group_bys:
    - provider
    - service
    - account
    - region

  # Metrics to include
  metrics:
    - cost
    - usage
    - effective_unit_price
```

### Step 3: Set Environment Variables

Configure authentication using environment variables:

```bash
# Set Vantage API token
export FINFOCUS_VANTAGE_TOKEN="your_vantage_api_token"

# Optional: Set specific tokens
export FINFOCUS_VANTAGE_COST_REPORT_TOKEN="cr_your_report_token"

# Persist in shell profile
echo 'export FINFOCUS_VANTAGE_TOKEN="your_token"' >> ~/.bashrc
source ~/.bashrc
```

**Security Best Practice:** Never hardcode tokens in configuration files.
Always use environment variables or secrets management systems.

### Step 4: Verify Configuration

Test your configuration file:

```bash
# Verify YAML syntax
yamllint config.yaml

# Test configuration loading
finfocus-vantage pull --config config.yaml --dry-run
```

---

## Verification

### Verify Plugin Registration

Check that FinFocus recognizes the plugin:

```bash
# List registered plugins
finfocus plugin list

# Expected output:
# NAME      VERSION  STATUS   LOCATION
# vantage   0.1.0    active   ~/.finfocus/plugins/vantage
```

### Verify API Connectivity

Test Vantage API connection:

```bash
# Test authentication
curl -H "Authorization: Bearer $FINFOCUS_VANTAGE_TOKEN" \
  https://api.vantage.sh/costs

# Expected: 200 OK or 400 (bad params), NOT 401 (auth failure)
```

### Verify Cost Data Access

Test cost data retrieval:

```bash
# Dry run to test configuration
finfocus-vantage pull --config config.yaml --dry-run

# Expected output:
# Configuration valid
# API connection successful
# Cost Report: cr_abc123def456
# Date range: 2024-01-01 to 2024-12-31
# Estimated records: ~15,000
```

---

## Initial Sync

### Perform Initial Backfill

Import historical cost data:

```bash
# Backfill last 12 months of cost data
finfocus-vantage backfill --config config.yaml --months 12

# Expected output:
# Fetching costs from 2024-01-01 to 2024-12-31...
# Progress: [====================] 100%
# Total records imported: 25,432
# Duration: 45s
# Bookmark saved: 2024-12-31
```

### Verify Data Import

Check imported data:

```bash
# Query projected costs using FinFocus CLI
finfocus cost projected \
  --plugin vantage \
  --provider aws \
  --start-date 2024-01-01 \
  --end-date 2024-01-31

# Query actual costs
finfocus cost actual \
  --plugin vantage \
  --start-date 2024-01-01 \
  --end-date 2024-01-31
```

### Setup Scheduled Sync

Configure daily incremental sync:

**Using Cron:**

```bash
# Add to crontab (daily at 2 AM UTC)
crontab -e

# Add this line:
0 2 * * * /usr/local/bin/finfocus-vantage pull --config ~/.finfocus/plugins/vantage/config.yaml
```

**Using systemd Timer:**

Create `/etc/systemd/system/finfocus-vantage.service`:

```ini
[Unit]
Description=FinFocus Vantage Daily Sync
After=network.target

[Service]
Type=oneshot
User=finfocus
Environment="FINFOCUS_VANTAGE_TOKEN=your_token"
ExecStart=/usr/local/bin/finfocus-vantage pull --config /etc/finfocus/config.yaml
```

Create `/etc/systemd/system/finfocus-vantage.timer`:

```ini
[Unit]
Description=Run FinFocus Vantage Sync Daily

[Timer]
OnCalendar=daily
OnCalendar=02:00
Persistent=true

[Install]
WantedBy=timers.target
```

Enable and start the timer:

```bash
sudo systemctl daemon-reload
sudo systemctl enable finfocus-vantage.timer
sudo systemctl start finfocus-vantage.timer

# Check timer status
systemctl status finfocus-vantage.timer
```

---

## Common Setup Issues

### Issue: Plugin Not Found

**Symptoms:**

```text
Error: plugin 'vantage' not found
```

**Solutions:**

1. Verify installation path:

   ```bash
   ls -la ~/.finfocus/plugins/vantage/
   ```

2. Check plugin registry:

   ```bash
   finfocus plugin list
   ```

3. Reinstall plugin:

   ```bash
   finfocus plugin install vantage
   ```

### Issue: Authentication Failed

**Symptoms:**

```text
Error: 401 Unauthorized
```

**Solutions:**

1. Verify token is set:

   ```bash
   echo $FINFOCUS_VANTAGE_TOKEN
   # Should output your token (not empty)
   ```

2. Test token validity:

   ```bash
   curl -H "Authorization: Bearer $FINFOCUS_VANTAGE_TOKEN" \
     https://api.vantage.sh/costs
   ```

3. Regenerate token in Vantage console

See [Authentication Guide](authentication.md) for detailed troubleshooting.

### Issue: Configuration Parse Error

**Symptoms:**

```text
Error: failed to parse config.yaml
```

**Solutions:**

1. Validate YAML syntax:

   ```bash
   yamllint config.yaml
   ```

2. Check required fields:

   ```yaml
   version: 0.1 # Required
   source: vantage # Required
   credentials:
     token: ${...} # Required
   params:
     cost_report_token: 'cr_...' # Required
     granularity: 'day' # Required
   ```

### Issue: No Cost Data Returned

**Symptoms:**

```text
No cost records found
```

**Solutions:**

1. Verify date range has data in Vantage console
2. Check Cost Report Token is valid:

   ```yaml
   params:
     cost_report_token: 'cr_valid_token_here'
   ```

3. Ensure Cost Report has data for selected providers
4. Account for late posting (data lags 2-3 days)

---

## Next Steps

After successful setup:

1. **Configure Authentication** - See [Authentication Guide](authentication.md)
   for security best practices
2. **Explore Features** - Review [Features Guide](features.md) for supported
   capabilities
3. **Understand Cost Mapping** - Read [Cost Mapping Guide](cost-mapping.md) to
   understand data transformation
4. **Troubleshoot Issues** - Consult [Troubleshooting Guide](troubleshooting.md)
   for common problems

---

## Additional Resources

- [Vantage Plugin README](https://github.com/rshade/finfocus-plugin-vantage)
- [Vantage API Documentation](https://docs.vantage.sh/api)
- [FinFocus Plugin Development Guide](../plugin-development.md)
- [FinFocus Plugin SDK Reference](../plugin-sdk.md)
