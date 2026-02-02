---
layout: default
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
export PULUMICOST_PLAIN=1
# or
export NO_COLOR=1
```

---

## Configuration Reference

### Command Line Flags

These flags can be added to any `finfocus` command that produces output.

| Flag              | Description            | Effect                                                                       |
| ----------------- | ---------------------- | ---------------------------------------------------------------------------- |
| `--no-color`      | Disable colored output | Removes ANSI color codes.                                                    |
| `--high-contrast` | Enable high contrast   | Uses strictly black/white/bold colors for maximum visibility.                |
| `--plain`         | Enable plain text mode | Removes colors, borders, and interactive elements. Ideal for screen readers. |

### Environment Variables

Environment variables allow you to set preferences globally without typing flags every time.

| Variable                   | Value         | Description                                                |
| -------------------------- | ------------- | ---------------------------------------------------------- |
| `NO_COLOR`                 | `1` or `true` | Standard no-color variable. See [no-color.org][no-color].  |
| `PULUMICOST_HIGH_CONTRAST` | `1` or `true` | Forces high contrast mode.                                 |
| `PULUMICOST_PLAIN`         | `1` or `true` | Forces plain text mode.                                    |

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

![Budget display in plain text mode](../assets/screenshots/budget-plain-mode.png)

**Figure 1**: Clean output without boxes or colors.

### Example 2: No Color Mode

**Use Case**: Terminals that don't support color or user preference.

**Command:**

```bash
finfocus cost projected --pulumi-json plan.json --no-color
```

**Explanation:**

This retains the layout (tables, borders) but strips all color codes. This is useful if you want structure but no color distraction.

---

## Troubleshooting

### Issue: Colors still showing despite settings

**Symptoms:**

- You set `NO_COLOR=1` but still see colors.

**Cause:**

- Some CI environments force color.
- Flag usage might override environment variable.

**Solution:**

Ensure you aren't passing `--color=always` or similar flags (if applicable). Verify environment variable is exported:

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

**Last Updated**: 2026-01-20
**FinFocus Version**: v0.1.0
**Feedback**: [Open an issue](https://github.com/rshade/finfocus/issues/new) to improve accessibility
