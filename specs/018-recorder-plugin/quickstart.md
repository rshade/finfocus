# Quickstart: Recorder Plugin

**Time to complete**: ~10 minutes

This guide shows how to build, install, and use the Recorder plugin to inspect gRPC requests from FinFocus Core.

## Prerequisites

- Go 1.25.6+
- finfocus-core built (`make build`)
- A Pulumi plan JSON file (optional, for testing)

## Build and Install the Plugin

```bash
# From finfocus-core repository root
make install-recorder

# Verify installation
./bin/finfocus plugin list
```

Expected output:

```text
PLUGIN     VERSION  PATH
recorder   0.1.0    ~/.finfocus/plugins/recorder/0.1.0/finfocus-plugin-recorder
```

## Basic Usage

### Inspect Request Data (Default Mode)

Run with a sample Pulumi plan to capture requests:

```bash
# Set output directory (optional, defaults to ./recorded_data)
export FINFOCUS_RECORDER_OUTPUT_DIR=./my-recordings

# Run cost calculation
./bin/finfocus cost projected --pulumi-json examples/plans/aws-simple-plan.json

# View recorded requests
ls -la ./my-recordings/
cat ./my-recordings/*.json | jq .
```

### Enable Mock Responses

For testing Core's aggregation logic without real costs:

```bash
# Enable mock mode
export FINFOCUS_RECORDER_MOCK_RESPONSE=true

# Run cost calculation (will show randomized costs)
./bin/finfocus cost projected --pulumi-json examples/plans/aws-simple-plan.json --output json
```

Example output with mock mode:

```json
{
  "results": [
    {
      "resourceType": "aws:ec2:Instance",
      "adapter": "recorder",
      "currency": "USD",
      "monthly": 73.42,
      "notes": "Mock cost: $73.42/month (recorder plugin)"
    }
  ]
}
```

## Configuration Reference

| Environment Variable | Default | Description |
|---------------------|---------|-------------|
| `FINFOCUS_RECORDER_OUTPUT_DIR` | `./recorded_data` | Directory for JSON files |
| `FINFOCUS_RECORDER_MOCK_RESPONSE` | `false` | Enable randomized responses |

## Recorded Request Format

Each request creates a JSON file:

```text
./recorded_data/
├── 20251211T143052Z_Name_01JEK7X2J3K4M5N6P7Q8R9S0T1.json
├── 20251211T143052Z_GetProjectedCost_01JEK7X2J3K4M5N6P7Q8R9S1T2.json
└── 20251211T143053Z_GetProjectedCost_01JEK7X2J3K4M5N6P7Q8R9S2T3.json
```

Example file content:

```json
{
  "timestamp": "2025-12-11T14:30:52Z",
  "method": "GetProjectedCost",
  "requestId": "01JEK7X2J3K4M5N6P7Q8R9S1T2",
  "request": {
    "resource": {
      "resourceType": "aws:ec2:Instance",
      "provider": "aws",
      "sku": "t3.medium",
      "region": "us-east-1",
      "tags": {
        "instanceType": "t3.medium"
      }
    }
  }
}
```

## Use Cases

### 1. Debug Plugin Integration

See exactly what data Core sends to plugins:

```bash
FINFOCUS_RECORDER_OUTPUT_DIR=./debug ./bin/finfocus cost projected --pulumi-json my-plan.json
cat ./debug/*.json | jq '.request.resource'
```

### 2. Test Core Aggregation

Generate mock data to test output formatting:

```bash
FINFOCUS_RECORDER_MOCK_RESPONSE=true ./bin/finfocus cost projected \
  --pulumi-json examples/plans/aws-simple-plan.json \
  --output table
```

### 3. Contract Testing

Use as a reference plugin in integration tests:

```bash
# In test setup
export FINFOCUS_RECORDER_OUTPUT_DIR=/tmp/test-recordings
export FINFOCUS_RECORDER_MOCK_RESPONSE=true

# Run tests
go test ./test/integration/... -v
```

## Troubleshooting

### Plugin Not Found

```bash
# Check plugin directory structure
ls -la ~/.finfocus/plugins/recorder/

# Verify binary is executable
chmod +x ~/.finfocus/plugins/recorder/0.1.0/finfocus-plugin-recorder
```

### No Files Being Recorded

```bash
# Check output directory exists and is writable
mkdir -p "$FINFOCUS_RECORDER_OUTPUT_DIR"
touch "$FINFOCUS_RECORDER_OUTPUT_DIR/test.txt" && rm "$FINFOCUS_RECORDER_OUTPUT_DIR/test.txt"

# Check plugin is being used (not falling back to specs)
./bin/finfocus cost projected --debug --pulumi-json plan.json 2>&1 | grep recorder
```

### Mock Responses Not Working

```bash
# Verify environment variable is set correctly
echo $FINFOCUS_RECORDER_MOCK_RESPONSE  # Should be "true"

# Check for typos (case-insensitive)
export FINFOCUS_RECORDER_MOCK_RESPONSE=TRUE  # Also works
```

## Next Steps

- Read the [full plugin documentation](../../plugins/recorder/README.md)
- Study the [source code](../../plugins/recorder/) as a reference implementation
- Use recorded data to build your own plugin
