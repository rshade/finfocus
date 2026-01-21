# Documentation Content Data Model

**Feature**: TUI Documentation Updates (specs/119-tui-docs-update)
**Date**: 2026-01-20
**Phase**: Design Phase 1

## Overview

This document defines the content taxonomy and entity relationships for the TUI documentation feature. It establishes a structured model for organizing guides, reference documentation, examples, visual assets, and machine-readable schemas.

---

## Content Entities

### 1. Guide Entity

**Purpose**: Long-form tutorial or explanatory document designed for learning and discovery.

**Attributes**:

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `title` | string | Yes | Document title (becomes H1 via Jekyll front matter) |
| `slug` | string | Yes | URL-friendly identifier (e.g., "budgets", "recommendations") |
| `target_audience` | enum | Yes | Primary audience (user, developer, architect, business) |
| `prerequisites` | list[string] | No | Required knowledge or completed setup (e.g., "FinFocus installed", "API key configured") |
| `learning_objectives` | list[string] | Yes | What readers will learn (3-5 bullet points) |
| `sections` | list[Section] | Yes | Structured content sections (see Section structure below) |
| `estimated_time` | integer | No | Minutes to complete guide (for quick start sections) |

**Section Structure**:

```yaml
sections:
  - name: "Overview"
    heading_level: 2
    content_type: "prose"
    required: true
  - name: "Quick Start"
    heading_level: 2
    content_type: "tutorial"
    required: true
  - name: "Configuration Reference"
    heading_level: 2
    content_type: "reference_table"
    required: true
  - name: "Examples"
    heading_level: 2
    content_type: "scenarios"
    required: true
  - name: "Troubleshooting"
    heading_level: 2
    content_type: "faq"
    required: false
  - name: "See Also"
    heading_level: 2
    content_type: "cross_links"
    required: true
```

**Relationships**:

- **References** → CLI Reference Entries (for command syntax)
- **Links to** → Configuration Examples (for practical scenarios)
- **Points to** → Configuration Reference (for complete option details)
- **Embeds** → Visual Examples (screenshots, GIFs)
- **Validates against** → JSON Schema (for config snippets)

**Example**:

```yaml
guide:
  title: "Budget Configuration Guide"
  slug: "budgets"
  target_audience: "user"
  prerequisites:
    - "FinFocus CLI installed"
    - "Pulumi project with cost data"
  learning_objectives:
    - "Configure budget thresholds and alerts"
    - "Integrate budget checks into CI/CD pipelines"
    - "Set up webhook notifications for budget overruns"
  sections:
    - Overview
    - Quick Start
    - Configuration Reference
    - Examples
    - CI/CD Integration
    - Troubleshooting
    - See Also
```

---

### 2. Reference Entry Entity

**Purpose**: Factual reference documentation for CLI commands, configuration options, or API endpoints.

**Attributes**:

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `command_name` | string | Yes | Full command path (e.g., "finfocus cost recommendations") |
| `synopsis` | string | Yes | One-line description of command purpose |
| `usage_pattern` | string | Yes | Command syntax (e.g., "finfocus cost recommendations [flags]") |
| `flags` | list[Flag] | Yes | Command-line flags (see Flag structure below) |
| `examples` | list[Example] | Yes | Practical usage examples (3-5 minimum) |
| `exit_codes` | list[ExitCode] | No | Command exit codes and meanings |
| `related_commands` | list[string] | No | Related CLI commands |

**Flag Structure**:

```yaml
flags:
  - name: "--pulumi-json"
    type: "string"
    default: null
    required: true
    description: "Path to Pulumi plan JSON file"
    example: "plan.json"
  - name: "--filter"
    type: "string"
    default: ""
    required: false
    description: "Filter expression for recommendations (e.g., 'type=rightsize')"
    example: "priority=high"
```

**Example Structure**:

```yaml
examples:
  - description: "Basic usage with Pulumi plan"
    command: "finfocus cost recommendations --pulumi-json plan.json"
    output_snippet: "│ RESOURCE │ RECOMMENDATION │ SAVINGS │"
  - description: "Filter high-priority recommendations"
    command: "finfocus cost recommendations --pulumi-json plan.json --filter priority=high"
    output_snippet: null
```

**Relationships**:

- **Referenced by** → Guides (for command documentation)
- **Links to** → Configuration Reference (for config-based options)
- **Includes** → Visual Examples (for command output)

**Example**:

```yaml
reference_entry:
  command_name: "finfocus cost recommendations"
  synopsis: "Display cost optimization recommendations from cloud providers"
  usage_pattern: "finfocus cost recommendations [flags]"
  flags:
    - name: "--pulumi-json"
      type: "string"
      required: true
      description: "Path to Pulumi plan JSON"
    - name: "--filter"
      type: "string"
      default: ""
      description: "Filter recommendations by type or priority"
    - name: "--output"
      type: "string"
      default: "table"
      description: "Output format (table, json, ndjson)"
  examples:
    - "Basic usage"
    - "With filters"
    - "JSON output for automation"
```

