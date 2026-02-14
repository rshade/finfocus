# Contract: Structured Error in JSON Output

## JSON Schema

### StructuredError Object

```json
{
  "type": "object",
  "required": ["code", "message", "resourceType"],
  "properties": {
    "code": {
      "type": "string",
      "enum": ["PLUGIN_ERROR", "VALIDATION_ERROR", "TIMEOUT_ERROR", "NO_COST_DATA"],
      "description": "Stable error category identifier. New codes may be added in minor versions but existing codes will never be removed or renamed."
    },
    "message": {
      "type": "string",
      "description": "Human-readable error description for logging or display."
    },
    "resourceType": {
      "type": "string",
      "description": "The cloud resource type that caused the error (e.g., aws:ec2:Instance)."
    }
  },
  "additionalProperties": false
}
```

### CostResult with Error (extended)

```json
{
  "resourceType": "aws:ec2:Instance",
  "resourceId": "urn:pulumi:stack::project::aws:ec2/instance:Instance::my-instance",
  "adapter": "",
  "currency": "USD",
  "monthly": 0,
  "hourly": 0,
  "notes": "",
  "breakdown": null,
  "error": {
    "code": "PLUGIN_ERROR",
    "message": "connection refused: plugin aws-public not responding",
    "resourceType": "aws:ec2:Instance"
  }
}
```

### Key Invariants

1. `error` field is `null` or absent when no error occurred
2. When `error` is present, `notes` MUST NOT contain `ERROR:` or `VALIDATION:` prefixes
3. When `error` is present, `monthly` and `hourly` are `0`
4. Error codes are stable across versions (FR-005)
5. Table output ignores the `error` field entirely (FR-009)
