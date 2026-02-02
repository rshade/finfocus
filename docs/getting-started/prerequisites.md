---
layout: default
title: Prerequisites
description: System requirements and dependencies for FinFocus
---

## System Requirements

### Minimum

- OS: Linux, macOS, or Windows
- Memory: 256 MB RAM
- Disk: 50 MB free space
- Network: Internet connection (for plugins)

### Recommended

- OS: Latest Linux distribution or macOS 12+
- Memory: 1 GB RAM
- Disk: 500 MB free space

## Required Software

### Pulumi CLI

FinFocus works with existing Pulumi projects.

```bash
# Check if installed
pulumi --version

# Install: https://www.pulumi.com/docs/install/
```

### Cloud Credentials

For projected costs, you don't need cloud credentials.

For actual costs (with plugins), you need:

- **AWS:** AWS access key or IAM role
- **Azure:** Azure credentials or service principal
- **GCP:** GCP service account

### Optional: Go 1.25.6+ (for building from source)

```bash
# Check if installed
go version

# Install: https://golang.org/doc/install
```

## Network Access

### Required Ports

- **Inbound:** None (CLI tool)
- **Outbound:**
  - HTTPS (443): For plugin communication
  - gRPC (default): For plugin protocols

### Firewall

If behind corporate firewall:

- Allow HTTPS outbound (443)
- Allow gRPC outbound (variable ports)
- Contact your IT team for exceptions

## Cloud Provider Access

### For Projected Costs

No credentials needed - works with Pulumi definitions.

### For Actual Costs

Requires plugin credentials:

#### Vantage

- API key from [vantage.sh](https://vantage.sh)
- Read-only access sufficient

#### Kubecost (future)

- Kubecost cluster access
- Metrics endpoint

## Verification Checklist

- [ ] Pulumi CLI installed (`pulumi --version`)
- [ ] Internet connection working
- [ ] Terminal/command line access
- [ ] Cloud credentials configured (if needed)

## Troubleshooting

### "pulumi command not found"

- Install Pulumi: [Pulumi install guide](https://www.pulumi.com/docs/install/)
- Add to PATH

### "Permission denied" on binary

```bash
chmod +x finfocus
```

### "No space left on device"

- Free up disk space
- Install to different location

---

[Continue to Installation](installation.md)
