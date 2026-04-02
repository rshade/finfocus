---
title: Frequently Asked Questions
description: Common questions and answers about FinFocus
---

This FAQ addresses common questions about FinFocus installation, usage, plugins,
and troubleshooting. For detailed guidance, see the linked documentation in each
section. Jump to: [Installation](#installation--setup) |
[Usage](#usage) | [Plugins](#plugins) | [Analyzer](#pulumi-analyzer) |
[Troubleshooting](#troubleshooting) | [Data & Privacy](#data--privacy) |
[Performance](#performance) | [Support](#support)

## Installation & Setup

### Q: How do I install FinFocus?

A: The fastest way is the install script:

```bash
curl -fsSL https://raw.githubusercontent.com/rshade/finfocus/main/scripts/install.sh | sh
```

For full options see the [Installation Guide](../getting-started/installation.md).

### Q: Does FinFocus require a specific version of Pulumi?

A: FinFocus works with Pulumi 3.0+. We recommend the latest version.

### Q: Can I use FinFocus with my cloud provider?

A: For projected costs, yes — works with all Pulumi-supported clouds via the
`aws-public` plugin or local pricing specs.
For actual costs, a plugin with billing API access is required (e.g., Vantage,
AWS Cost Explorer).

## Usage

### Q: What is the simplest way to see my costs?

A: Navigate into any Pulumi project directory and run:

```bash
finfocus
```

FinFocus auto-detects your project and stack, exports the current state, runs
`pulumi preview --json`, and opens an interactive cost dashboard. No flags needed.
`finfocus overview` and `finfocus ov` are aliases for the same command.

### Q: Do I need to export the Pulumi state and plan manually?

A: No — auto-detection handles this when you run `finfocus` from inside a Pulumi
project directory. Use `--pulumi-state` and `--pulumi-json` only when you want
to provide pre-exported files (e.g., in CI/CD where you control the export step).

### Q: How do I select a specific Pulumi stack?

A: Pass `--stack`:

```bash
finfocus overview --stack production
```

### Q: Why are some resources showing $0 cost?

A: Some resources don't have pricing data available. Common cases:

- S3 buckets (storage costs depend on actual data stored)
- VPCs, subnets, IAM roles (no direct compute/storage cost)
- Resources not yet covered by an installed plugin or local spec

### Q: How accurate are the projected costs?

A: Accuracy depends on the plugin or spec used. The `aws-public` plugin uses
AWS public pricing and is accurate for on-demand instance types. Actual costs
may vary based on reserved instances, savings plans, and data transfer.

### Q: Can I filter by custom tags?

A: Yes. Use `--filter "tag:key=value"` with any cost command:

```bash
finfocus overview --filter tag:env=production --plain --yes
finfocus cost projected --filter tag:team=platform
```

### Q: How do I reset configuration?

A: FinFocus uses two config locations:

- **Global:** `rm -rf ~/.finfocus`
- **Project-local:** `rm -rf $YOUR_PULUMI_PROJECT/.finfocus`

Project-local config takes precedence over global when running inside a Pulumi project.

## Plugins

### Q: What plugins are available?

A: The main plugins today:

| Plugin | Projected Costs | Actual Costs | Notes |
|--------|----------------|--------------|-------|
| `aws-public` | ✓ | ✓ | AWS public pricing API |
| Vantage | — | ✓ | Requires Vantage account |
| Kubecost | — | ✓ | Planned |

### Q: Do I need a plugin?

A: For projected costs: No — local pricing specs work without plugins.
For actual costs: Yes — a plugin with billing API access is required.

### Q: How do I install a plugin?

A: Use the plugin install command:

```bash
finfocus plugin install aws-public
finfocus plugin list
```

See [Plugin Documentation](../plugins/) for per-plugin setup.

## Pulumi Analyzer

### Q: What is the Pulumi Analyzer?

A: The analyzer is a FinFocus mode that shows cost estimates directly inside
`pulumi preview` output — no separate command needed. It runs as a Pulumi
Policy Pack.

### Q: How do I set up the analyzer?

A: Install the analyzer plugin, then add it to your PATH:

```bash
finfocus analyzer install
export PATH="${HOME}/.pulumi/plugins/analyzer-finfocus-v$(finfocus --version | cut -d' ' -f2):${PATH}"
pulumi preview --policy-pack ~/.finfocus/analyzer
```

See the [Analyzer Setup Guide](../getting-started/analyzer-setup.md) for the
full walkthrough.

### Q: Does the analyzer block deployments?

A: No. All cost diagnostics are `ADVISORY` severity — they appear as
informational output and never prevent `pulumi up` from running.

## Troubleshooting

### Q: "Plugin not found" error

A: Install the plugin first:

```bash
finfocus plugin install aws-public
finfocus plugin list   # confirm it appears
finfocus plugin validate
```

### Q: "No cost data available"

A: Check in order:

1. Is a plugin installed? (`finfocus plugin list`)
2. Does the plugin support this resource type? (`finfocus plugin validate`)
3. Does the plugin have credentials configured?
4. Is the resource type covered by a local spec in `~/.finfocus/specs/`?

### Q: Cost data is slow to load

A: FinFocus caches results in a local BoltDB database
(`~/.finfocus/cache/cache.db`). On subsequent runs, cached resources load
instantly. To force a fresh fetch, set `--cache-ttl 0`:

```bash
finfocus overview --cache-ttl 0
finfocus cost projected --cache-ttl 0
```

### Q: The overview TUI doesn't launch (terminal compatibility)

A: Use `--plain` to force non-interactive output:

```bash
finfocus overview --plain --yes
```

## Data & Privacy

### Q: Does FinFocus send my data anywhere?

A: Only to the plugins you configure (e.g., a Vantage or Kubecost endpoint).
Local specs and the cache are entirely on your machine. No telemetry is sent.

### Q: Is my infrastructure data secure?

A: Pulumi state JSON contains resource details — treat it as sensitive. FinFocus
is a local CLI tool; all data stays on your machine unless explicitly sent to a
plugin's external API.

## Performance

### Q: How long does cost calculation take?

A: First run: 1–10 seconds depending on stack size and plugin API latency.
Subsequent runs: sub-second for cached results.

### Q: Can I use FinFocus with large infrastructure (1000+ resources)?

A: Yes. The overview enriches resources concurrently and paginates the TUI
automatically. For very large stacks, use `--output ndjson` for streaming output
or `--filter` to narrow scope.

## Support

### Q: Where can I get help?

A: See these resources:

- [Troubleshooting Guide](troubleshooting.md)
- [GitHub Issues](https://github.com/rshade/finfocus/issues)
- [GitHub Discussions](https://github.com/rshade/finfocus/discussions)

### Q: How do I report a bug?

A: [Open a GitHub Issue](https://github.com/rshade/finfocus/issues/new)

### Q: Can I contribute?

A: Yes! See [Contributing Guide](contributing.md).
