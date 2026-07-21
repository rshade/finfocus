# Analyzer Integration Testing Notes

This document tracks what we've learned about integrating the finfocus analyzer
with Pulumi.

## Current Status: WORKING

The analyzer integration is fully functional as of 2025-12-09.

## Working Configuration

### Required Files

1. **Binary**: `pulumi-analyzer-policy-finfocus` (copy of finfocus binary)
2. **Config**: `PulumiPolicy.yaml`

### PulumiPolicy.yaml Contents

```yaml
runtime: finfocus
name: finfocus
version: 0.0.0-dev
```

### Commands to Set Up

```bash
# Create policy pack directory
mkdir -p /path/to/policy-pack

# Copy binary with correct name
cp bin/finfocus /path/to/policy-pack/pulumi-analyzer-policy-finfocus
chmod +x /path/to/policy-pack/pulumi-analyzer-policy-finfocus

# Create PulumiPolicy.yaml
cat > /path/to/policy-pack/PulumiPolicy.yaml << 'EOF'
runtime: finfocus
name: finfocus
version: 0.0.0-dev
EOF

# Add to PATH and run
export PATH="/path/to/policy-pack:$PATH"
pulumi preview --policy-pack /path/to/policy-pack
```

## Test Log

### Test Session: 2025-12-09

**Binary Location**: `$GOPATH/src/github.com/rshade/finfocus/bin/finfocus`

**Test Project**: `$GOPATH/src/github.com/rshade/finfocus/test/e2e/projects/analyzer`

#### Test 1: Binary Name Detection

- **Method**: Run binary named `pulumi-analyzer-policy-finfocus`
- **Command**: `/tmp/pulumi-analyzer-policy-finfocus`
- **Expected**: Print port to stdout
- **Result**: SUCCESS - Port `41331` printed to stdout

#### Test 2: Full Pulumi Integration (no AWS creds)

- **Method**: `pulumi preview --policy-pack`
- **Command**: `pulumi preview --policy-pack /tmp/finfocus-policy-test`
- **Expected**: Analyzer loads and returns diagnostics
- **Result**: SUCCESS - Diagnostics for internal types shown:

```text
Policies:
    finfocus@v0.0.0-dev (local: /tmp/finfocus-policy-test)
        - [advisory] cost-estimate (pulumi:providers:aws: default)
          Internal Pulumi resource (no cloud cost)
        - [advisory] cost-estimate (pulumi:pulumi:Stack: ...)
          Internal Pulumi resource (no cloud cost)
```

#### Test 3: Full Pulumi Integration (with AWS creds)

- **Method**: `pulumi preview --policy-pack` with AWS credentials
- **Command**:

```bash
eval "$(aws configure export-credentials --format env)"
pulumi preview --policy-pack /tmp/finfocus-policy-test
```

- **Expected**: Cost estimates for EC2 instance
- **Result**: SUCCESS - Full output:

```text
Policies:
    finfocus@v0.0.0-dev (local: /tmp/finfocus-policy-test)
        - [advisory] cost-estimate (pulumi:providers:aws: default)
          Internal Pulumi resource (no cloud cost)
        - [advisory] cost-estimate (pulumi:pulumi:Stack: ...)
          Internal Pulumi resource (no cloud cost)
        - [advisory] cost-estimate (aws:ec2/instance:Instance: test-instance)
          Estimated Monthly Cost: $7.59 USD (source: finfocus-plugin-aws-public)
        - [advisory] stack-cost-summary (pulumi:pulumi:Stack: ...)
          Total Estimated Monthly Cost: $7.59 USD (1 resources analyzed)
```

#### Test 4: No Duplicate Diagnostics

- **Verified**: Each resource appears once, summary appears once
- **Result**: SUCCESS - The cost caching mechanism works correctly

## Methods Tested and Results

### 1. Policy Pack Approach (--policy-pack flag) - WORKING

**Setup**:

```bash
mkdir -p /tmp/policy-pack
cp bin/finfocus /tmp/policy-pack/pulumi-analyzer-policy-finfocus
cat > /tmp/policy-pack/PulumiPolicy.yaml << 'EOF'
runtime: finfocus
name: finfocus
version: 0.0.0-dev
EOF
export PATH="/tmp/policy-pack:$PATH"
```

**Command**:

```bash
pulumi preview --policy-pack /tmp/policy-pack
```

**Result**: WORKS

### 2. Plugin Directory Approach (~/.pulumi/plugins/) - NOT TESTED

This approach is for provider plugins, not analyzer plugins.

### 3. plugins.analyzers in Pulumi.yaml - DOES NOT WORK

**Finding**: The `plugins.analyzers` section only provides path hints for plugin
discovery. It does NOT automatically load or invoke the analyzer.

## Key Findings

### Binary Naming

- Must be named `pulumi-analyzer-policy-<runtime>`
- Runtime from PulumiPolicy.yaml determines suffix
- Binary must be on PATH or in policy pack directory

### Stdout/Stderr Protocol

- Port number MUST be printed to stdout (only the port, nothing else)
- All logging MUST go to stderr
- Breaking this protocol causes handshake failure

### RPC Flow

1. Pulumi calls `ConfigureStack` (clears cache)
2. Pulumi calls `Analyze` for each resource (per-resource diagnostics + caching)
3. Pulumi calls `AnalyzeStack` at end (summary only, uses cached costs)

### Cost Caching

- Costs are cached during `Analyze()` calls
- `AnalyzeStack()` uses cached costs for summary
- This prevents re-querying plugins with different property formats
- This prevents duplicate diagnostics

## Documentation Created

- `/docs/analyzer-integration.md` - User-facing integration guide
- This file - Technical testing notes

## References

- [Pulumi Policy Pack docs](https://www.pulumi.com/docs/using-pulumi/crossguard/)
