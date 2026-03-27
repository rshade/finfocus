---
name: finfocus-install
description: >
  Automated FinFocus CLI and plugin setup workflow. Use when installing finfocus,
  setting up plugins for cloud providers, initializing configuration, bootstrapping
  a new finfocus environment, or onboarding a project for cost analysis. Triggers on:
  "install finfocus", "setup finfocus", "configure cost analysis", "install plugins",
  "bootstrap finfocus", "finfocus setup", "add cost tracking", "onboard project",
  or any task involving initial finfocus installation and environment configuration.
---
<!-- Copyright 2025-2026 Richard Shade. Licensed under Apache-2.0. -->
<!-- SPDX-License-Identifier: Apache-2.0 -->

# FinFocus Installation

## Quick Path: Existing Install

If finfocus is already installed, prefer the built-in setup command:

```bash
finfocus setup                    # Full bootstrap (idempotent)
finfocus setup --skip-analyzer    # Without Pulumi analyzer
finfocus setup --skip-plugins     # Without plugins (offline)
finfocus setup --non-interactive  # CI/CD mode
```

Setup runs: version check, Pulumi detection, directory creation, config init,
analyzer install, and default plugin (`aws-public`) install.

## Full Workflow

### Step 1: Check Existing Installation

```bash
finfocus --version
```

If installed, skip to Step 3. If not, continue.

### Step 2: Install the CLI Binary

**Install script (Linux/macOS):**

```bash
curl -fsSL https://raw.githubusercontent.com/rshade/finfocus/main/scripts/install.sh | sh
```

| Env Var | Purpose |
|---------|---------|
| `FINFOCUS_VERSION` | Pin specific version (default: latest) |
| `FINFOCUS_INSTALL_DIR` | Custom install dir (default: `/usr/local/bin` or `~/.local/bin`) |
| `FINFOCUS_NO_VERIFY=1` | Skip SHA256 checksum verification |

**Go install (fallback/Windows):**

```bash
go install github.com/rshade/finfocus/cmd/finfocus@latest
```

**Source build (last resort):**

```bash
git clone https://github.com/rshade/finfocus.git && cd finfocus && make build
```

Verify: `finfocus --version`

### Step 3: Detect Cloud Providers

Determine which plugins to install:

1. Check for `Pulumi.yaml`/`Pulumi.yml` — scan resource type prefixes (`aws:`, `kubernetes:`)
2. Check Pulumi state (`pulumi stack export`) for resource types
3. Check filesystem: `~/.aws/`, `~/.config/gcloud/`, `~/.azure/`, kubeconfig
4. Default to `aws-public` if nothing detected

Provider-to-plugin mapping — see
[references/registry-plugins.md](references/registry-plugins.md).

### Step 4: Install Plugins

```bash
finfocus plugin install aws-public
finfocus plugin install kubecost
```

Key flags — see [references/install-commands.md](references/install-commands.md)
for the complete flag reference.

```bash
# Region-specific AWS pricing
finfocus plugin install aws-public --metadata="region=us-east-1"

# CI: auto-fallback if version lacks platform assets
finfocus plugin install aws-public --fallback-to-latest

# Reinstall + clean old versions
finfocus plugin install aws-public --force --clean
```

### Step 5: Initialize Configuration

```bash
# Inside a Pulumi project → project-local config
finfocus config init
# Creates: $PROJECT/.finfocus/config.yaml + .gitignore

# Global config
finfocus config init --global
# Creates: ~/.finfocus/config.yaml
```

Config precedence: project-local > global > env vars > defaults.

### Step 6: Install Pulumi Analyzer (Optional)

For inline cost estimates during `pulumi preview`:

```bash
finfocus analyzer install
finfocus analyzer check           # Verify setup
```

Then run: `pulumi preview --policy-pack ~/.finfocus/analyzer`

Add to PATH for discovery: `export PATH="$HOME/.finfocus/analyzer:$PATH"`

### Step 7: Validate

```bash
finfocus plugin validate          # Check all plugin binaries
finfocus plugin list --verbose    # Confirm installed plugins
finfocus plugin list --available  # Show registry plugins
```

## Directory Structure

```text
~/.finfocus/
├── config.yaml          # Global configuration
├── plugins/             # Plugin binaries
│   ├── aws-public/<version>/finfocus-plugin-aws-public
│   └── kubecost/<version>/finfocus-plugin-kubecost
├── cache/               # BoltDB cache (cache.db)
├── logs/                # Log files
└── analyzer/            # Pulumi policy pack
```

## References

- [references/install-commands.md](references/install-commands.md) — Complete CLI
  flag reference for install, setup, config, and plugin commands
- [references/registry-plugins.md](references/registry-plugins.md) — Available
  plugins, provider mappings, and capability matrix
