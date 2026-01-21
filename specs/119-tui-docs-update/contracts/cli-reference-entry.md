# CLI Reference Entry Template

**Purpose**: Template for documenting CLI commands in `docs/reference/cli-commands.md`.

**Usage**: Add new command documentation following this structure. Update existing entries when flags change.

---

## Template Structure

```markdown
## finfocus [command] [subcommand]

[One-sentence description of what this command does]

### Synopsis

[Command synopsis with placeholder syntax]

\```bash
finfocus [command] [subcommand] [flags]
\```

### Description

[2-3 paragraph detailed description covering:]
- What the command does
- When to use it
- Key features or capabilities
- Related commands

### Flags

| Flag | Type | Default | Required | Description |
|------|------|---------|----------|-------------|
| `--flag-name` | string | `"default"` | No | [Description] |
| `--another-flag` | boolean | `false` | No | [Description] |

**Global Flags:**

All commands support these global flags:

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--debug` | boolean | `false` | Enable debug logging |
| `--output` | string | `"table"` | Output format (table, json, ndjson) |

### Examples

#### Example 1: [Basic usage scenario]

[Brief explanation of what this example demonstrates]

\```bash
finfocus [command] [subcommand] --flag value
\```

**Output:**

\```text
[Expected output from command]
\```

---

#### Example 2: [Intermediate scenario]

[Explanation]

\```bash
finfocus [command] [subcommand] --flag1 value1 --flag2 value2
\```

---

#### Example 3: [Advanced scenario]

[Explanation]

\```bash
finfocus [command] [subcommand] --flag value | jq '.field'
\```

### Exit Codes

| Code | Meaning |
|------|---------|
| `0` | Success |
| `1` | General error |
| `2` | [Command-specific exit code] |

### Environment Variables

These environment variables affect command behavior:

| Variable | Description | Example |
|----------|-------------|---------|
| `FINFOCUS_VAR_NAME` | [Description] | `value` |

### Related Commands

- [`finfocus [related-command]`](#finfocus-related-command) - [Brief description]
- [`finfocus [another-command]`](#finfocus-another-command) - [Brief description]

### See Also

- [Guide Name](../guides/guide-name.md) - Tutorial for this feature
- [Configuration Reference](./config-reference.md#section) - Configuration options
- [Examples](../examples/directory/) - Complete example files

---
```

---

## Complete Example: `finfocus cost recommendations`

