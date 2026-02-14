# Contract: Plugin List JSON Output

## JSON Schema

### Plugin List Response

```json
{
  "type": "array",
  "items": {
    "type": "object",
    "required": ["name", "version", "path"],
    "properties": {
      "name": {
        "type": "string",
        "description": "Plugin name (directory name in registry)"
      },
      "version": {
        "type": "string",
        "description": "Plugin version (directory name in registry)"
      },
      "path": {
        "type": "string",
        "description": "Absolute path to plugin binary"
      },
      "specVersion": {
        "type": "string",
        "description": "finfocus-spec version the plugin was built against"
      },
      "runtimeVersion": {
        "type": "string",
        "description": "Plugin runtime version from GetPluginInfo()"
      },
      "supportedProviders": {
        "type": "array",
        "items": { "type": "string" },
        "description": "Cloud providers this plugin supports (e.g., [\"aws\"], [\"*\"])"
      },
      "capabilities": {
        "type": "array",
        "items": { "type": "string" },
        "description": "Plugin capabilities (e.g., [\"projected_costs\", \"actual_costs\"])"
      },
      "notes": {
        "type": "string",
        "description": "Error or status notes if metadata retrieval failed"
      }
    }
  }
}
```

### Examples

#### Plugins installed

```json
[
  {
    "name": "aws-public",
    "version": "1.0.0",
    "path": "/home/user/.finfocus/plugins/aws-public/1.0.0/finfocus-plugin-aws-public",
    "specVersion": "v0.5.6",
    "runtimeVersion": "1.0.0",
    "supportedProviders": ["aws"],
    "capabilities": ["projected_costs", "actual_costs", "recommendations"]
  },
  {
    "name": "recorder",
    "version": "0.1.0",
    "path": "/home/user/.finfocus/plugins/recorder/0.1.0/finfocus-plugin-recorder",
    "specVersion": "v0.5.6",
    "runtimeVersion": "0.1.0",
    "supportedProviders": ["*"],
    "capabilities": ["projected_costs", "actual_costs"],
    "notes": ""
  }
]
```

#### No plugins installed

```json
[]
```

#### Plugin with metadata retrieval failure

```json
[
  {
    "name": "broken-plugin",
    "version": "0.0.1",
    "path": "/home/user/.finfocus/plugins/broken-plugin/0.0.1/finfocus-plugin-broken",
    "specVersion": "N/A",
    "runtimeVersion": "N/A",
    "supportedProviders": null,
    "capabilities": null,
    "notes": "failed to connect: connection refused"
  }
]
```

### Key Invariants

1. Output is always a JSON array, even with zero or one plugin
2. Empty plugin list produces `[]`, never `null` or an error message
3. Failed metadata retrieval still produces a plugin entry with `notes` field
4. `supportedProviders` and `capabilities` may be `null` on failure (FR-010)
5. Table output is completely unchanged (FR-009)
