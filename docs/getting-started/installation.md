---
layout: default
title: Installation Guide
description: Detailed installation instructions for FinFocus
---

Detailed steps to install FinFocus on your system.

## Prerequisites

- Pulumi CLI installed
- Go 1.25.7+ (if building from source)
- Git (if building from source)
- 5-10 minutes

## Installation Methods

### Option 1: Build from Source (Recommended)

#### Step 1: Clone the repository

```bash
git clone https://github.com/rshade/finfocus
cd finfocus
```

#### Step 2: Build

```bash
make build
```

Binary will be created at: `bin/finfocus`

#### Step 3: Add to PATH (optional)

```bash
# macOS/Linux
export PATH="$PWD/bin:$PATH"

# Or copy to system path
sudo cp bin/finfocus /usr/local/bin/
```

#### Step 4: Verify

```bash
finfocus --version
finfocus --help
```

### Option 2: Download Prebuilt Binary (Coming Soon)

Prebuilt binaries for Linux, macOS, and Windows will be available in GitHub releases.

```bash
# Download
curl -L https://github.com/rshade/finfocus/releases/latest/download/finfocus-darwin-arm64 -o finfocus

# Make executable
chmod +x finfocus

# Move to PATH
sudo mv finfocus /usr/local/bin/
```

### Option 3: Docker

```bash
docker run ghcr.io/rshade/finfocus:latest cost projected --help
```

## Verification

```bash
# Check version
finfocus --version

# Test with example plan
finfocus cost projected --pulumi-json examples/plans/aws-simple-plan.json
```

## Next Steps

- [Quick Start Guide](quickstart.md)
- [User Guide](../guides/user-guide.md)
- [Plugin Setup](../plugins/vantage/setup.md)
