# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Registry Package Overview

The `internal/registry` package implements plugin discovery and lifecycle management for FinFocus. It scans the local filesystem for installed plugins, validates their structure, and provides clients with connections to active plugin processes.

## Architecture

### Core Components

1. **Registry** (`registry.go`)
   - Main plugin discovery and management interface
   - Scans `~/.finfocus/plugins/<name>/<version>/` directory structure
   - Creates and manages plugin client connections
   - Implements platform-specific binary detection

2. **Manifest System** (`manifest.go`)  
   - JSON-based plugin metadata handling
   - Optional `plugin.manifest.json` files in plugin directories
   - Validation for plugin name/version consistency

### Plugin Directory Structure

```text
~/.finfocus/plugins/
├── aws-plugin/
│   └── v1.0.0/
│       ├── aws-plugin(.exe)          # Executable binary
│       └── plugin.manifest.json     # Optional metadata
├── kubecost/
│   ├── v1.0.0/
│   │   ├── kubecost(.exe)
│   │   └── plugin.manifest.json
│   └── v2.1.0/
│       ├── kubecost(.exe)
│       └── plugin.manifest.json
```

## Testing Commands

```bash
# Run registry tests
go test ./internal/registry/...

# Run with verbose output to see test structure creation
go test -v ./internal/registry/...

# Test registry functionality via CLI commands
./bin/finfocus plugin list
./bin/finfocus plugin validate

# Test with specific setup scenarios
go test ./internal/registry/... -run TestListPlugins
go test ./internal/registry/... -run TestFindBinary
```

## Plugin Discovery Flow

### ListPlugins Process

1. **Check Root Directory**: Return empty list if `~/.finfocus/plugins` doesn't exist
2. **Scan Plugin Names**: Iterate through subdirectories (plugin names)
3. **Scan Versions**: For each plugin, iterate through version subdirectories
4. **Find Binaries**: Search for executable files in version directories
5. **Build PluginInfo**: Create metadata structure with name, version, and path

### Binary Detection Logic

**Metadata-Aware Selection:**

When `plugin.metadata.json` exists in the version directory and contains a
`region` key (e.g., `"region": "us-west-2"`), the registry first looks for a
region-specific binary named `finfocus-plugin-<name>-<region>`. If the region
binary is not found, it falls back to the standard naming patterns below.

**Unix/Linux/macOS:**

- Check file permissions for executable bit (`info.Mode()&0111 != 0`)
- Any file with execute permissions is considered a valid binary

**Windows:**

- Check file extension for `.exe`
- Only `.exe` files are considered valid binaries

### Plugin Client Creation

1. **List Available Plugins**: Scan filesystem for installed plugins
2. **Filter by Name**: Optional filter to load only specific plugin
3. **Launch Processes**: Use pluginhost.NewClient() to start plugin processes
4. **Return Clients + Cleanup**: Provide active clients and cleanup function

## Manifest Validation

### Manifest Structure

```json
{
  "name": "aws-plugin",
  "version": "v1.0.0", 
  "description": "AWS cost calculation plugin",
  "author": "FinFocus Team",
  "providers": ["aws"],
  "metadata": {
    "supportedRegions": "us-east-1,us-west-2"
  }
}
```

### Validation Rules

- **Optional**: Manifests are not required for plugin operation
- **Name Consistency**: If present, manifest name must match directory name
- **Version Consistency**: If present, manifest version must match directory name
- **JSON Format**: Must be valid JSON structure

## Error Handling Patterns

### Graceful Degradation

- **Missing Directories**: Return empty plugin list, don't error
- **Invalid Binaries**: Skip non-executable files, continue scanning
- **Plugin Connection Failures**: Skip failed plugins, return successful ones
- **Manifest Errors**: Only validate if manifest exists

### Client Connection Management

```go
clients, cleanup, err := registry.Open(ctx, "")
defer cleanup() // Always call cleanup to prevent resource leaks
```

## Testing Patterns

### Directory Structure Creation

Tests use helper functions to create realistic plugin directory structures:

- `createSinglePluginDir()`: Single plugin with one version
- `createMultiplePluginsDir()`: Multiple plugins with different versions  
- `createMultiVersionPluginDir()`: One plugin with multiple versions
- `createNonExecutablePluginDir()`: Test skipping non-executable files

### Cross-Platform Testing

- Tests handle both Unix permissions and Windows `.exe` extensions
- Use `runtime.GOOS` to apply platform-specific logic
- Create appropriate binary files with correct permissions/extensions

### Temporary Directory Management

- All tests use `t.TempDir()` for isolated test environments
- Automatic cleanup prevents test pollution
- Realistic filesystem structures mimic production

## Integration Points

### Config Package

- `config.New().PluginDir` provides default plugin directory path
- Typically resolves to `~/.finfocus/plugins`

### PluginHost Package  

- Uses `pluginhost.NewProcessLauncher()` as default launcher
- Creates `pluginhost.Client` instances for active plugins
- Handles plugin process lifecycle through pluginhost abstraction

### CLI Package

- `plugin list` command uses `registry.ListPlugins()`
- `plugin validate` command uses `registry.LoadManifest()`
- CLI gracefully handles missing plugin directories

## Common Usage Patterns

### Default Registry Creation

```go
registry := registry.NewDefault() // Uses config.PluginDir and ProcessLauncher
```

### Plugin Discovery

```go
plugins, err := registry.ListPlugins()
// Returns []PluginInfo with Name, Version, Path
```

### Client Management

```go
clients, cleanup, err := registry.Open(ctx, "aws-plugin") // Filter by name
defer cleanup()
// clients contains active plugin connections
```

### Manifest Handling

```go
manifest, err := registry.LoadManifest("/path/to/plugin.manifest.json")
// Returns *Manifest with metadata
```

## Platform Considerations

### Unix/Linux/macOS

- Executable detection via file permissions (`chmod +x`)
- Standard binary naming (no extension required)
- Shell script executables supported

### Windows

- Executable detection via `.exe` extension
- Binary must have `.exe` suffix
- Batch files and PowerShell scripts not automatically detected

This package serves as the plugin discovery and lifecycle management layer, bridging the gap between the filesystem-based plugin installation and the runtime plugin communication system.