---

### 3. Configuration Example Entity

**Purpose**: Complete, runnable YAML configuration demonstrating a specific scenario.

**Attributes**:

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `scenario_name` | string | Yes | Descriptive name (e.g., "Single Budget Threshold") |
| `filename` | string | Yes | File name in docs/examples/ (e.g., "single-threshold.yaml") |
| `yaml_content` | string | Yes | Complete YAML configuration |
| `use_case_description` | string | Yes | When to use this configuration (1-2 sentences) |
| `inline_comments` | boolean | Yes | Whether YAML includes inline # comments |
| `validation_status` | enum | Yes | Schema validation status (valid, invalid, not_tested) |
| `related_guide_section` | string | Yes | Link to guide section explaining this scenario |

**Relationships**:

- **Referenced by** → Guides (in Examples sections)
- **Validates against** → JSON Schema (for correctness)
- **Demonstrates** → Configuration Reference options (practical usage)

**Example**:

```yaml
config_example:
  scenario_name: "Single Budget Threshold with Exit Code"
  filename: "single-threshold.yaml"
  use_case_description: "Fail CI/CD pipeline when total cost exceeds monthly budget"
  inline_comments: true
  validation_status: "valid"
  related_guide_section: "budgets.md#cicd-integration"
  yaml_content: |
    # Single Budget Threshold Configuration
    # Use case: Block deployments exceeding $500/month
    # Exit code: 1 when budget exceeded

    cost:
      budgets:
        amount: 500.00
        currency: USD
        period: monthly
        alerts:
          - threshold: 100  # Alert at 100% of budget
            type: actual
```

---

### 4. Visual Example Entity

**Purpose**: Screenshot or animated GIF demonstrating tool output or UI behavior.

**Attributes**:

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `filename` | string | Yes | File name in docs/assets/screenshots/ (e.g., "budget-tty-mode.png") |
| `alt_text` | string | Yes | Descriptive alt text for accessibility (under 125 chars) |
| `caption` | string | Yes | Figure caption explaining what image demonstrates |
| `context` | string | Yes | Where this visual is used (guide section or README) |
| `display_mode` | enum | No | Terminal mode shown (tty, plain, high_contrast) |
| `format` | enum | Yes | Image format (png, gif, svg) |
| `dimensions` | string | No | Image dimensions (e.g., "1600x900") |
| `file_size_kb` | integer | No | File size in kilobytes (for optimization tracking) |

**Relationships**:

- **Embedded in** → Guides (for visual explanations)
- **Referenced in** → README (for feature showcase)
- **Demonstrates** → CLI Commands (showing actual output)

**Example**:

```yaml
visual_example:
  filename: "budget-tty-mode.png"
  alt_text: "Budget status display with color-coded threshold bars and emoji indicators"
  caption: "Figure 1: Budget display in TTY mode with adaptive colors (75% of budget used)"
  context: "budgets.md#quick-start"
  display_mode: "tty"
  format: "png"
  dimensions: "1600x900"
  file_size_kb: 87
```

---

### 5. JSON Schema Entity

**Purpose**: Machine-readable configuration validation and IDE autocompletion support.

**Attributes**:

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `schema_version` | string | Yes | JSON Schema version (e.g., "draft-2020-12") |
| `schema_id` | string | Yes | Schema URL (e.g., `https://rshade.github.io/finfocus/schemas/config.json`) |
| `definitions` | list[Definition] | Yes | Reusable schema definitions (see Definition structure) |
| `required_fields` | list[string] | Yes | Required top-level fields |
| `validation_rules` | list[ValidationRule] | Yes | Custom validation logic (patterns, ranges, enums) |

**Definition Structure**:

```json
{
  "definitions": {
    "BudgetAlert": {
      "type": "object",
      "properties": {
        "threshold": {
          "type": "number",
          "minimum": 1,
          "maximum": 100,
          "description": "Percentage of budget at which to trigger alert (1-100)"
        },
        "type": {
          "type": "string",
          "enum": ["actual", "forecasted"],
          "description": "Alert trigger type: actual (current costs) or forecasted (projected)"
        }
      },
      "required": ["threshold", "type"]
    }
  }
}
```

**Relationships**:

- **Validates** → Configuration Examples (ensures correctness)
- **Enables** → IDE Autocompletion (via yaml-language-server)
- **Documents** → Configuration Reference (provides field descriptions)

**Example**:

