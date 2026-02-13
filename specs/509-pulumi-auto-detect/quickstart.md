# Quickstart: Automatic Pulumi Integration

**Feature**: 509-pulumi-auto-detect

## Prerequisites

- FinFocus CLI installed (`finfocus` in PATH)
- Pulumi CLI installed (`pulumi` in PATH) — [Install Pulumi](https://www.pulumi.com/docs/install/)
- A Pulumi project with at least one configured stack

## Usage

### Projected Costs (simplest case)

```bash
cd my-pulumi-project/
finfocus cost projected
```

This automatically:

1. Detects the Pulumi project (finds `Pulumi.yaml`)
2. Identifies the current stack
3. Runs `pulumi preview --json`
4. Calculates and displays projected costs

### Actual Costs

```bash
cd my-pulumi-project/
finfocus cost actual
```

This automatically:

1. Detects the Pulumi project
2. Exports the current stack state
3. Auto-detects the date range from resource timestamps
4. Calculates and displays actual historical costs

### Targeting a Specific Stack

```bash
finfocus cost projected --stack production
finfocus cost actual --stack staging --from 2026-01-01
```

### Using Pre-Generated Files (existing workflow)

The traditional workflow still works:

```bash
# Projected costs from file
pulumi preview --json > plan.json
finfocus cost projected --pulumi-json plan.json

# Actual costs from file
pulumi stack export > state.json
finfocus cost actual --pulumi-state state.json --from 2026-01-01
```

## Troubleshooting

### "pulumi CLI not found"

Install Pulumi: `curl -fsSL https://get.pulumi.com | sh`

Or use the file-based workflow: `finfocus cost projected --pulumi-json plan.json`

### "no Pulumi project found"

Make sure you're in a directory containing `Pulumi.yaml` or a subdirectory of one.

### "no active Pulumi stack"

Either set a current stack (`pulumi stack select dev`) or specify one explicitly:

```bash
finfocus cost projected --stack dev
```

### Preview is slow

Large infrastructure stacks can take several minutes to preview. The tool will show progress indication. You can cancel with Ctrl+C at any time.
