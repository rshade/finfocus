# Quickstart: Config Routes CLI Commands

**Feature**: 604-config-routes-cli

## Prerequisites

- FinFocus CLI built (`make build`)
- Optional: routing configuration in `~/.finfocus/config.yaml`

## Usage

### View routing configuration

```bash
# Display routing rules table
finfocus config routes list

# JSON output for scripting
finfocus config routes list --output json
```

### Test plugin selection

```bash
# Which plugins match an EC2 Instance?
finfocus config routes test aws:ec2:Instance

# Factor in region
finfocus config routes test aws:ec2:Instance us-east-1

# JSON output
finfocus config routes test aws:ec2:Instance --output json
```

### Example routing configuration

Add to `~/.finfocus/config.yaml`:

```yaml
routing:
  plugins:
    - name: aws-public
      priority: 10
      features:
        - ProjectedCosts
      patterns:
        - type: glob
          pattern: "aws:ec2:*"
    - name: aws-ce
      priority: 5
      features:
        - ActualCosts
        - Recommendations
      patterns:
        - type: glob
          pattern: "aws:*"
      fallback: true
    - name: recorder
      priority: 1
      fallback: true
```

### Expected output

**`config routes list`:**

```text
PLUGIN ROUTING RULES

  PRIORITY  PLUGIN      FEATURES                      PATTERNS       FALLBACK
  --------  ------      --------                      --------       --------
  10        aws-public  ProjectedCosts                glob:aws:ec2:* no
  5         aws-ce      ActualCosts,Recommendations   glob:aws:*     yes
  1         recorder    (all)                         (all)          yes

  Source: /home/user/.finfocus/config.yaml (global)
```

**`config routes test aws:ec2:Instance`:**

```text
Plugin selection for aws:ec2:Instance:

  #  PLUGIN      PRIORITY  MATCH REASON  SOURCE
  -  ------      --------  ------------  ------
  1  aws-public  10        pattern       config
  2  aws-ce      5         pattern       config
  3  recorder    1         global        automatic

Feature availability:
  ProjectedCosts:   aws-public (priority 10)
  ActualCosts:      aws-ce (priority 5)
  Recommendations:  aws-ce (priority 5)
  Carbon:           recorder (priority 1)
  DryRun:           recorder (priority 1)
  Budgets:          recorder (priority 1)
```

## Troubleshooting

**"No routing configured (automatic mode)"**: No `routing` section exists in
config. All plugins are queried for all resource types based on their
`SupportedProviders` metadata.

**"No plugins match"**: The test resource type doesn't match any configured
patterns and no plugins support the extracted provider automatically. Check
your patterns and provider names.
