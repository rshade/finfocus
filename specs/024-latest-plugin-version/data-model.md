# Data Model: Latest Plugin Version

## Entities

### PluginInfo

Represents a discovered plugin version on the file system.

```go
type PluginInfo struct {
    Name    string // Plugin name (e.g., "kubecost", "aws-public")
    Version string // Semver string (e.g., "v1.0.0")
    Path    string // Absolute path to the plugin binary
}
```

**Validation Rules**:
- `Name`: Non-empty string.
- `Version`: Must be parseable by `semver/v3`.
- `Path`: Must exist and be executable.

## Registry State

The `Registry` does not maintain persistent state between calls; it scans the filesystem on demand.

- **ListPlugins**: Returns `[]PluginInfo` (all versions).
- **ListLatestPlugins**: Returns `[]PluginInfo` (unique by Name, highest Version).