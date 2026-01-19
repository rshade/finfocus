# Registry API Contract

## Interface: Registry

The internal registry interface for plugin discovery.

### Method: `ListLatestPlugins`

Scans the plugin directory and returns the latest version of each plugin.

**Signature**:
```go
func (r *Registry) ListLatestPlugins() ([]PluginInfo, []string, error)
```

**Returns**:
- `[]PluginInfo`: List of unique plugins (by name), each being the highest semver found.
- `[]string`: List of warnings (e.g., invalid versions skipped).
- `error`: Filesystem errors (e.g., permission denied).

**Behavior**:
1. Scans `~/.finfocus/plugins/`.
2. Groups by plugin name.
3. Parses versions using SemVer.
4. Selects highest version.
5. Ignores/warns on invalid versions.

### Method: `Open`

**Signature**:
```go
func (r *Registry) Open(ctx context.Context, onlyName string) ([]*pluginhost.Client, func(), error)
```

**Behavior**:
- MUST call `ListLatestPlugins` to determine which binaries to execute.