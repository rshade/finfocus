# TUI Documentation Feature - Research Findings

**Spec:** specs/119-tui-docs-update/spec.md
**Date:** 2026-01-20
**Researcher:** Claude Code

## Executive Summary

This research document provides comprehensive findings on the FinFocus documentation infrastructure, Jekyll/GitHub Pages conventions, markdown standards, and best practices for creating technical documentation. The findings will inform the creation of new TUI-focused documentation.

---

## 1. Jekyll & GitHub Pages Conventions

### 1.1 Jekyll Configuration Analysis

**File:** `/mnt/c/GitHub/go/src/worktrees/fix-errors/docs/_config.yml`

**Key Configuration Settings:**

| Setting | Value | Purpose |
| ------- | ----- | ------- |
| `baseurl` | `/finfocus` | GitHub Pages project URL path |
| `theme` | `jekyll-theme-minimal` | Minimal theme with light skin |
| `markdown` | `kramdown` | Markdown processor with GFM input |
| `highlighter` | `rouge` | Syntax highlighting engine |
| `permalink` | `/docs/:title/` | URL structure for pages |

**Collections Enabled:**

```yaml
collections:
  guides:
    output: true
    permalink: /:path/
  architecture:
    output: true
    permalink: /:path/
  plugins:
    output: true
    permalink: /:path/
```

### 1.2 Front Matter Patterns

**Required Fields (from docs/_config.yml defaults):**

```yaml
---
layout: default
title: Page Title
description: Brief description for SEO and previews
---
```

**Category-Specific Defaults:**

Documents in specific directories inherit additional front matter:

- `docs/guides/*.md` → `category: "guides"`
- `docs/architecture/*.md` → `category: "architecture"`
- `docs/plugins/*.md` → `category: "plugins"`
- `docs/reference/*.md` → `category: "reference"`

**Real-World Examples:**

**User Guide Front Matter:**

```yaml
---
layout: default
title: User Guide
description: Complete guide for end users - install, configure, and use FinFocus
---
```

**Developer Guide Front Matter:**

```yaml
---
layout: default
title: Developer Guide
description: Complete guide for engineers - extend FinFocus and build plugins
---
```

**Architect Guide Front Matter:**

```yaml
---
layout: default
title: Architect Guide
description: System design and integration guide for software architects
---
```

### 1.3 Jekyll Plugins in Use

```yaml
plugins:
  - jekyll-include-cache
  - jekyll-sitemap
```

**Note:** Project does NOT use `jekyll-seo-tag` plugin. Manual SEO tags required if needed.

### 1.4 Front Matter Best Practices

**✅ DO:**

- Use `layout: default` for all documentation pages
- Provide concise, actionable `description` (50-160 characters for SEO)
- Use sentence case for `title` (not Title Case)
- Keep front matter minimal - let defaults handle category assignment

**❌ DON'T:**

- Use `{% seo %}` tags (plugin not installed)
- Override `permalink` unless absolutely necessary
- Add `category` manually (inherited from directory structure)
- Use complex front matter with custom variables

---

## 2. Markdown Style Guide

### 2.1 Markdownlint Configuration

**Primary Config:** `/mnt/c/GitHub/go/src/worktrees/fix-errors/docs/.markdownlint-cli2.jsonc`

**Key Rules:**