```markdown
## finfocus cost recommendations

Display cost optimization recommendations from cloud providers.

### Synopsis

Display actionable cost optimization recommendations based on Pulumi infrastructure definitions.

\```bash
finfocus cost recommendations [flags]
\```

### Description

The `cost recommendations` command queries cloud provider APIs (via plugins) for cost optimization
suggestions based on your infrastructure. Recommendations include:

- **Rightsizing**: Downsize over-provisioned resources
- **Terminate**: Remove unused or idle resources
- **Purchase Commitments**: Reserved instances or savings plans
- **Adjust Requests**: Optimize CPU/memory requests for containers
- **Other**: Provider-specific recommendations

Recommendations are displayed in a terminal UI with interactive keyboard navigation, or output
as structured JSON/NDJSON for automation.

**When to use this command:**

- Before deploying infrastructure to identify optimization opportunities
- During regular cost reviews to find savings
- In CI/CD pipelines to block deployments with high-cost recommendations

See the [Recommendations Guide](../guides/recommendations.md) for detailed usage patterns.

### Flags

| Flag | Type | Default | Required | Description |
|------|------|---------|----------|-------------|
| `--pulumi-json` | string | - | Yes | Path to Pulumi plan JSON file (from `pulumi preview --json`) |
| `--filter` | string | `""` | No | Filter recommendations by action type (rightsize, terminate, etc.) |
| `--output` | string | `"table"` | No | Output format: `table` (interactive TUI), `json`, `ndjson` |
| `--no-color` | boolean | `false` | No | Disable color output (useful for CI/CD) |
| `--plain` | boolean | `false` | No | Use plain text rendering (no Unicode box drawing) |

**Global Flags:**

All commands support these global flags:

| Flag | Type | Default | Description |
|------|------|---------|----------|-------------|
| `--debug` | boolean | `false` | Enable debug logging to stderr |
| `--config` | string | `~/.finfocus/config.yaml` | Path to configuration file |

### Examples

#### Example 1: Basic usage with interactive table

Display recommendations in interactive terminal UI with keyboard navigation.

\```bash
finfocus cost recommendations --pulumi-json plan.json
\```

**Output:**

\```text
┌──────────────────────────────────────────────────────────────────────┐
│ RESOURCE              │ RECOMMENDATION  │ MONTHLY SAVINGS │ PRIORITY │
├──────────────────────────────────────────────────────────────────────┤
│ aws:ec2:Instance      │ Rightsize       │ $127.50         │ High     │
│ aws:rds:Instance      │ Terminate       │ $450.00         │ Medium   │
└──────────────────────────────────────────────────────────────────────┘

Press ↑/↓ to navigate, Enter for details, q to quit
\```

![Recommendations table with interactive navigation](../assets/screenshots/recommendations-table.png)

**Figure 1**: Interactive recommendations table with keyboard shortcuts

---

#### Example 2: Filter recommendations by action type

Show only "rightsize" recommendations for focused cost optimization.

\```bash
finfocus cost recommendations --pulumi-json plan.json --filter rightsize
\```

---

#### Example 3: JSON output for automation

Output recommendations as JSON for CI/CD pipeline processing.

\```bash
finfocus cost recommendations --pulumi-json plan.json --output json | jq '.recommendations[] | select(.priority == "high")'
\```

**Output:**

\```json
{
  "resource": "aws:ec2:Instance",
  "recommendation_type": "rightsize",
  "current_cost": 255.00,
  "recommended_cost": 127.50,
  "monthly_savings": 127.50,
  "priority": "high",
  "description": "Downsize from m5.2xlarge to m5.xlarge (CPU utilization < 30%)"
}
\```

---

#### Example 4: CI/CD integration with exit codes

Fail pipeline if high-priority recommendations exceed savings threshold.

\```bash
#!/bin/bash
SAVINGS=$(finfocus cost recommendations --pulumi-json plan.json --output json | \
  jq '[.recommendations[] | select(.priority == "high") | .monthly_savings] | add')

if (( $(echo "$SAVINGS > 100" | bc -l) )); then
  echo "High-priority recommendations exceed \$100/month savings threshold"
  exit 1
fi
\```

---

#### Example 5: Plain text for CI logs

Disable color and Unicode for clean CI/CD logs.

\```bash
finfocus cost recommendations --pulumi-json plan.json --no-color --plain
\```

### Exit Codes

| Code | Meaning |
|------|---------|
| `0` | Success (recommendations displayed) |
| `1` | Error (invalid flags, missing Pulumi JSON, plugin failure) |

**Note**: The command does NOT exit with error when recommendations exist. Use `--output json`
and parse results to fail CI/CD based on recommendation criteria.

### Environment Variables

| Variable | Description | Example |
|----------|-------------|---------|
| `FINFOCUS_OUTPUT_FORMAT` | Override default output format | `json` |
| `FINFOCUS_NO_COLOR` | Disable color output (set to any value) | `1` |
| `FINFOCUS_PLAIN` | Enable plain text rendering | `true` |

### Interactive Keyboard Shortcuts

When using `--output table` (default), these keyboard shortcuts are available:

| Key | Action |
|-----|--------|
| `↑` / `k` | Move selection up |
| `↓` / `j` | Move selection down |
| `Enter` | View recommendation details |
| `f` | Toggle filter menu |
| `s` | Sort by savings (descending) |
| `p` | Sort by priority |
| `q` / `Esc` | Quit |

### Related Commands

- [`finfocus cost projected`](#finfocus-cost-projected) - Estimate projected infrastructure costs
- [`finfocus cost actual`](#finfocus-cost-actual) - Fetch historical cost data
- [`finfocus plugin list`](#finfocus-plugin-list) - List installed cost data plugins

### See Also

- [Recommendations Guide](../guides/recommendations.md) - Complete tutorial for cost recommendations
- [Configuration Reference](./config-reference.md#recommendations) - Configuration options
- [Examples](../examples/config-recommendations/) - Example configurations and scripts
- [TUI Accessibility](../guides/accessibility.md) - Terminal display customization

---
```

---

## Flag Documentation Guidelines

### Flag Table Columns

1. **Flag**: Include `--` prefix, use backticks for code formatting
2. **Type**: `string`, `boolean`, `integer`, `number`, `duration`
3. **Default**: Use backticks for values, `-` for required flags with no default
4. **Required**: `Yes` or `No`
5. **Description**: Brief explanation, include allowed values for enums

