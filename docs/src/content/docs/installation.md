---
title: Installation Guide
description: Complete installation instructions for FinFocus across platforms and deployment scenarios.
---
Complete installation instructions for FinFocus Core across different platforms and deployment scenarios.

## Table of Contents

- [Prerequisites](#prerequisites)
- [Installation Methods](#installation-methods)
- [Platform-Specific Instructions](#platform-specific-instructions)
- [Docker Installation](#docker-installation)
- [Build from Source](#build-from-source)
- [Installing as a Pulumi Tool Plugin](#installing-as-a-pulumi-tool-plugin)
- [Plugin Installation](#plugin-installation)
- [Verification](#verification)
- [Troubleshooting](#troubleshooting)

## Prerequisites

### System Requirements

- **Operating System**: Linux, macOS, or Windows
- **Architecture**: x86_64 (amd64) or ARM64
- **Memory**: Minimum 256 MB RAM
- **Storage**: 50 MB for the binary. Plugins and specs need more.

### Software Dependencies

- **Pulumi CLI**: Required for generating plan JSON files
  - Install from: https://www.pulumi.com/docs/get-started/install/
  - Minimum version: 3.0.0

## Installation Methods

### 1. Install Script (Recommended)

The install script picks the release archive for your OS (Linux or macOS) and
architecture (amd64 or arm64) and checks it against the release's
`checksums.txt` when the release has one:

```bash
curl -fsSL https://raw.githubusercontent.com/rshade/finfocus/main/scripts/install.sh | sh
```

The script installs to `/usr/local/bin` when it can write there and to
`~/.local/bin` otherwise. That directory is often not on `PATH`, especially
on macOS. The script prints the `export PATH=...` line to add.

`FINFOCUS_VERSION` pins a release and `FINFOCUS_INSTALL_DIR` changes the
install directory; [Quick Install](getting-started/installation.md#option-2-install-script-recommended)
lists the options. On Windows, use a release archive instead.

### 2. Download a Release Archive

Each [release](https://github.com/rshade/finfocus/releases) publishes
versioned archives named `finfocus-<version>-<os>-<arch>`: `.tar.gz` for
`linux` and `macos`, `.zip` for `windows`, each on `amd64` and `arm64`, plus
`checksums.txt`. There is no unversioned asset such as `finfocus-linux-amd64`.

#### Linux and macOS

```bash
# Set the release, OS (linux or macos), and architecture (amd64 or arm64)
VERSION=v0.5.8
OS=linux
ARCH=amd64
ARCHIVE="finfocus-${VERSION}-${OS}-${ARCH}.tar.gz"

curl -fLO "https://github.com/rshade/finfocus/releases/download/${VERSION}/${ARCHIVE}"
curl -fLO "https://github.com/rshade/finfocus/releases/download/${VERSION}/checksums.txt"

# Verify the archive (on macOS use: shasum -a 256 -c -)
grep " ${ARCHIVE}\$" checksums.txt | sha256sum -c -

tar -xzf "${ARCHIVE}" finfocus
sudo mv finfocus /usr/local/bin/
```

#### Windows

```powershell
# Set the release and architecture (amd64 or arm64)
$Version = "v0.5.8"
$Arch = "amd64"
$Zip = "finfocus-$Version-windows-$Arch.zip"

$Base = "https://github.com/rshade/finfocus/releases/download/$Version"

Invoke-WebRequest -Uri "$Base/$Zip" -OutFile $Zip
Invoke-WebRequest -Uri "$Base/checksums.txt" -OutFile checksums.txt

# Verify the archive against checksums.txt
$Expected = ((Get-Content checksums.txt | Where-Object { $_ -like "*  $Zip" }) -split '\s+')[0]
if ((Get-FileHash $Zip -Algorithm SHA256).Hash -ne $Expected) { throw "Checksum mismatch for $Zip" }

Expand-Archive $Zip -DestinationPath "finfocus-$Version" -Force

# Move the binary into a bin directory on PATH
New-Item -ItemType Directory -Force -Path $env:USERPROFILE\bin
Move-Item "finfocus-$Version\finfocus.exe" $env:USERPROFILE\bin\ -Force
$env:PATH += ";$env:USERPROFILE\bin"
```

Replace `v0.5.8` with the release you want.

### 3. Package Managers

FinFocus is not published to Homebrew, Chocolatey, or APT. Use the install
script or a release archive.

## Platform-Specific Instructions

### Linux

#### Ubuntu/Debian

```bash
# Install dependencies
sudo apt update
sudo apt install curl tar

# Download and install
curl -fsSL https://raw.githubusercontent.com/rshade/finfocus/main/scripts/install.sh | sh

# Verify installation
finfocus --version
```

#### CentOS/RHEL/Fedora

```bash
# Install dependencies
sudo yum install curl tar  # or dnf install curl tar

# Download and install
curl -fsSL https://raw.githubusercontent.com/rshade/finfocus/main/scripts/install.sh | sh

# Verify installation
finfocus --version
```

#### Alpine Linux

```bash
# Install dependencies
apk add curl tar

# Download and install (the release binary is statically linked)
curl -fsSL https://raw.githubusercontent.com/rshade/finfocus/main/scripts/install.sh | sh

# Verify installation
finfocus --version
```

### macOS

#### Using the Install Script (Recommended)

The script picks `macos-amd64` on Intel Macs and `macos-arm64` on Apple
Silicon:

```bash
curl -fsSL https://raw.githubusercontent.com/rshade/finfocus/main/scripts/install.sh | sh

# Verify installation
finfocus --version
```

#### Bypass Gatekeeper (if needed)

```bash
# If macOS blocks the binary due to security settings, use the path the
# install script printed (/usr/local/bin/finfocus or ~/.local/bin/finfocus)
sudo spctl --add "$(command -v finfocus)"
sudo xattr -dr com.apple.quarantine "$(command -v finfocus)"
```

### Windows

#### PowerShell Installation

```powershell
# Create bin directory
New-Item -ItemType Directory -Force -Path $env:USERPROFILE\bin

# Download, verify, and unpack a release (replace the version as needed)
$Version = "v0.5.8"
$Zip = "finfocus-$Version-windows-amd64.zip"
$Base = "https://github.com/rshade/finfocus/releases/download/$Version"
Invoke-WebRequest -Uri "$Base/$Zip" -OutFile $Zip
Invoke-WebRequest -Uri "$Base/checksums.txt" -OutFile checksums.txt
$Expected = ((Get-Content checksums.txt | Where-Object { $_ -like "*  $Zip" }) -split '\s+')[0]
if ((Get-FileHash $Zip -Algorithm SHA256).Hash -ne $Expected) { throw "Checksum mismatch for $Zip" }
Expand-Archive $Zip -DestinationPath "finfocus-$Version" -Force
Move-Item "finfocus-$Version\finfocus.exe" "$env:USERPROFILE\bin\" -Force

# Add to PATH (persistent)
[Environment]::SetEnvironmentVariable("PATH", [Environment]::GetEnvironmentVariable("PATH", "User") + ";$env:USERPROFILE\bin", "User")

# Refresh current session
$env:PATH += ";$env:USERPROFILE\bin"

# Verify installation
finfocus --version
```

#### Windows Subsystem for Linux (WSL)

```bash
# Use Linux installation instructions inside WSL
curl -fsSL https://raw.githubusercontent.com/rshade/finfocus/main/scripts/install.sh | sh
```

## Docker Installation

### Using Official Docker Image

The image is published to the GitHub Container Registry, not Docker Hub. See
[Docker Deployment](deployment/docker.md) for tags and details.

```bash
# Pull the most recent release
docker pull ghcr.io/rshade/finfocus:latest

# Run with volume mounts
docker run --rm -v $(pwd):/workspace ghcr.io/rshade/finfocus:latest \
  cost projected --pulumi-json /workspace/plan.json

# Create an alias for easier usage
alias finfocus='docker run --rm -v $(pwd):/workspace ghcr.io/rshade/finfocus:latest'
```

### Docker Compose

```yaml
# docker-compose.yml
version: '3.8'
services:
  finfocus:
    image: ghcr.io/rshade/finfocus:latest
    volumes:
      - ./plans:/workspace/plans:ro
      - ./specs:/workspace/specs:ro
      # The image runs as user finfocus with HOME=/home/finfocus
      - ~/.finfocus:/home/finfocus/.finfocus
    command: ['cost', 'projected', '--pulumi-json', '/workspace/plans/plan.json']
```

### Building Docker Image

```bash
# Clone repository
git clone https://github.com/rshade/finfocus
cd finfocus

# Build image (the Dockerfile is in docker/; the context is the repository root)
docker build -f docker/Dockerfile -t finfocus .

# Run
docker run --rm finfocus --version
```

## Build from Source

### Prerequisites for Building

- **Go**: Version 1.27.1 or later
  - Install from: https://golang.org/dl/
- **Git**: For cloning the repository
- **Make**: For using build scripts (optional)

### Build Steps

```bash
# Clone the repository
git clone https://github.com/rshade/finfocus
cd finfocus

# Build using Make (recommended)
make build

# Or build directly with Go
go build -o bin/finfocus ./cmd/finfocus

# Install to system
sudo cp bin/finfocus /usr/local/bin/

# Verify installation
finfocus --version
```

### Development Build

```bash
# Clone and setup for development
git clone https://github.com/rshade/finfocus
cd finfocus

# Install dependencies
go mod download

# Run tests
make test

# Build and run
make dev

# Or run directly
go run ./cmd/finfocus --help
```

### Cross-compilation

```bash
# Build for different platforms
GOOS=linux GOARCH=amd64 go build -o finfocus-linux-amd64 ./cmd/finfocus
GOOS=darwin GOARCH=arm64 go build -o finfocus-darwin-arm64 ./cmd/finfocus
GOOS=windows GOARCH=amd64 go build -o finfocus-windows-amd64.exe ./cmd/finfocus
```

## Installing as a Pulumi Tool Plugin

FinFocus can be installed as a Pulumi Tool Plugin, allowing you to run it through the Pulumi CLI.

### Build and Install

```bash
# Clone the repository
git clone https://github.com/rshade/finfocus
cd finfocus

# Build with the plugin binary name
make build-plugin
# Or manually: go build -o pulumi-tool-cost ./cmd/finfocus

# Install into Pulumi plugins directory
mkdir -p ~/.pulumi/plugins/tool-cost-v0.1.0/
cp pulumi-tool-cost ~/.pulumi/plugins/tool-cost-v0.1.0/

# Verify installation
pulumi plugin ls
```

### Usage as Plugin

Once installed, run FinFocus through the Pulumi CLI:

```bash
# Show help
pulumi plugin run tool cost -- --help

# Calculate projected costs
pulumi plugin run tool cost -- cost projected --pulumi-json plan.json

# Get actual costs
pulumi plugin run tool cost -- cost actual --pulumi-json plan.json --from 2025-01-01
```

### Configuration Behavior

When running as a Pulumi plugin:

- Configuration is stored in `$PULUMI_HOME/finfocus/` instead of `~/.finfocus/`
- Help text automatically shows `pulumi plugin run tool cost` syntax
- The `FINFOCUS_PLUGIN_MODE=true` environment variable can force plugin mode

### Testing Plugin Installation

```bash
# Verify plugin is recognized
pulumi plugin ls

# Expected output:
# NAME  TYPE  VERSION    SIZE   INSTALLED   LAST USED
# cost  tool  v0.1.0-dev 15 MB  just now    just now

# Test with a Pulumi plan
cd your-pulumi-project
pulumi preview --json > plan.json
pulumi plugin run tool cost -- cost projected --pulumi-json plan.json
```

## Plugin Installation

### Plugin Directory Setup

Create the plugin directory structure:

```bash
# Create plugin directory
mkdir -p ~/.finfocus/plugins

# Create specs directory
mkdir -p ~/.finfocus/specs
```

### Installing Plugins

#### OpenCost Plugin Example

```bash
# Install from the registry (verifies the release checksum)
finfocus plugin install opencost
```

To install by hand, download the archive for your platform from the
[plugin releases](https://github.com/rshade/finfocus-plugin-opencost/releases),
extract it into `~/.finfocus/plugins/opencost/<version>/`, and make the binary
executable. The archive includes `plugin.manifest.json`.

#### Plugin Directory Structure

```text
~/.finfocus/plugins/
├── opencost/
│   └── 0.1.2/
│       ├── finfocus-plugin-opencost   # Plugin binary
│       └── plugin.manifest.json        # Optional manifest
├── aws-plugin/
│   └── 0.1.0/
│       └── finfocus-aws
└── azure-plugin/
    └── 0.2.0/
        └── finfocus-azure
```

### Installing Pricing Specs

Create local pricing specifications:

```bash
# Example AWS EC2 pricing spec
cat > ~/.finfocus/specs/aws-ec2-t3-micro.yaml << EOF
provider: aws
service: ec2
sku: t3.micro
currency: USD
pricing:
  instanceType: t3.micro
  onDemandHourly: 0.0104
  monthlyEstimate: 7.59
  vcpu: 2
  memory: 1
metadata:
  region: us-west-2
  operatingSystem: linux
EOF
```

## Verification

### Basic Verification

```bash
# Check version
finfocus --version

# Show help
finfocus --help

# List available commands
finfocus help
```

### Plugin Verification

```bash
# List installed plugins
finfocus plugin list

# Validate plugin installation
finfocus plugin validate
```

### Test with Example

```bash
# Test with provided example
cd finfocus  # if you cloned the repo
finfocus cost projected --pulumi-json examples/plans/aws-simple-plan.json

# Expected output should show resource cost estimates
```

### Integration Test

```bash
# Test complete workflow
cd your-pulumi-project
pulumi preview --json > plan.json
finfocus cost projected --pulumi-json plan.json

# Should display cost estimates for your resources
```

## Troubleshooting

### Common Issues

#### Binary Not Found

```bash
# Check if binary is in PATH
which finfocus

# If not found, add directory to PATH
export PATH=$PATH:/usr/local/bin

# Or use full path
/usr/local/bin/finfocus --version
```

#### Permission Denied (Linux/macOS)

```bash
# Make binary executable
chmod +x /usr/local/bin/finfocus

# Check file permissions
ls -la /usr/local/bin/finfocus
```

#### macOS Security Warning

```bash
# Allow the binary to run
sudo spctl --add /usr/local/bin/finfocus
sudo xattr -dr com.apple.quarantine /usr/local/bin/finfocus
```

#### Plugin Not Found

```bash
# Check plugin directory structure
ls -la ~/.finfocus/plugins/

# Verify plugin permissions
chmod +x ~/.finfocus/plugins/*/*/finfocus-*
```

#### Network Issues

```bash
# Use proxy if needed
export HTTP_PROXY=http://proxy.company.com:8080
export HTTPS_PROXY=http://proxy.company.com:8080

# Or download manually and transfer
# Download on another machine and copy via SCP/USB
```

### Getting Help

If you encounter issues:

1. Check the [Troubleshooting Guide](troubleshooting.md)
2. Review the [GitHub Issues](https://github.com/rshade/finfocus/issues)
3. Join the community discussion
4. File a bug report with:
   - Operating system and version
   - FinFocus version (`finfocus --version`)
   - Complete error message
   - Steps to reproduce

## Next Steps

After installation:

1. Read the [User Guide](user-guide.md) for detailed usage instructions
2. Review [Cost Calculations](cost-calculations.md) to understand methodologies
3. Set up [Plugin System](plugin-system.md) for actual cost tracking
4. Check out the [Examples](https://github.com/rshade/finfocus/tree/main/examples) directory

## Version Management

### Updating FinFocus

```bash
# Check current version
finfocus --version

# Re-run the install script; it installs the newest release
curl -fsSL https://raw.githubusercontent.com/rshade/finfocus/main/scripts/install.sh | sh

# Verify update
finfocus --version
```

### Multiple Versions

```bash
# Install specific version
curl -fL https://github.com/rshade/finfocus/releases/download/v0.3.0/finfocus-v0.3.0-linux-amd64.tar.gz -o finfocus-v0.3.0.tar.gz
tar -xzf finfocus-v0.3.0.tar.gz finfocus

# Use specific version
./finfocus --version
```