| Rule | Setting | Meaning |
| ---- | ------- | ------- |
| MD003 | `style: "atx"` | Use `#` for headings, not underline style |
| MD004 | `style: "dash"` | Use `-` for unordered lists |
| MD013 | `line_length: 120` | Max line length 120 chars (except code/tables) |
| MD022 | `lines_above: 1, lines_below: 1` | Blank lines around headings |
| MD024 | `siblings_only: true` | Allow duplicate headings at different levels |
| MD025 | `front_matter_title` | Single H1 allowed if in front matter |
| MD033 | Allowed HTML elements | Specific HTML tags permitted |
| MD040 | Enforced | Fenced code blocks must have language tags |
| MD046 | `style: "fenced"` | Use fenced code blocks (```) not indented |
| MD047 | `true` | Files must end with newline |

**Root Config:** `/mnt/c/GitHub/go/src/worktrees/fix-errors/.markdownlint.json`

```json
{
  "MD013": false  // Line length check disabled at root level
}
```

### 2.2 Heading Hierarchy Analysis

**From User Guide and Developer Guide:**

**Standard Hierarchy:**

```markdown
# H1 - Page Title (DO NOT USE - provided by Jekyll layout)

## H2 - Major Sections
### H3 - Subsections
#### H4 - Sub-subsections (rare)
```

**Observed Pattern:**

```markdown
---
layout: default
title: User Guide  # This becomes the H1
---

Content introduction paragraph.

## Table of Contents  # H2 for TOC

## What is FinFocus?  # H2 for major sections

### Key Features  # H3 for subsections
```

**✅ Best Practices:**

1. **Never use H1 (`#`) in content** - `title` in front matter provides the H1
2. **Start content with H2 (`##`)** for major sections
3. **Use H3 (`###`)** for subsections
4. **Rarely go deeper than H4** - restructure if needed
5. **Use descriptive headings** - "What is FinFocus?" not "Introduction"

### 2.3 Code Block Language Tags

**MD040 Requirement:** All fenced code blocks MUST specify language.

**Common Languages Used:**

```markdown
```bash
finfocus cost projected --pulumi-json plan.json
\```

```go
func main() {
    // Go code
}
\```

```yaml
# YAML configuration
key: value
\```

```json
{
  "json": "example"
}
\```

```text
# Plain text output
RESOURCE                TYPE              MONTHLY
\```
```

**Special Cases:**

- Console output → `text`
- Shell commands → `bash`
- Multi-line text → `text`
- Diagrams (ASCII) → `text`

### 2.4 Table Formatting

**Standard Markdown Tables:**

```markdown
| Column 1 | Column 2 | Column 3 |
| -------- | -------- | -------- |
| Value 1  | Value 2  | Value 3  |
```

**Real-World Example (from User Guide):**

```markdown
| Field         | Purpose             | Example                      |
| ------------- | ------------------- | ---------------------------- |
| `trace_id`    | Request correlation | "01HQ7X2J3K4M5N6P7Q8R9S0T1U" |
| `component`   | Package identifier  | "cli", "engine", "registry"  |
| `operation`   | Current operation   | "get_projected_cost"         |
| `duration_ms` | Operation timing    | 245                          |
```

**Best Practices:**

- Use backticks for code/technical terms in cells
- Align columns with dashes for readability
- Use `|` alignment markers (`:---`, `:---:`, `---:`) sparingly
- Keep cell content concise

### 2.5 Permitted HTML Elements

**MD033 Allowed Elements:**

```json
[
  "a", "abbr", "br", "div", "img", "kbd", "span", "strong",
  "sub", "sup", "table", "thead", "tbody", "tr", "th", "td",
  "details", "summary"
]
```

**When to Use HTML:**

- `<details>` and `<summary>` for collapsible sections
- `<kbd>` for keyboard shortcuts
- `<img>` with alt text for images
- `<br>` for forced line breaks (use sparingly)

---

## 3. Cross-Linking Strategy

### 3.1 Relative Link Patterns

**Analysis of docs/TABLE-OF-CONTENTS.md and docs/guides/user-guide.md:**

**Link Format Rules:**

1. **Relative paths from current document**
2. **Use `.md` extension** (Jekyll converts to HTML)
3. **Include directory structure** for clarity

**Examples from User Guide:**

```markdown
# From docs/guides/user-guide.md

## Links to Other Guides
- [Analyzer Setup Guide](../getting-started/analyzer-setup.md)
- [FAQ](../support/faq.md)
- [Troubleshooting](../support/troubleshooting.md)

## Links to Reference Docs
- [Complete CLI Commands](../reference/cli-commands.md)

## Links to Examples
- [Practical Examples](../getting-started/examples/)

## Links to Plugin Docs
- [Vantage Plugin Setup](../plugins/vantage/setup.md)
```

**Link Type Patterns:**

| Link Target | Pattern | Example |
| ----------- | ------- | ------- |
| Same directory | `[Text](filename.md)` | `[FAQ](faq.md)` |
| Parent directory | `[Text](../directory/file.md)` | `[User Guide](../guides/user-guide.md)` |
| Subdirectory | `[Text](subdir/file.md)` | `[Examples](examples/vantage-setup.md)` |
| Anchor link | `[Text](file.md#section)` | `[Installation](#installation)` |

### 3.2 "See Also" Section Patterns

**From User Guide (lines 660-676):**

```markdown
---

## Next Steps

- **Quick Start:** [5-Minute Quickstart](../getting-started/quickstart.md)
- **Installation:** [Detailed Installation Guide](../getting-started/installation.md)
- **Vantage Setup:** [Setting up Vantage Plugin](../plugins/vantage/setup.md)
- **CLI Reference:** [Complete CLI Commands](../reference/cli-commands.md)
- **Examples:** [Practical Examples](../getting-started/examples/)

---
```

**Pattern:**

- Use `## Next Steps` heading
- Bold category names followed by colon
- Descriptive link text
- Horizontal rules (`---`) before and after

### 3.3 Inline Cross-Reference Patterns

**From User Guide:**

```markdown
## Zero-Click Cost Estimation (Analyzer)

FinFocus can integrate directly with the Pulumi CLI as an Analyzer...

For detailed setup instructions, refer to the
[Analyzer Setup Guide](../getting-started/analyzer-setup.md).
```

**Pattern:**

- Introduce concept/feature
- Reference detailed documentation inline
- Use relative links to specific guides

### 3.4 Table of Contents Patterns

**From TABLE-OF-CONTENTS.md:**

**Audience-Based Navigation:**

```markdown
### 👤 For End Users

1. Start: [5-Minute Quickstart](getting-started/quickstart.md)
2. Install: [Installation Guide](getting-started/installation.md)
3. Learn: [User Guide](guides/user-guide.md)
4. Command Reference: [CLI Commands](reference/cli-commands.md)
5. Get Help: [FAQ](support/faq.md) | [Troubleshooting](support/troubleshooting.md)
```

**Best Practices:**

- Group by user persona/role
- Numbered workflow order
- Multiple links separated by `|` for alternatives
- Emoji icons for visual scanning (optional, project uses them)

---

## 4. Visual Example Formats

### 4.1 Directory Structure Analysis

**Assets Location:** `/mnt/c/GitHub/go/src/worktrees/fix-errors/docs/assets/`

**Current Structure:**

```text
docs/assets/
├── css/
│   └── style.scss
└── logo.png (7.3MB)
```

**Findings:**

- **No screenshots directory exists yet** - will need to create
- Only logo.png present (PNG format, large file size)
- CSS stored in `assets/css/` subdirectory

### 4.2 Image Format Guidelines

**Based on Jekyll Configuration and Best Practices:**

**Recommended Formats:**

| Format | Use Case | Max Size |
| ------ | -------- | -------- |
| PNG | Screenshots, diagrams with text | 500KB |
| JPG | Photos, complex images | 300KB |
| GIF | Animated demos (use sparingly) | 1MB |
| SVG | Vector diagrams (preferred) | 100KB |

**File Naming Convention:**

```text
# Pattern: feature-component-purpose.ext
tui-progress-indicator.png
tui-color-adaptive-light.png
tui-color-adaptive-dark.png
cost-table-output-example.png
```

### 4.3 Alt Text Requirements

**Accessibility and SEO Best Practices:**

```markdown
![Alt text describing image](../assets/screenshots/tui-progress-indicator.png)
```

**Alt Text Guidelines:**

1. **Describe what the image shows**, not "image of" or "screenshot of"
2. **Be concise** (under 125 characters)
3. **Include context** if relevant to documentation
4. **Use proper capitalization** (sentence case)

**Examples:**

```markdown
# ❌ Poor Alt Text
![screenshot](tui-example.png)
![TUI](../assets/tui.png)

# ✅ Good Alt Text
![Progress indicator showing 75% completion with elapsed time](../assets/screenshots/tui-progress-indicator.png)
![Adaptive color scheme in light terminal with blue accents](../assets/screenshots/tui-color-light.png)
```

### 4.4 Image Placement Patterns

**From Architecture Guide (ASCII diagrams):**

```markdown
### Component Diagram

\```text
┌──────────────────────────────────────────────────────────────┐
│                        FinFocus CLI                         │
│  ┌─────────────────────────────────────────────────────────┐ │
│  │              Command Interface (Cobra)                  │ │
\```
```

**Pattern:**

- **ASCII diagrams** in fenced `text` code blocks (current approach)
- **PNG/SVG images** for complex visual examples
- Place images **immediately after** the introducing paragraph
- Use horizontal rules (`---`) to separate image sections

**Proposed Image Section Pattern:**

```markdown
## Feature Name

Brief introduction to feature.

![Descriptive alt text](../assets/screenshots/feature-example.png)

**Figure 1:** Caption describing what the image demonstrates.

---
```

---

## 5. Configuration Example Patterns

### 5.1 YAML Example Analysis

**From User Guide (lines 297-310):**

```markdown
**Example spec file:**

\```yaml
---
resources:
  aws:ec2/instance:Instance:
    t3.micro:
      monthly: 7.50
      currency: USD
      notes: Linux on-demand
    t3.small:
      monthly: 15.00
      currency: USD
\```
```

**Pattern:**

- Introduce example with bold header
- Use fenced YAML code block with `yaml` language tag
- Include inline comments for explanation (when needed)
- Show complete, runnable examples

### 5.2 Inline Comment Patterns

**Best Practices for Configuration Examples:**

```yaml
# Configuration file for FinFocus
logging:
  # Log level: trace, debug, info, warn, error (default: info)
  level: info

  # Log format: json, text, console (default: console)
  format: json

  # Log to file (optional - defaults to stderr)
  file: /var/log/finfocus/finfocus.log
```

**Pattern:**

- Use `#` comments above configuration keys
- Include **allowed values** in parentheses
- Note **default values** in comments
- Indicate **optional** vs **required** settings

### 5.3 External Documentation Link Patterns

**From User Guide:**

```markdown
### Using Vantage Plugin

Vantage provides unified cost data from multiple cloud providers.

**Setup:**

1. Get Vantage API key from https://vantage.sh
2. Configure plugin (see [Vantage Plugin Setup](../plugins/vantage/setup.md))
3. Run commands with Vantage data
```

**Pattern:**

1. **Numbered setup steps** with clear actions
2. **External links** (https://) for third-party services
3. **Internal links** for detailed documentation
4. **Actionable instructions** ("Get", "Configure", "Run")

### 5.4 Configuration File Templates

**From User Guide (lines 490-508):**

```markdown
### Configuration File

Create or edit `~/.finfocus/config.yaml` to configure logging:

\```yaml
logging:
  # Log level: trace, debug, info, warn, error (default: info)
  level: info

  # Log format: json, text, console (default: console)
  format: json

  # Log to file (optional - defaults to stderr)
  file: /var/log/finfocus/finfocus.log

  # Audit logging for compliance (optional)
  audit:
    enabled: true
    file: /var/log/finfocus/audit.log
\```
```

**Pattern:**

1. **Specify file path** (`~/.finfocus/config.yaml`)
2. **Action verb** ("Create or edit")
3. **Complete, valid YAML** example
4. **Inline comments** explaining each field
5. **Default values** noted in comments

---

## 6. JSON Schema Best Practices

### 6.1 Yaml-Language-Server Integration

**Research Findings:**

The `yaml-language-server` is a Language Server Protocol (LSP) implementation for YAML files that provides:

- **Schema validation** via `# yaml-language-server: $schema=<url>` comments
- **Autocomplete** based on JSON Schema definitions
- **Inline documentation** from schema descriptions

**Standard Pattern:**

```yaml
# yaml-language-server: $schema=https://json.schemastore.org/github-workflow.json

name: CI Pipeline
on:
  push:
    branches: [main]
```

### 6.2 JSON Schema Structure Recommendations

**Based on Public Schema Examples (Kubernetes, GitHub Actions):**

**Basic Schema Structure:**

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "https://example.com/schemas/config.json",
  "title": "FinFocus Configuration",
  "description": "Configuration file for FinFocus CLI tool",
  "type": "object",
  "properties": {
    "logging": {
      "type": "object",
      "description": "Logging configuration",
      "properties": {
        "level": {
          "type": "string",
          "enum": ["trace", "debug", "info", "warn", "error"],
          "default": "info",
          "description": "Log level for console and file output"
        }
      }
    }
  },
  "required": ["logging"]
}
```

**Key Elements:**

| Field | Purpose | Example |
| ----- | ------- | ------- |
| `$schema` | Schema version | `https://json-schema.org/draft/2020-12/schema` |
| `$id` | Schema identifier | `https://finfocus.com/schemas/config.json` |
| `title` | Human-readable name | "FinFocus Configuration" |
| `description` | Schema purpose | "Configuration file for..." |
| `properties` | Field definitions | Object with field schemas |
| `required` | Required fields | Array of field names |
| `enum` | Allowed values | `["info", "warn", "error"]` |
| `default` | Default value | `"info"` |

### 6.3 Documentation Integration Pattern

**YAML File with Schema Comment:**

```yaml
# yaml-language-server: $schema=https://rshade.github.io/finfocus/schemas/config.json

logging:
  level: info    # Autocomplete suggests: trace, debug, info, warn, error
  format: json   # Autocomplete suggests: json, text, console
```

**Schema File Hosting:**

```text
docs/schemas/
├── config.json          # Main configuration schema
├── plugin-manifest.json # Plugin manifest schema
└── pricing-spec.json    # Pricing specification schema
```

**GitHub Pages URL:**

```text
https://rshade.github.io/finfocus/schemas/config.json
```

### 6.4 Schema Validation in Documentation

**Recommended Documentation Pattern:**

```markdown
## Configuration File Schema

The configuration file uses JSON Schema for validation and IDE autocomplete.

**Schema URL:** `https://rshade.github.io/finfocus/schemas/config.json`

**IDE Integration:**

Add this comment to the top of your `~/.finfocus/config.yaml`:

\```yaml
# yaml-language-server: $schema=https://rshade.github.io/finfocus/schemas/config.json
\```

Your editor will then provide:

- ✅ Field autocomplete
- ✅ Inline validation
- ✅ Documentation on hover
- ✅ Default value suggestions
```

---

## 7. Actionable Recommendations for TUI Documentation

### 7.1 Directory Structure Recommendations

**Create New Assets Structure:**

```text
docs/assets/
├── css/
│   └── style.scss
├── logo.png
└── screenshots/        # NEW: Create this directory
    └── tui/            # NEW: TUI-specific screenshots
        ├── progress-indicator.png
        ├── color-adaptive-light.png
        ├── color-adaptive-dark.png
        ├── table-rendering.png
        └── terminal-detection.png
```

### 7.2 TUI Documentation Front Matter Template

```yaml
---
layout: default
title: Terminal UI (TUI) Components
description: Adaptive, accessible terminal UI components for FinFocus CLI
---
```

### 7.3 TUI Code Example Template

**Recommended Pattern:**

```markdown
## Progress Indicator

The TUI package provides a reusable progress indicator with elapsed time tracking.

### Usage Example

\```go
import "github.com/rshade/finfocus/internal/tui"

// Create progress indicator
progress := tui.NewProgressIndicator("Calculating costs", 10)

// Update progress
for i := 0; i < 10; i++ {
    progress.Update(i + 1)
    time.Sleep(100 * time.Millisecond)
}

progress.Complete("Done")
\```

### Visual Example

![Progress indicator showing calculation progress with elapsed time](../assets/screenshots/tui/progress-indicator.png)

**Figure 1:** Progress indicator during cost calculation (75% complete, 1.2s elapsed).

### Configuration

\```yaml
# Optional: Configure TUI behavior in ~/.finfocus/config.yaml
tui:
  # Enable/disable adaptive colors (default: true)
  adaptive_colors: true

  # Force color scheme: auto, light, dark (default: auto)
  color_scheme: auto
\```

See [TUI Configuration Reference](../reference/tui-config.md) for all options.
```

### 7.4 Cross-Linking Strategy for TUI Docs

**Recommended Link Structure:**

```markdown
## Related Documentation

### User Guides
- [User Guide: Output Formats](../guides/user-guide.md#output-formats)
- [CLI Commands: Table Output](../reference/cli-commands.md#table-output)

### Developer Guides
- [Developer Guide: TUI Package](../guides/developer-guide.md#tui-package)
- [Architecture: TUI Design](../architecture/tui-architecture.md)

### Reference Documentation
- [TUI API Reference](../reference/tui-api.md)
- [Color Scheme Configuration](../reference/tui-config.md#color-schemes)
```

### 7.5 Configuration Schema for TUI

**Recommended Schema Location:**

```text
docs/schemas/tui-config.json
```

**Sample Schema:**

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "https://rshade.github.io/finfocus/schemas/tui-config.json",
  "title": "FinFocus TUI Configuration",
  "description": "Terminal UI configuration for FinFocus CLI",
  "type": "object",
  "properties": {
    "tui": {
      "type": "object",
      "description": "Terminal UI settings",
      "properties": {
        "adaptive_colors": {
          "type": "boolean",
          "default": true,
          "description": "Enable adaptive color schemes based on terminal background"
        },
        "color_scheme": {
          "type": "string",
          "enum": ["auto", "light", "dark"],
          "default": "auto",
          "description": "Force specific color scheme: auto (detect), light, or dark"
        }
      }
    }
  }
}
```

### 7.6 Markdownlint Compliance Checklist

**Pre-Publish Checklist:**

- [ ] Front matter includes `layout`, `title`, `description`
- [ ] No H1 headings in content (title in front matter only)
- [ ] All code blocks have language tags
- [ ] Line length under 120 characters (except code/tables)
- [ ] Blank lines before and after headings
- [ ] Unordered lists use `-` (not `*` or `+`)
- [ ] Fenced code blocks (not indented)
- [ ] File ends with newline
- [ ] All links use relative paths with `.md` extension
- [ ] Alt text provided for all images

**Validation Command:**

```bash
make docs-lint
```

---

## 8. Templates for Documentation Authors

### 8.1 Feature Documentation Template

```markdown
---
layout: default
title: [Feature Name]
description: [Brief one-sentence description for SEO]
---