```yaml
json_schema:
  schema_version: "draft-2020-12"
  schema_id: "https://rshade.github.io/finfocus/schemas/config.json"
  definitions:
    - BudgetAlert
    - BudgetConfig
    - AccessibilityOptions
  required_fields:
    - "cost"
  validation_rules:
    - "threshold must be 1-100"
    - "currency must be ISO 4217 code"
    - "period must be daily, weekly, monthly, or yearly"
```

---

## Content Relationships Diagram

```text
┌─────────────────────────────────────────────────────────────────┐
│                         Content Flow                            │
└─────────────────────────────────────────────────────────────────┘

README.md (Feature Overview)
   │
   ├─→ Guide (budgets.md)
   │     │
   │     ├─→ CLI Reference Entry (cost recommendations)
   │     │     └─→ Visual Example (command output)
   │     │
   │     ├─→ Configuration Example (single-threshold.yaml)
   │     │     └─→ JSON Schema (validates YAML)
   │     │
   │     ├─→ Configuration Reference (budgets section)
   │     │
   │     └─→ Visual Example (budget-tty-mode.png)
   │
   ├─→ Guide (recommendations.md)
   │     ├─→ CLI Reference Entry (cost recommendations)
   │     ├─→ Visual Example (recommendations-table.png)
   │     └─→ Configuration Example (webhook-notifications.yaml)
   │
   └─→ Guide (accessibility.md)
         ├─→ Configuration Reference (accessibility section)
         ├─→ Visual Example (high-contrast-mode.png)
         └─→ Configuration Example (cicd-integration.yaml)

Schema validates → Configuration Examples
Examples demonstrate → Configuration Reference
Reference documents → CLI Commands
Guides integrate → All entity types
```

---

## Content Taxonomy Rules

### 1. Guide Writing Rules

- **Start with context**: Explain what problem this solves and who it's for
- **Provide quick start**: Copy-paste example within first 3 paragraphs
- **Use progressive disclosure**: Basic → Intermediate → Advanced sections
- **Include visual proof**: At least 1 screenshot per major feature
- **End with cross-links**: Point to related guides, reference docs, examples

### 2. Reference Entry Rules

- **Be complete**: Document every flag, option, and behavior
- **Use tables**: Flag tables must include name, type, default, description
- **Provide examples**: Minimum 3 examples covering basic, filtered, and advanced usage
- **Note exit codes**: Document success (0), error (1), and special exit codes
- **Link to guides**: Reference guides for conceptual explanations

### 3. Configuration Example Rules

- **Be runnable**: Every example must be valid, complete YAML
- **Add context**: Inline comments explain why settings are chosen
- **State use case**: Header comment describes when to use this config
- **Link to docs**: Footer comment points to guide section for explanation
- **Validate**: Test against JSON schema before publishing

### 4. Visual Example Rules

- **Show real output**: Capture from actual tool execution, not mockups
- **Provide context**: Include command prompt showing command run
- **Optimize size**: PNG under 500KB, GIF under 1MB
- **Write alt text**: Describe what image shows (not "screenshot of")
- **Add captions**: Figure number + explanation of what's demonstrated

### 5. JSON Schema Rules

- **Use descriptions**: Every field has user-facing description
- **Provide defaults**: Specify default values in schema
- **Define enums**: List allowed values for type-safety
- **Set constraints**: Minimum/maximum for numbers, patterns for strings
- **Version schemas**: Update schema_id when structure changes

---

## Content Organization Strategy

### Directory Structure

```text
docs/
├── guides/                     # Long-form tutorials
│   ├── budgets.md
│   ├── recommendations.md
│   └── accessibility.md
│
├── reference/                  # Factual reference docs
│   ├── cli-commands.md         # CLI Reference Entries
│   └── config-reference.md     # Configuration Reference
│
├── examples/                   # Runnable configurations
│   ├── config-budgets/
│   │   ├── single-threshold.yaml
│   │   ├── multiple-thresholds.yaml
│   │   └── README.md           # Index of examples
│   └── README.md
│
├── assets/
│   └── screenshots/            # Visual examples
│       ├── budget-tty-mode.png
│       └── recommendations-table.png
│
└── schemas/                    # JSON schemas
    └── config-schema.json
```

### Cross-Linking Strategy

**Guides should link to**:

- CLI Reference (for command syntax)
- Configuration Reference (for option details)
- Examples (for practical scenarios)
- Other Guides (for related concepts)

**Reference docs should link to**:

- Guides (for tutorials and explanations)
- Examples (for practical usage)
- Schemas (for validation)

**Examples should link to**:

- Guides (for context and explanation)
- Reference (for option documentation)

**Navigation hierarchy**:

1. README → Guides (primary entry point)
2. Guides → Reference + Examples (deep dive)
3. Reference → Guides (for learning)
4. Examples → Guides (for explanation)

---

## Validation & Quality Criteria

### Guide Quality Checklist

