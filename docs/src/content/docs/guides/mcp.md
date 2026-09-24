---
title: Use finfocus from an AI assistant (MCP)
description: Run finfocus as a Model Context Protocol server so Claude Code, Claude Desktop, and other MCP clients can query projected and actual cloud costs.
---

## Overview

FinFocus has a built-in [Model Context Protocol](https://modelcontextprotocol.io)
(MCP) server. An MCP client, such as Claude Code or Claude Desktop, starts
finfocus as a subprocess. It can then call the finfocus commands as tools.
For example, it can calculate the projected cost of a Pulumi plan, fetch actual
spend, or list installed plugins. You do not need to install a separate server.

**Target Audience**: Developers and platform engineers using AI assistants

**Prerequisites**:

- [FinFocus CLI installed](../getting-started/installation.md) and on your `PATH`
- An MCP client (Claude Code, Claude Desktop, or any MCP-compatible tool)
- At least one cost plugin installed for non-zero results (`finfocus plugin list`)

**Estimated Time**: 5 minutes

---

## Entry points

The server has two entry points. They serve the same tools.

| Command                  | Use it when                                                   |
| ------------------------ | ------------------------------------------------------------- |
| `finfocus --mcp`         | The short form for MCP client configs. Serves over stdio      |
| `finfocus mcp-server`    | You need transport options (`--transport`, `--addr`)          |

```bash
# stdio (what MCP clients launch)
finfocus --mcp
finfocus mcp-server

# Streamable HTTP, loopback only by default
finfocus mcp-server --transport=http --addr=127.0.0.1:8080

# Non-loopback HTTP requires an explicit opt-in
finfocus mcp-server --transport=http --addr=0.0.0.0:8080 --allow-non-loopback
```

The server writes only MCP protocol messages to stdout. Logs go to the
finfocus log file (`~/.finfocus/logs/finfocus.log` by default). Other
diagnostics go to stderr.

## Claude Code

Register finfocus as an MCP server:

```bash
claude mcp add finfocus -- finfocus --mcp
```

The equivalent command with the subcommand entry point:

```bash
claude mcp add finfocus -- finfocus mcp-server
```

Run `claude mcp list` to confirm that the server connects. Then ask something
like "What is the projected monthly cost of `plan.json`?"

## Claude Desktop

Add finfocus to `claude_desktop_config.json`. On macOS the file is in
`~/Library/Application Support/Claude/`. On Windows it is in
`%APPDATA%\Claude\`. Claude Desktop does not use your shell `PATH`, so set
`command` to the absolute path of the finfocus binary (`which finfocus`).

```json
{
  "mcpServers": {
    "finfocus": {
      "command": "/path/to/finfocus",
      "args": ["--mcp"]
    }
  }
}
```

With the subcommand entry point:

```json
{
  "mcpServers": {
    "finfocus": {
      "command": "/path/to/finfocus",
      "args": ["mcp-server"]
    }
  }
}
```

Add an `"env"` object to point the server at a different finfocus home, for
example `"env": { "FINFOCUS_HOME": "/path/to/.finfocus" }`. Restart Claude
Desktop after you edit the file.

## How tools map to commands

Each tool is a finfocus command. The tool name is the command path joined with
`-`, and the tool arguments are the command flags.

| Tool                            | CLI equivalent                           |
| ------------------------------- | ---------------------------------------- |
| `finfocus-cost-projected`       | `finfocus cost projected`                |
| `finfocus-cost-actual`          | `finfocus cost actual`                   |
| `finfocus-cost-estimate`        | `finfocus cost estimate`                 |
| `finfocus-cost-recommendations` | `finfocus cost recommendations`          |
| `finfocus-overview`             | `finfocus overview`                      |
| `finfocus-plugin-list`          | `finfocus plugin list`                   |
| `finfocus-plugin-validate`      | `finfocus plugin validate`               |
| `finfocus-config-list`          | `finfocus config list`                   |
| `finfocus-config-validate`      | `finfocus config validate`               |
| `finfocus-config-routes-list`   | `finfocus config routes list`            |
| `finfocus-config-init`          | `finfocus config init`                   |
| `finfocus-analyzer-check`       | `finfocus analyzer check`                |
| `finfocus-analyzer-install`     | `finfocus analyzer install`              |
| `finfocus-analyzer-uninstall`   | `finfocus analyzer uninstall`            |

For example, this tool call runs
`finfocus cost projected --pulumi-json plan.json`:

```json
{
  "name": "finfocus-cost-projected",
  "arguments": { "pulumi-json": "/absolute/path/to/plan.json" }
}
```

The server runs in the directory where the client started it. Pass absolute
paths for file arguments such as `pulumi-json`.

### Output format

Tool calls return JSON by default. The server passes `--format json` on every
call, and each command then emits its JSON output. An explicit `output`
argument takes precedence. Pass `"output": "table"` to get the human table
instead. The same rule applies on the command line: `finfocus cost projected
--format json`, or setting `AGENT_MODE=1`, prints JSON unless you also pass
`--output`. Piping output alone does not switch it to JSON.

### Safety arguments

Every tool also accepts the agent-safety arguments:

- `dry-run`: report what a mutating command would do without changing
  anything. `finfocus-config-init` with `"dry-run": true` returns
  `{"action": "would_create", ...}` and writes no file.
- `yes`: approve a confirmation-gated operation. The server never approves on
  the client's behalf. Today the confirmation-gated commands, such as
  `cost recommendations dismiss <id>`, take positional arguments, so they are
  CLI-only.
- `idempotency-key`: a retry-deduplication key. The server generates one when
  you omit it.

A failed call returns the finfocus error envelope as JSON, with `isError`
set. For example: `{"error_code": "validation_error", "message": "..."}`.

## Commands that are not tools

These commands are never exposed as tools:

| Command            | Reason                                                              |
| ------------------ | ------------------------------------------------------------------- |
| `analyzer serve`   | Long-running Pulumi handshake that would block the server           |
| `setup`            | Interactive first-run wizard                                        |
| `plugin init`      | Scaffolds a plugin project; a developer action, not a query         |
| Positional-arg commands | For example `plugin install <name>`, `config set <key> <value>`, and `cost recommendations dismiss <id>`. MCP tool calls can pass flags only |

The tool list still contains the `finfocus` root tool, the `finfocus-help`
tool, and the command-group tools (`finfocus-cost`, `finfocus-plugin`, and
others). These return usage text or an error. Calling the `finfocus` root tool
returns a `validation_error`. A future release removes them from the tool
list.

## Migrating from finfocus-mcp

The standalone [finfocus-mcp](https://github.com/rshade/finfocus-mcp) server
(formerly `pulumicost-mcp`) is superseded by the built-in server. It had
hand-written tools that called the CLI. The built-in server exposes the CLI
directly, so it stays in step with each finfocus release. Replace the old
server entry in your MCP client configuration with `finfocus --mcp`. The old
tools map to built-in tools as follows:

| Old finfocus-mcp tool                | Built-in tool                                                                     |
| ------------------------------------ | --------------------------------------------------------------------------------- |
| `analyze_projected_cost`             | `finfocus-cost-projected`                                                         |
| `get_actual_cost`                    | `finfocus-cost-actual`                                                            |
| `analyze_resource_cost`              | `finfocus-cost-projected` with `filter`                                           |
| `query_cost_by_tags`                 | `finfocus-cost-actual` with `filter` (`tag:k=v`) or `group-by` (`tag:k=v`)        |
| `analyze_stack`                      | `finfocus-overview`                                                               |
| `compare_costs`                      | Partial: `finfocus-overview` (projected versus actual delta)                      |
| `list_plugins`                       | `finfocus-plugin-list`                                                            |
| `get_plugin_info`                    | `finfocus plugin inspect` (CLI only: it takes positional arguments)               |
| `validate_plugin` / `health_check`   | `finfocus-plugin-validate`                                                        |
| `get_recommendations`                | `finfocus-cost-recommendations`                                                   |
| `track_budget`                       | `budgets` in `finfocus-overview` JSON; `finfocus-cost-projected` with `exit-on-threshold` |
| `detect_anomalies`                   | Partial: `CostDrift` in `finfocus-overview` JSON                                  |
| `forecast_costs`                     | Partial: `ExtrapolatedMonthly` in `finfocus-overview` JSON                        |

## Troubleshooting

**The server exits immediately with a version error.** The MCP handshake
requires a real build version. Use a released binary or `make build`. A plain
`go build` without the version `ldflags` reports a placeholder version.

**Cost tools return `$0.00`.** No installed plugin prices the resources. Run
`finfocus plugin list` and install a plugin (`finfocus plugin install <name>`).

**A tool reports `no Pulumi project found`.** The tool auto-detects the Pulumi
project from the server's working directory. Pass an explicit input such as
`pulumi-json` or `pulumi-state` with an absolute path.

**To inspect the server interactively**, run `make inspect` from a finfocus
checkout. It opens the MCP Inspector against `bin/finfocus --mcp`.
