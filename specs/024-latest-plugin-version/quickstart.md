# Quickstart: Verify Plugin Version Selection

## Prerequisites

- FinFocus Core installed or built locally.
- Access to `~/.finfocus/plugins` directory.

## Verification Steps

### 1. Setup Test Plugins

Create a dummy plugin with multiple versions.

```bash
# Create directories
mkdir -p ~/.finfocus/plugins/test-plugin/v1.0.0
mkdir -p ~/.finfocus/plugins/test-plugin/v2.0.0

# Create dummy binaries
touch ~/.finfocus/plugins/test-plugin/v1.0.0/finfocus-plugin-test-plugin
chmod +x ~/.finfocus/plugins/test-plugin/v1.0.0/finfocus-plugin-test-plugin

touch ~/.finfocus/plugins/test-plugin/v2.0.0/finfocus-plugin-test-plugin
chmod +x ~/.finfocus/plugins/test-plugin/v2.0.0/finfocus-plugin-test-plugin
```

### 2. Verify List (All Versions)

If a command exists to list all (e.g. `finfocus plugin list --all`), verify it shows both.

### 3. Verify Selection (Latest)

Run a command that uses plugins (e.g., `finfocus cost ...` or `finfocus plugin list` if it defaults to latest).

*Note: As of this plan, the specific CLI command output verification depends on how `plugin list` is implemented. The core logic verification is done via unit tests.*

### 4. Run Unit Tests

To verify the logic without CLI:

```bash
go test -v ./internal/registry/ -run TestListLatestPlugins
```