# Routing Scenarios

## Multi-Region (Separate Pricing Plugins)

Each region has its own plugin binary with region-specific pricing data.
Equal priority = queried in parallel; region matching routes automatically.

```yaml
routing:
  plugins:
    - name: aws-public-us-east-1
      priority: 10
    - name: aws-public-eu-west-1
      priority: 10
```

Region is extracted from resource properties and matched against
`client.Metadata.Metadata["region"]` (set via `--metadata="region=..."` at
install time).

## Feature-Based (Projected vs Actual from Different Sources)

Route projected costs to public pricing and actual costs to Cost Explorer.

```yaml
routing:
  plugins:
    - name: aws-public
      priority: 10
      features: [ProjectedCosts]
    - name: aws-ce
      priority: 10
      features: [ActualCosts, Recommendations]
```

## Pattern-Based with Fallback

Specialist plugin for EC2, general plugin as fallback for everything else.

```yaml
routing:
  plugins:
    - name: ec2-specialist
      priority: 10
      patterns:
        - type: glob
          pattern: "aws:ec2*"
      fallback: true
    - name: aws-public
      priority: 5
```

If `ec2-specialist` returns nil/empty for a matched resource, fallback to
`aws-public` (priority 5). If it returns $0.00, that's a valid result — no
fallback.

## Multi-Cloud

Multiple providers with a recorder plugin for debugging.

```yaml
routing:
  plugins:
    - name: aws-public
      priority: 10
    - name: gcp-public
      priority: 10
    - name: kubecost
      priority: 10
    - name: recorder
      priority: 1
      fallback: true
```

Automatic provider matching routes `aws:*` to aws-public, `gcp:*` to
gcp-public, `kubernetes:*` to kubecost. The recorder (priority 1) only
receives resources that fall through.

## Regex Patterns

Match multiple services with one pattern.

```yaml
routing:
  plugins:
    - name: aws-compute-specialist
      priority: 10
      patterns:
        - type: regex
          pattern: "aws:(ec2|ecs|eks|lambda)/.*"
    - name: aws-public
      priority: 5
```

## Disable Fallback for Strict Routing

Ensure only the designated plugin handles specific resources.

```yaml
routing:
  plugins:
    - name: production-pricer
      priority: 10
      patterns:
        - type: glob
          pattern: "aws:ec2:*"
      fallback: false     # Never fall through
    - name: aws-public
      priority: 5
```

## Testing Routes

```bash
# See which plugin handles a resource type
finfocus config routes test aws:ec2:Instance
# Output: RANK | PLUGIN | PRIORITY | MATCH_REASON | SOURCE

# With region
finfocus config routes test aws:ec2:Instance us-east-1

# JSON for scripting
finfocus config routes test aws:ec2:Instance --output json

# List all configured rules
finfocus config routes list
# Output: PRIORITY | PLUGIN | FEATURES | PATTERNS | FALLBACK
```

## Debugging

```bash
# Enable debug logging to see routing decisions
FINFOCUS_LOG_LEVEL=debug finfocus cost projected --pulumi-json plan.json

# Verify routing config source
finfocus config routes list --output json
# Check "source": "project" or "global" and "config_path"
```
