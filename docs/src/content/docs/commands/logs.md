---
title: logs Command
description: Print a snapshot of the finfocus log file with source, level, and text filters
---

Print the last entries of the finfocus log file, oldest first. The command
reads the file once and exits. It never modifies the file, never starts a
plugin, and works whether or not the dashboard Logs view is enabled.

It uses the same parsing, redaction, filters, and display text as the
dashboard Logs view, so a line looks the same in both places. It reads both
JSON lines and the default text format (`logging.format: text`) that ordinary
runs write.

> **Note:** This is a snapshot, not a follow mode. Run it again, or open the
> dashboard Logs view, to see newer entries.

## Usage

```bash
finfocus logs [options]
```

## Options

| Flag | Description | Default |
|------|-------------|---------|
| `--file` | Log file to read | The configured `logging.file` (`~/.finfocus/logs/finfocus.log`) |
| `--tail` | Number of most recent matching entries to show; `0` shows all | `200` |
| `--source` | Only entries from this source, `core` or a plugin name. Repeatable | All sources |
| `--level` | Minimum level: `trace`, `debug`, `info`, `warn`, `error`, `fatal`, `panic` | All levels |
| `--grep` | Only entries whose message or field values contain this text, ignoring case | No text filter |
| `--output` | Output format: `table`, `json`, or `ndjson` | `table` |
| `--no-color`, `--plain` | Print the table without colors | Color on a terminal |
| `--color` | Force colors when stdout is not a terminal | Off |

Filters combine: an entry must match a listed source, be at or above the
level, and contain the text. `--tail` applies after the filters, so
`--level error --tail 20` shows the last 20 errors. `--grep` is a plain
substring match, not a regular expression.

`NO_COLOR` turns colors off. Level names are always printed as text, so the
table is readable without color.

## Output

The table has one line per entry:

```text
10:00:02.123 INFO  [overview] overview started resources=12
10:00:03.000 WARN  [kubernetes] slow node
10:00:04.000 ERROR [core] plugin call failed api_token=[redacted]
```

Each line shows the local time, the level, the source in brackets, the
message, and the remaining fields as `key=value`. A line that is not a log
record, such as plain text a plugin printed, is shown at level `INFO` with the
source `unknown`.

`--output json` prints one array of entries and `--output ndjson` prints one
entry per line. Both use the entry shape of the dashboard stream, including
the `display` strings. With no matching entries, `json` prints `[]` and
`ndjson` prints nothing.

Credential values and Pulumi secrets are replaced with `[redacted]` in every
format. Log lines can still contain resource names and file paths.

## Exit codes

| Code | Meaning |
|------|---------|
| `0` | Success, including no matching entries |
| `1` | The default log file does not exist or cannot be read. The message names the path |
| `2` | A bad flag or value, an unknown `--output`, or a missing or unreadable `--file` |

## Examples

```bash
# Last 200 entries of the configured log file
finfocus logs

# Everything one plugin logged, as JSON
finfocus logs --tail 0 --source aws-public --output json

# The last 20 warnings and errors that mention a region
finfocus logs --level warn --grep us-east-1 --tail 20

# Core and one plugin together, for another tool to read
finfocus logs --source core --source kubernetes --output ndjson

# Read a file copied from another machine
finfocus logs --file ./finfocus.log
```

The command is also available to agents as the `finfocus-logs` MCP tool.
