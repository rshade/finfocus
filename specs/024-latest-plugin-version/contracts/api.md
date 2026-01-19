# API Contracts: Latest Plugin Version Selection

**Date**: 2025-01-05  
**Feature**: Latest Plugin Version Selection

## Plugin Registry API

### Core Operations

#### ListPlugins

Returns the latest version of each installed plugin for cost analysis operations.

**Endpoint**: Internal Go API  
**Method**: `ListPlugins() ([]Plugin, []Warning, error)`

**Request Parameters**: None (uses configured plugin directory)

**Response**:

```go
type Plugin struct {
    Name    string         `json:"name"`
    Version string         `json:"version"`
    Path    string         `json:"path"`
}

type Warning struct {
    Path    string `json:"path"`
    Message string `json:"message"`
}
```

**Behavior**:

- Scans plugin directory structure
- Returns only latest version per plugin name
- Includes warnings for invalid/corrupted plugins
- Excludes plugins with invalid versions from results

**Error Conditions**:

- Plugin directory doesn't exist (error)
- Insufficient permissions to plugin directory (error)
- Critical file system errors (error)

#### ListAllPlugins

Returns all installed versions of all plugins for inventory management.

**Endpoint**: Internal Go API  
**Method**: `ListAllPlugins() ([]Plugin, []Warning, error)`

**Request Parameters**: None

**Response**: Same structure as `ListPlugins` but includes all versions

**Behavior**:

- Scans plugin directory structure
- Returns all versions of all plugins
- Includes warnings for invalid/corrupted plugins
- Does not filter by latest version

#### GetLatestPlugin

Returns the latest version of a specific plugin by name.

**Endpoint**: Internal Go API  
**Method**: `GetLatestPlugin(name string) (Plugin, error)`

**Request Parameters**:

- `name` (string, required): Plugin name to search for

**Response**: Single `Plugin` struct

**Behavior**:

- Scans for all versions of specified plugin
- Returns only the latest version
- Returns error if plugin not found

**Error Conditions**:

- Plugin not found (error)
- Invalid plugin name (error)

## CLI Command Contracts

### plugin list Command

Lists all installed plugins with their versions.

**Command**: `finfocus plugin list`  
**Output Format**: Human-readable table

**Response Structure**:

```
PLUGIN NAME    VERSION    PATH
aws-public     v2.0.0     /home/user/.finfocus/plugins/aws-public/v2.0.0
aws-public     v1.0.0     /home/user/.finfocus/plugins/aws-public/v1.0.0
vantage        v1.1.0     /home/user/.finfocus/plugins/vantage/v1.1.0
```

**Behavior**:

- Uses `ListAllPlugins()` internally
- Displays all versions of all plugins
- Shows warnings for invalid plugins
- Returns exit code 0 even if some plugins have warnings

### cost analyze Command

Runs cost analysis using latest plugin versions.

**Command**: `finfocus cost analyze [options]`  
**Behavior**: Uses `ListPlugins()` internally to get latest versions

**Error Handling**:

- Warnings displayed but don't stop analysis
- Critical errors stop analysis with error message

## Error Handling Contracts

### Warning Types

#### InvalidVersionWarning

**Condition**: Directory name is not valid semantic version  
**Message**: `"Invalid version string '{dir}' in plugin '{plugin}', skipping"`

#### CorruptedDirectoryWarning

**Condition**: Plugin directory exists but is inaccessible or corrupted  
**Message**: `"Corrupted plugin directory '{path}': {error}, skipping"`

#### PermissionWarning

**Condition**: Cannot access plugin directory due to permissions  
**Message**: `"Permission denied accessing '{path}': {error}, skipping"`

### Error Types

#### PluginDirectoryError

**Condition**: Plugin root directory doesn't exist or is inaccessible  
**Message**: `"Plugin directory '{path}' not found or inaccessible: {error}"`

#### ConfigurationError

**Condition**: Invalid plugin directory configuration  
**Message**: `"Invalid plugin directory configuration: {error}"`

#### CriticalFileSystemError

**Condition**: Unrecoverable file system error during scanning  
**Message**: `"Critical file system error scanning plugins: {error}"`

## Performance Contracts

### Response Time Targets

- **Small inventory** (<100 plugins): <50ms total scan time
- **Medium inventory** (100-500 plugins): <200ms total scan time
- **Large inventory** (500-1,000 plugins): <500ms total scan time

### Memory Usage Targets

- **Per-plugin memory**: ~1-2KB (metadata + version info)
- **Total registry memory**: <10MB for 1,000 plugins
- **Peak memory during scan**: <50MB for large inventories

### Concurrency Guarantees

- Thread-safe for concurrent read operations
- Plugin instances are immutable
- Registry operations are reentrant

## Integration Contracts

### Plugin Host Integration

**Interface**: Plugin host receives filtered plugin list from registry  
**Method**: `LoadPlugins(plugins []Plugin) error`  
**Behavior**: Loads only latest versions for cost analysis

### Configuration Integration

**Interface**: Registry reads plugin directory from configuration  
**Method**: `GetPluginDirectory() (string, error)`  
**Behavior**: Returns validated plugin directory path

### Logging Integration

**Interface**: Registry logs warnings and errors through structured logger  
**Method**: `Warn(msg string, fields ...Field)`  
**Behavior**: Logs warnings with context (path, error details)

## Testing Contracts

### Unit Test Contracts

**Coverage Requirements**:

- Version comparison logic: 100% coverage
- Plugin discovery logic: 95% coverage
- Error handling paths: 90% coverage

**Test Fixtures**:

- Multi-version plugin directories
- Invalid version directories
- Corrupted plugin directories
- Empty plugin directories

### Integration Test Contracts

**Test Scenarios**:

- End-to-end plugin discovery
- CLI command integration
- Error handling propagation
- Performance validation

**Test Data**:

- Real plugin directory structures
- Simulated permission errors
- Large plugin inventories (performance testing)
