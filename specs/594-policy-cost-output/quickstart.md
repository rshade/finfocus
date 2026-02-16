# Quickstart: Policy-Compatible Cost Output

## Prerequisites

- FinFocus CLI built (`make build`)
- Pulumi CLI installed
- A Pulumi project with at least one resource

## 1. Configure Cost Threshold

### Via config file

Add to `~/.finfocus/config.yaml` (global) or `$PROJECT/.finfocus/config.yaml` (project):

```yaml
analyzer:
  max_monthly_cost: 5000.00
  enforcement: advisory    # or "mandatory" to block deployments
```

### Via environment variables

```bash
export FINFOCUS_MAX_MONTHLY_COST=5000
export FINFOCUS_ENFORCEMENT=mandatory
```

## 2. Run Pulumi Preview with FinFocus Analyzer

```bash
pulumi preview --analyzer=finfocus
```

## 3. Check Results

### Threshold diagnostic in preview output

If cost exceeds threshold (advisory mode):

```text
warning: [finfocus] Stack cost $7,500.00/mo exceeds threshold $5,000.00/mo
```

If cost exceeds threshold (mandatory mode):

```text
error: [finfocus] Stack cost $7,500.00/mo exceeds threshold $5,000.00/mo — deployment blocked
```

### Cost summary file

After the preview completes, read the summary file:

```bash
cat $PROJECT/.finfocus/last-cost-summary.json | jq .
```

Example output:

```json
{
  "schema_version": "1",
  "timestamp": "2026-02-16T10:30:00Z",
  "stack": "dev",
  "project": "my-infra",
  "total_monthly_cost": 7500.50,
  "currency": "USD",
  "resource_count": 12,
  "resources": [
    {
      "type": "aws:ec2/instance:Instance",
      "name": "web-server",
      "monthly_cost": 150.00,
      "currency": "USD",
      "adapter": "aws-public"
    }
  ]
}
```

### Parse diagnostic metadata

Extract machine-parseable cost data from diagnostic output:

```bash
# The diagnostic message includes embedded metadata:
# Estimated Monthly Cost: $150.00 USD (source: aws-public)
# <!-- finfocus:cost:{"monthly":150.00,"currency":"USD","adapter":"aws-public"} -->
```

## 4. CI/CD Integration Example

```yaml
# GitHub Actions example
- name: Pulumi Preview with Cost Check
  run: |
    export FINFOCUS_MAX_MONTHLY_COST=10000
    export FINFOCUS_ENFORCEMENT=mandatory
    pulumi preview --analyzer=finfocus

- name: Read Cost Summary
  if: always()
  run: |
    if [ -f ".finfocus/last-cost-summary.json" ]; then
      TOTAL=$(jq '.total_monthly_cost' .finfocus/last-cost-summary.json)
      echo "Total monthly cost: \$${TOTAL}"
    fi
```
