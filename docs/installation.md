# Installation Guide

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
- **Memory**: Minimum 256MB RAM
- **Storage**: 50MB for binary, additional space for plugins and specs

### Software Dependencies

- **Pulumi CLI**: Required for generating plan JSON files
  - Install from: https://www.pulumi.com/docs/get-started/install/
  - Minimum version: 3.0.0

## Installation Methods

### 1. Download Pre-built Binaries (Recommended)

Download the latest release for your platform:

#### Linux (x86_64)

```bash
curl -L https://github.com/rshade/finfocus/releases/latest/download/finfocus-linux-amd64 -o finfocus
chmod +x finfocus
sudo mv finfocus /usr/local/bin/
```

#### Linux (ARM64)

```bash
curl -L https://github.com/rshade/finfocus/releases/latest/download/finfocus-linux-arm64 -o finfocus
chmod +x finfocus
sudo mv finfocus /usr/local/bin/
```

#### macOS (Intel)

```bash
curl -L https://github.com/rshade/finfocus/releases/latest/download/finfocus-darwin-amd64 -o finfocus
chmod +x finfocus
sudo mv finfocus /usr/local/bin/
```

#### macOS (Apple Silicon)

```bash
curl -L https://github.com/rshade/finfocus/releases/latest/download/finfocus-darwin-arm64 -o finfocus
chmod +x finfocus
sudo mv finfocus /usr/local/bin/
```

#### Windows (x86_64)

```powershell
# Download using PowerShell
Invoke-WebRequest -Uri "https://github.com/rshade/finfocus/releases/latest/download/finfocus-windows-amd64.exe" -OutFile "finfocus.exe"

# Add to PATH (optional)
Move-Item finfocus.exe $env:USERPROFILE\bin\
$env:PATH += ";$env:USERPROFILE\bin"
```

### 2. Package Managers

#### Homebrew (macOS/Linux)

```bash
# Coming soon - not yet available
brew install finfocus
```

#### Chocolatey (Windows)

```powershell
# Coming soon - not yet available
choco install finfocus
```

#### APT (Ubuntu/Debian)

```bash
# Coming soon - not yet available
sudo apt update
sudo apt install finfocus
```

## Platform-Specific Instructions

### Linux

#### Ubuntu/Debian

```bash
# Install dependencies
sudo apt update
sudo apt install curl

# Download and install
curl -L https://github.com/rshade/finfocus/releases/latest/download/finfocus-linux-amd64 -o finfocus
chmod +x finfocus
sudo mv finfocus /usr/local/bin/

# Verify installation
finfocus --version
```

#### CentOS/RHEL/Fedora

```bash
# Install dependencies
sudo yum install curl  # or dnf install curl

# Download and install
curl -L https://github.com/rshade/finfocus/releases/latest/download/finfocus-linux-amd64 -o finfocus
chmod +x finfocus
sudo mv finfocus /usr/local/bin/

# Verify installation
finfocus --version
```

#### Alpine Linux

```bash
# Install dependencies
apk add curl

# Download and install
curl -L https://github.com/rshade/finfocus/releases/latest/download/finfocus-linux-amd64 -o finfocus
chmod +x finfocus
mv finfocus /usr/local/bin/

# Verify installation
finfocus --version
```

### macOS

#### Using curl (Recommended)

```bash
# Download for your architecture
# Intel Macs
curl -L https://github.com/rshade/finfocus/releases/latest/download/finfocus-darwin-amd64 -o finfocus

# Apple Silicon Macs
curl -L https://github.com/rshade/finfocus/releases/latest/download/finfocus-darwin-arm64 -o finfocus

# Install
chmod +x finfocus
sudo mv finfocus /usr/local/bin/

# Verify installation
finfocus --version
```

#### Bypass Gatekeeper (if needed)

```bash
# If macOS blocks the binary due to security settings
sudo spctl --add /usr/local/bin/finfocus
sudo xattr -dr com.apple.quarantine /usr/local/bin/finfocus
```

### Windows

#### PowerShell Installation

```powershell
# Create bin directory
New-Item -ItemType Directory -Force -Path $env:USERPROFILE\bin

# Download binary
Invoke-WebRequest -Uri "https://github.com/rshade/finfocus/releases/latest/download/finfocus-windows-amd64.exe" -OutFile "$env:USERPROFILE\bin\finfocus.exe"

# Add to PATH (persistent)
[Environment]::SetEnvironmentVariable("PATH", $env:PATH + ";$env:USERPROFILE\bin", [System.EnvironmentVariableTarget]::User)

# Refresh current session
$env:PATH += ";$env:USERPROFILE\bin"

# Verify installation
finfocus --version
```

#### Windows Subsystem for Linux (WSL)

```bash
# Use Linux installation instructions inside WSL
curl -L https://github.com/rshade/finfocus/releases/latest/download/finfocus-linux-amd64 -o finfocus
chmod +x finfocus
sudo mv finfocus /usr/local/bin/
```

## Docker Installation

### Using Official Docker Image

```bash
# Pull the latest image
docker pull rshade/finfocus:latest

# Run with volume mounts
docker run --rm -v $(pwd):/workspace rshade/finfocus:latest \
  cost projected --pulumi-json /workspace/plan.json

# Create an alias for easier usage
alias finfocus='docker run --rm -v $(pwd):/workspace rshade/finfocus:latest'
```

### Docker Compose

```yaml
# docker-compose.yml
version: '3.8'
services:
  finfocus:
    image: rshade/finfocus:latest
    volumes:
      - ./plans:/workspace/plans:ro
      - ./specs:/workspace/specs:ro
      - ~/.finfocus:/root/.finfocus
    command: ['cost', 'projected', '--pulumi-json', '/workspace/plans/plan.json']
```

### Building Docker Image

```bash
# Clone repository
git clone https://github.com/rshade/finfocus
cd finfocus

# Build image
docker build -t finfocus .

# Run
docker run --rm finfocus --version
```

## Build from Source

### Prerequisites for Building

- **Go**: Version 1.25.6 or later
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

#### Kubecost Plugin Example

```bash
# Download plugin binary (example)
curl -L https://github.com/rshade/finfocus-plugin-kubecost/releases/latest/download/finfocus-kubecost-linux-amd64 \
  -o ~/.finfocus/plugins/kubecost/1.0.0/finfocus-kubecost

# Make executable
chmod +x ~/.finfocus/plugins/kubecost/1.0.0/finfocus-kubecost

# Create plugin manifest (optional)
cat > ~/.finfocus/plugins/kubecost/1.0.0/plugin.manifest.json << EOF
{
  "name": "kubecost",
  "version": "1.0.0",
  "binary": "finfocus-kubecost",
  "supports": ["actual_cost"]
}
EOF
```

#### Plugin Directory Structure

```text
~/.finfocus/plugins/
├── kubecost/
│   └── 1.0.0/
│       ├── finfocus-kubecost          # Plugin binary
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
4. Check out the [Examples](../examples/) directory

## Version Management

### Updating FinFocus

```bash
# Check current version
finfocus --version

# Download and replace binary with new version
curl -L https://github.com/rshade/finfocus/releases/latest/download/finfocus-linux-amd64 -o /tmp/finfocus
chmod +x /tmp/finfocus
sudo mv /tmp/finfocus /usr/local/bin/finfocus

# Verify update
finfocus --version
```

### Multiple Versions

```bash
# Install specific version
curl -L https://github.com/rshade/finfocus/releases/download/v0.1.0/finfocus-linux-amd64 -o finfocus-v0.1.0
chmod +x finfocus-v0.1.0

# Use specific version
./finfocus-v0.1.0 --version
```
