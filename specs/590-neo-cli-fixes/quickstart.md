# Quickstart: Neo-Friendly CLI Fixes

**Feature Branch**: `590-neo-cli-fixes`

## Prerequisites

- Go 1.25.7+
- `make build` succeeds
- At least one plugin installed (for plugin list testing)

## Verifying Exit Codes

```bash
# Build the binary
make build

# Configure a budget with custom exit code
cat > /tmp/finfocus-budget.yaml << 'EOF'
budget:
  monthly_limit: 0.01
  currency: USD
  exit_on_threshold: true
  exit_code: 2
EOF

# Run projected cost with budget (should exit 2 if costs exceed $0.01)
./bin/finfocus cost projected \
  --pulumi-json examples/plans/aws-simple-plan.json \
  --config /tmp/finfocus-budget.yaml
echo "Exit code: $?"
# Expected: Exit code: 2

# Run with costs within budget (should exit 0)
cat > /tmp/finfocus-budget-high.yaml << 'EOF'
budget:
  monthly_limit: 999999
  currency: USD
  exit_on_threshold: true
  exit_code: 2
EOF

./bin/finfocus cost projected \
  --pulumi-json examples/plans/aws-simple-plan.json \
  --config /tmp/finfocus-budget-high.yaml
echo "Exit code: $?"
# Expected: Exit code: 0
```

## Verifying Structured Errors in JSON

```bash
# Run with JSON output to see structured errors
# (Use a plan with resources that will fail validation or plugin lookup)
./bin/finfocus cost projected \
  --pulumi-json examples/plans/aws-simple-plan.json \
  --output json 2>/dev/null | jq '.finfocus.results[] | select(.error != null)'

# Expected output shape:
# {
#   "resourceType": "aws:ec2:Instance",
#   "error": {
#     "code": "NO_COST_DATA",
#     "message": "No pricing information available",
#     "resourceType": "aws:ec2:Instance"
#   }
# }
```

## Verifying Plugin List JSON

```bash
# List plugins as JSON
./bin/finfocus plugin list --output json | jq .

# Expected output shape:
# [
#   {
#     "name": "recorder",
#     "version": "0.1.0",
#     "path": "/home/user/.finfocus/plugins/recorder/0.1.0/finfocus-plugin-recorder",
#     "specVersion": "v0.5.6",
#     "runtimeVersion": "0.1.0",
#     "supportedProviders": ["*"],
#     "capabilities": ["projected_costs", "actual_costs"]
#   }
# ]

# With no plugins installed (empty array)
# Expected: []
```

## Running Tests

```bash
# Run all tests
make test

# Run specific test suites
go test -v ./cmd/finfocus/... -run TestMainExitCode
go test -v ./internal/engine/... -run TestStructuredError
go test -v ./internal/cli/... -run TestPluginListJSON

# Lint
make lint
```