- [ ] Target audience clearly stated in Overview
- [ ] Prerequisites listed (if any)
- [ ] Quick Start section with copy-paste example
- [ ] At least 1 visual example per major feature
- [ ] Configuration Reference table with all options
- [ ] 3+ practical examples covering common scenarios
- [ ] Troubleshooting section with solutions
- [ ] See Also section with cross-links
- [ ] Passes markdownlint validation
- [ ] All links verified with markdown-link-check

### Reference Entry Quality Checklist

- [ ] Synopsis clearly describes command purpose
- [ ] Usage pattern shows correct syntax
- [ ] Flag table includes: name, type, default, description
- [ ] Minimum 3 examples provided
- [ ] Exit codes documented (if non-standard)
- [ ] Related commands listed
- [ ] Examples tested against actual CLI

### Configuration Example Quality Checklist

- [ ] Complete, valid YAML (no placeholders)
- [ ] Header comment states use case
- [ ] Inline comments explain each field
- [ ] Footer comment links to guide section
- [ ] Validates against JSON schema
- [ ] Tested in actual FinFocus environment

### Visual Example Quality Checklist

- [ ] Captured from real tool output
- [ ] Resolution: 1600x900 minimum
- [ ] Format: PNG (static) or GIF (animated, <5MB)
- [ ] Alt text under 125 characters
- [ ] Caption explains what's demonstrated
- [ ] Optimized file size (PNG <500KB)
- [ ] Context shows command prompt + output

### JSON Schema Quality Checklist

- [ ] Uses JSON Schema draft-2020-12
- [ ] Every field has description
- [ ] Default values specified
- [ ] Enums defined for allowed values
- [ ] Constraints set (min/max, patterns)
- [ ] Validates all configuration examples
- [ ] Hosted at correct GitHub Pages URL

---

## Content Lifecycle

```text
1. Draft → 2. Review → 3. Publish → 4. Maintain

Draft:
- Author creates content from template
- Validates against style guide
- Tests examples against actual tool

Review:
- Technical review (accuracy)
- Editorial review (clarity, grammar)
- Accessibility review (alt text, structure)
- Link validation (markdown-link-check)

Publish:
- Merge to main branch
- GitHub Pages deploys automatically
- Update TABLE-OF-CONTENTS.md
- Announce in release notes

Maintain:
- Update when features change
- Refresh screenshots (quarterly)
- Validate links (monthly)
- Review feedback and improve
```

---

## Appendix: Entity Templates

### Guide Template (Markdown)

```markdown
---
layout: default
title: [Guide Title]
description: [Brief one-sentence description for SEO]
---

## Overview

[What this guide covers, who it's for, what you'll learn]

**Target Audience**: [User/Developer/Architect]

**Prerequisites**:
- [Prerequisite 1]
- [Prerequisite 2]

**Learning Objectives**:
- [Objective 1]
- [Objective 2]
- [Objective 3]

---

## Quick Start

[Copy-paste example with expected output]

---

## Configuration Reference

[Table with all options: name, type, default, description]

---

## Examples

### Example 1: [Scenario Name]

[Description, code, explanation]

### Example 2: [Scenario Name]

[Description, code, explanation]

---

## Troubleshooting

### Issue: [Problem]

**Solution**: [Fix]

---

## See Also

- [Related Guide 1](../guides/guide1.md)
- [CLI Reference](../reference/cli-commands.md#command)
- [Configuration Reference](../reference/config-reference.md#section)
- [Examples](../examples/config-section/)
```

### Configuration Example Template (YAML)

```yaml
# [Scenario Name]
# Use case: [When to use this configuration]
# Related docs: [Link to guide section]
# Tested with: FinFocus v[version]

cost:
  budgets:
    amount: [value]
    currency: USD
    period: monthly
    alerts:
      - threshold: [value]
        type: [actual|forecasted]
        # [Explanation of this alert]

# For more information, see:
# [Link to guide section]
```

### JSON Schema Template

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "https://rshade.github.io/finfocus/schemas/config.json",
  "title": "FinFocus Configuration",
  "description": "Configuration file for FinFocus CLI tool",
  "type": "object",
  "properties": {
    "cost": {
      "type": "object",
      "description": "Cost management settings",
      "properties": {
        "budgets": {
          "type": "object",
          "description": "Budget thresholds and alerts",
          "properties": {
            "amount": {
              "type": "number",
              "minimum": 0,
              "description": "Budget amount in specified currency"
            },
            "currency": {
              "type": "string",
              "enum": ["USD", "EUR", "GBP"],
              "default": "USD",
              "description": "ISO 4217 currency code"
            }
          },
          "required": ["amount", "currency"]
        }
      }
    }
  }
}
```

---

**Data Model Version**: 1.0.0
**Last Updated**: 2026-01-20
**Status**: Complete