### Flag Description Patterns

- **Boolean flags**: "Enable [feature]" or "Disable [feature]"
- **String flags**: "Path to [file]" or "[Description] (allowed: value1, value2)"
- **File paths**: Always specify what type of file (JSON, YAML, CSV)
- **Enums**: List allowed values in parentheses

### Examples

```markdown
| `--output` | string | `"table"` | No | Output format (allowed: table, json, ndjson) |
| `--debug` | boolean | `false` | No | Enable debug logging to stderr |
| `--filter` | string | `""` | No | Filter expression (e.g., 'type=rightsize') |
| `--pulumi-json` | string | - | Yes | Path to Pulumi plan JSON file |
```

---

## Example Documentation Guidelines

### Example Structure

1. **Heading**: `#### Example N: [Descriptive scenario name]`
2. **Explanation**: 1-2 sentences describing what example demonstrates
3. **Command**: Fenced bash code block with actual command
4. **Output**: (Optional) Fenced text block showing expected output
5. **Visual**: (Optional) Screenshot showing actual output
6. **Separator**: `---` between examples

### Example Categories

- **Example 1**: Basic usage (minimal flags, common case)
- **Example 2**: Filtering or options (show key flags)
- **Example 3**: Automation (JSON output, piping to other tools)
- **Example 4**: CI/CD integration (exit codes, error handling)
- **Example 5**: Accessibility (plain text, no color)

### Writing Examples

- Use realistic data (not "foo" or "example")
- Show actual commands that work
- Include context in comments when needed
- Demonstrate output when helpful
- Link to visual examples for TUI commands

---

## Exit Code Documentation

### When to Document Exit Codes

- Always document `0` (success) and `1` (general error)
- Document special exit codes (e.g., `2` for budget exceeded)
- Explain non-obvious exit behavior

### Exit Code Table Format

```markdown
| Code | Meaning |
|------|---------|
| `0` | Success |
| `1` | General error (invalid flags, missing files, plugin failure) |
| `2` | Budget threshold exceeded (when using budget alerts) |
```

---

## Environment Variable Documentation

### When to Document Environment Variables

- Variables that override flags or configuration
- Variables that affect output format or display
- Variables used for credentials or API keys

### Environment Variable Table Format

```markdown
| Variable | Description | Example |
|----------|-------------|---------|
| `FINFOCUS_OUTPUT_FORMAT` | Override default output format | `json` |
| `FINFOCUS_NO_COLOR` | Disable color output (any value) | `1` |
| `VANTAGE_API_KEY` | Vantage API key for actual costs | `vnt_abc123...` |
```

---

## Related Commands Section

**Purpose**: Help users discover related functionality.

**Format**: Bullet list with command name, anchor link, and brief description.

```markdown
### Related Commands

- [`finfocus cost projected`](#finfocus-cost-projected) - Estimate projected infrastructure costs
- [`finfocus plugin list`](#finfocus-plugin-list) - List installed cost data plugins
```

**Guidelines**:

- Link to command heading with anchor (lowercase, replace spaces with dashes)
- Provide 1-sentence description
- List 3-5 most related commands

---

## See Also Section

**Purpose**: Cross-link to guides, configuration, examples, and architecture docs.

**Format**: Bullet list grouped by category.

```markdown
### See Also

- [Recommendations Guide](../guides/recommendations.md) - Complete tutorial
- [Configuration Reference](./config-reference.md#recommendations) - Config options
- [Examples](../examples/config-recommendations/) - Example configurations
```

---

## Validation Checklist

Before adding CLI reference entry:

- [ ] Synopsis includes command syntax
- [ ] Description explains what, when, why
- [ ] Flag table includes: flag, type, default, required, description
- [ ] Minimum 3 examples (basic, intermediate, advanced)
- [ ] Exit codes documented (at least 0 and 1)
- [ ] Environment variables listed (if applicable)
- [ ] Related commands section included
- [ ] See Also links to guides, reference, examples
- [ ] All code blocks have language tags (bash, json, text)
- [ ] All commands tested against actual CLI
- [ ] Visual examples included for TUI commands
- [ ] File ends with newline

---

**Template Version**: 1.0.0
**Last Updated**: 2026-01-20
**Usage**: Add to `docs/reference/cli-commands.md`
