---
title: Accessibility Features Guide
description: Configure FinFocus for optimal readability with high-contrast, no-color, and plain text modes.
---

## Overview

FinFocus is designed to be accessible and usable in diverse environments, including those with limited color support,
for users with visual impairments, or for automated processing.

This guide explains how to configure display modes, control color output, and use environment variables for persistent
accessibility settings.

**Target Audience**: All Users

**Prerequisites**:

- [FinFocus CLI installed](../getting-started/installation.md)

**Learning Objectives**:

- Disable color output for compatibility
- Enable high-contrast mode for visibility
- Use plain text mode for screen readers or logging
- Persist settings via environment variables

**Estimated Time**: 5 minutes

---

## Quick Start

Configure your preferred display mode instantly.

### Step 1: Try Plain Mode

If you use a screen reader or prefer minimal styling:

```bash
finfocus cost projected --pulumi-json plan.json --plain
```

### Step 2: Try High Contrast

If you need higher visibility:

```bash
finfocus cost projected --pulumi-json plan.json --high-contrast
```

### Step 3: Set Persistent Defaults

Set environment variables in your shell profile (e.g., `~/.bashrc` or `~/.zshrc`) to make these settings permanent:

```bash
export FINFOCUS_PLAIN=1
# or
export NO_COLOR=1
```

---

## Configuration Reference

### Command Line Flags

These flags are accepted by `overview`, `cost projected`, `cost actual`, and `cost recommendations`.

| Flag              | Description            | Effect                                                                       |
| ----------------- | ---------------------- | ---------------------------------------------------------------------------- |
| `--no-color`      | Disable colored output | Selects plain text. Wins over `--color` and `--high-contrast`.               |
| `--plain`         | Enable plain text mode | Removes colors, borders, and interactive elements. Ideal for screen readers. |
| `--color`         | Force colored output   | Styles output when stdout is not a terminal. `--force-color` is the same switch. |
| `--high-contrast` | Enable high contrast   | Brighter budget colors: OK 46, warning 226, critical 196, header 231.        |

`--high-contrast` recolors the budget box, including the provider, tag, and type
breakdown. `overview` and `cost recommendations` accept it and ignore it.

`cost projected` prints its cost diff as a plain text table in every mode, so
there the four flags change only the budget box printed after it. `cost actual`
and `cost recommendations` follow the flags for their tables as well.

### Environment Variables

Environment variables allow you to set preferences globally without typing flags every time.

| Variable                   | Value         | Description                                                |
| -------------------------- | ------------- | ---------------------------------------------------------- |
| `NO_COLOR`                 | any non-empty | Standard no-color variable. See [no-color.org][no-color].  |
| `FINFOCUS_PLAIN`           | `1` or `true` | Forces plain text mode.                                    |
| `FORCE_COLOR`              | any non-empty value | Forces colored output ([force-color.org](https://force-color.org)). `0` and `false` leave it off, as in the supports-color package. |
| `FINFOCUS_HIGH_CONTRAST`   | `1` or `true` | Forces high contrast mode. Invalid values are ignored.     |

An explicit flag wins over the environment. `--plain` and `--no-color` win over
`--color`, `--force-color`, and `--high-contrast`. `FINFOCUS_PLAIN` and `NO_COLOR`
win over `FORCE_COLOR` and `FINFOCUS_HIGH_CONTRAST` when those color modes were
not set by a flag.

`cost history view` accepts `--plain`, and `FINFOCUS_PLAIN` selects its plain
chart when no output format was asked for. An explicit `--output json`, a
`--format json`, or `AGENT_MODE=json` keeps the JSON output, so a server or
script started with `FINFOCUS_PLAIN=1` still gets JSON when it asks for it. An
explicit `--plain` flag still wins. `--plain` is not a parent `cost` flag,
because `cost history view` already defines it.

An explicit `--color` or `--high-contrast` wins over `NO_COLOR`, so the whole
output, table and budget box, is styled together.

[no-color]: https://no-color.org

---

## Examples

### Example 1: Plain Text Mode

**Use Case**: Screen readers or log file capture where ANSI codes cause issues.

**Command:**

```bash
finfocus cost projected --pulumi-json plan.json --plain
```

**Output:**

```text
Budget: $500.00 (75% used)
$375.00 / $500.00

RESOURCE                          ADAPTER     MONTHLY   CURRENCY  NOTES
aws:ec2/instance:Instance         aws-spec    $375.00   USD       t3.xlarge
```

![Budget display in plain text mode](/finfocus/screenshots/budget-plain-mode.png)

**Figure 1**: Clean output without boxes or colors.

### Example 2: No Color Mode

**Use Case**: Terminals that don't support color or user preference.

**Command:**

```bash
finfocus cost projected --pulumi-json plan.json --no-color
```

**Explanation:**

`--no-color` selects the same plain text path as `--plain`. Borders and the
interactive TUI are omitted, and status text keeps a bracketed label such as
`[OK]` or `[WARNING]`.

---

## Troubleshooting

### Issue: Colors still showing despite settings

**Symptoms:**

- You set `NO_COLOR=1` but still see colors.

**Cause:**

- Some CI environments force color.
- Flag usage might override environment variable.

**Solution:**

Ensure you aren't passing `--color` or `--force-color`. Verify the environment variable is exported:

```bash
echo $NO_COLOR
```

### Issue: TUI navigation broken in plain mode

**Symptoms:**

- Interactive commands like `recommendations` don't respond to keys.

**Cause:**

- Plain mode intentionally disables complex interactive elements for compatibility.

**Solution:**

If you need interaction, disable plain mode. If you need accessible interaction, please
[open an issue](https://github.com/rshade/finfocus/issues) describing your needs.

---

## See Also

**Related Guides:**

- [User Guide](./user-guide.md) - General usage

**CLI Reference:**

- [CLI Flags](../reference/cli-flags.md) - Detailed flag reference

---

**Last Updated**: 2026-10-03
**FinFocus Version**: v0.3.0
**Feedback**: [Open an issue](https://github.com/rshade/finfocus/issues/new) to improve accessibility