[Brief introduction paragraph explaining what this feature is and why it matters.]

## Table of Contents

1. [Overview](#overview)
2. [Getting Started](#getting-started)
3. [Usage Examples](#usage-examples)
4. [Configuration](#configuration)
5. [API Reference](#api-reference)
6. [Troubleshooting](#troubleshooting)

---

## Overview

[Detailed explanation of the feature, its purpose, and key benefits.]

**Key Features:**

- Feature 1 - Description
- Feature 2 - Description
- Feature 3 - Description

---

## Getting Started

### Prerequisites

- Prerequisite 1
- Prerequisite 2

### Quick Start

\```bash
# Example command
finfocus [command] [options]
\```

**Expected Output:**

\```text
[Example output]
\```

---

## Usage Examples

### Example 1: [Use Case Name]

\```go
// Code example
\```

![Descriptive alt text](../assets/screenshots/feature-example.png)

**Figure 1:** Caption explaining the example.

### Example 2: [Use Case Name]

[Similar structure]

---

## Configuration

\```yaml
# yaml-language-server: $schema=https://rshade.github.io/finfocus/schemas/config.json

feature:
  # Setting description (default: value)
  setting: value
\```

See [Configuration Reference](../reference/config-reference.md#feature) for all options.

---

## API Reference

See [API Reference: Feature](../reference/api-reference.md#feature) for complete API documentation.

---

## Troubleshooting

### Issue: [Problem Description]

**Solution:**

\```bash
# Solution command or fix
\```

---

## Related Documentation

- [User Guide](../guides/user-guide.md)
- [Developer Guide](../guides/developer-guide.md)
- [API Reference](../reference/api-reference.md)

---

**Last Updated:** YYYY-MM-DD
```

### 8.2 Screenshot Documentation Template

```markdown
## Visual Examples

This section provides visual examples of [feature] in different scenarios.

### Light Terminal Theme

![Feature in light terminal with blue accents and high contrast](../assets/screenshots/feature-light.png)

**Figure 1:** [Feature] adapts to light terminal backgrounds with appropriate color schemes.

### Dark Terminal Theme

![Feature in dark terminal with warm accents and reduced brightness](../assets/screenshots/feature-dark.png)

**Figure 2:** [Feature] adapts to dark terminal backgrounds for comfortable viewing.

---
```

### 8.3 Configuration Example Template

```markdown
## Configuration

### File Location

`~/.finfocus/config.yaml`

### Example Configuration

\```yaml
# yaml-language-server: $schema=https://rshade.github.io/finfocus/schemas/config.json

# [Section description]
section:
  # [Setting description] (default: value)
  setting1: value

  # [Setting description] (options: opt1, opt2, opt3)
  setting2: opt1

  # [Optional setting description]
  optional_setting: value
\```

### Configuration Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `setting1` | string | `"value"` | [Description] |
| `setting2` | enum | `"opt1"` | Allowed: `opt1`, `opt2`, `opt3` |
| `optional_setting` | boolean | `false` | [Description] |

### Schema Validation

For IDE autocomplete and validation, add this comment to your config file:

\```yaml
# yaml-language-server: $schema=https://rshade.github.io/finfocus/schemas/config.json
\```

---
```

---

## 9. Key Findings Summary

### 9.1 Critical Success Factors

1. **Front Matter Compliance:** Must use `layout: default`, `title`, and `description`
2. **No H1 in Content:** Title in front matter becomes H1; start content with H2
3. **Language Tags Required:** All code blocks must specify language (MD040)
4. **Relative Links:** Use `.md` extension and relative paths from current document
5. **Image Optimization:** Keep screenshots under 500KB; use descriptive alt text

### 9.2 Quality Gates

**Before Publishing:**

1. Run `make docs-lint` - must pass without errors
2. Verify all links work locally (`make docs-serve`)
3. Check front matter matches template
4. Validate code examples are runnable
5. Ensure images have descriptive alt text
6. Confirm file ends with newline

### 9.3 Common Pitfalls to Avoid

**❌ DON'T:**

- Use H1 (`#`) in document content
- Forget language tags on code blocks
- Use absolute paths for internal links
- Create images over 1MB
- Skip alt text on images
- Use `{% seo %}` tags (plugin not installed)
- Violate 120-character line length in prose
- Use `*` or `+` for unordered lists (must use `-`)

**✅ DO:**

- Start content with H2 (`##`)
- Tag all code blocks with language
- Use relative paths with `.md` extension
- Optimize images to under 500KB
- Write descriptive alt text
- Use manual meta tags if SEO needed
- Break long lines (except code/tables)
- Use `-` for unordered lists

---

## 10. Next Steps for TUI Documentation

### 10.1 Immediate Actions

1. **Create screenshot directory:** `docs/assets/screenshots/tui/`
2. **Generate TUI screenshots:** Light and dark theme examples
3. **Create schema file:** `docs/schemas/tui-config.json`
4. **Draft initial TUI guide:** Using feature documentation template
5. **Validate with linter:** `make docs-lint`

### 10.2 Documentation Files to Create

| File | Purpose | Priority |
| ---- | ------- | -------- |
| `docs/guides/tui-guide.md` | User-facing TUI usage guide | High |
| `docs/reference/tui-api.md` | Developer API reference | High |
| `docs/reference/tui-config.md` | Configuration reference | Medium |
| `docs/architecture/tui-architecture.md` | Design documentation | Low |
| `docs/schemas/tui-config.json` | JSON Schema for validation | Medium |

### 10.3 Integration Points

**Update Existing Documentation:**

| File | Changes Needed |
| ---- | -------------- |
| `docs/guides/user-guide.md` | Add TUI section with examples |
| `docs/guides/developer-guide.md` | Add TUI package documentation |
| `docs/TABLE-OF-CONTENTS.md` | Add TUI documentation links |
| `docs/reference/cli-commands.md` | Note TUI output formatting |

---

## References

### Documentation Files Analyzed

- `/mnt/c/GitHub/go/src/worktrees/fix-errors/docs/_config.yml`
- `/mnt/c/GitHub/go/src/worktrees/fix-errors/docs/guides/user-guide.md`
- `/mnt/c/GitHub/go/src/worktrees/fix-errors/docs/guides/developer-guide.md`
- `/mnt/c/GitHub/go/src/worktrees/fix-errors/docs/guides/architect-guide.md`
- `/mnt/c/GitHub/go/src/worktrees/fix-errors/docs/TABLE-OF-CONTENTS.md`
- `/mnt/c/GitHub/go/src/worktrees/fix-errors/.markdownlint.json`
- `/mnt/c/GitHub/go/src/worktrees/fix-errors/docs/.markdownlint-cli2.jsonc`

### External Resources Consulted

- Jekyll Documentation: <https://jekyllrb.com/>
- Kramdown GFM: <https://kramdown.gettalong.org/>
- Markdownlint Rules: <https://github.com/DavidAnson/markdownlint>
- JSON Schema: <https://json-schema.org/>
- YAML Language Server: <https://github.com/redhat-developer/yaml-language-server>
- GitHub Pages: <https://docs.github.com/en/pages>

---

**Research Completed:** 2026-01-20
**Total Files Analyzed:** 7 documentation files + 2 config files
**Total External Resources:** 6 technical references
