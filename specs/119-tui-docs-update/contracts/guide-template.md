# Guide Template

**Purpose**: Template for creating long-form tutorial and explanation documents (budgets.md, recommendations.md, accessibility.md).

**Usage**: Copy this template to `docs/guides/[slug].md` and fill in sections following the structure below.

---

## Front Matter

```yaml
---
layout: default
title: [Guide Title]
description: [Brief one-sentence description for SEO - 50-160 characters]
---
```

**Front Matter Rules**:

- `layout: default` (required - uses Jekyll default layout)
- `title` becomes H1 heading (do NOT use `#` in content)
- `description` used for SEO and social previews
- No additional fields needed (category inherited from directory)

---

## Content Structure

### Section 1: Overview

**Purpose**: Introduce the feature, explain value proposition, and set expectations.

```markdown
## Overview

[2-3 paragraph introduction explaining:]
- What this feature is
- Why it's useful (problem it solves)
- Who should use it (target audience)

**Target Audience**: [User/Developer/Architect/Business]

**Prerequisites**:
- [Prerequisite 1 - e.g., "FinFocus CLI installed"]
- [Prerequisite 2 - e.g., "Pulumi project configured"]

**Learning Objectives**:
- [Objective 1 - What readers will learn]
- [Objective 2]
- [Objective 3]

**Estimated Time**: [X minutes for quick start, Y minutes for full guide]

---
```

**Writing Guidelines**:

- Start with concrete benefit ("Configure budget alerts to prevent cost overruns")
- Use active voice and present tense
- Keep prerequisites minimal (link to setup guides)
- Learning objectives should be specific and measurable

---

### Section 2: Quick Start

**Purpose**: Provide immediate, copy-paste example to demonstrate core functionality.

```markdown
## Quick Start

Get started with [feature] in under 5 minutes.

### Step 1: [Action Verb - e.g., "Configure Budget"]

[Brief explanation of what user is doing]

```yaml
# Paste this into ~/.finfocus/config.yaml
cost:
  budgets:
    amount: 500.00
    currency: USD
    period: monthly
```

### Step 2: [Action Verb - e.g., "Run Command"]

```bash
finfocus cost projected --pulumi-json plan.json
```

### Step 3: [Action Verb - e.g., "Review Output"]

**Expected Output:**

```text
[Show actual output from tool]
```

![Descriptive alt text](../assets/screenshots/feature-quick-start.png)

**Figure 1**: [Caption explaining what visual demonstrates]

**What's Next?**

- [Link to next logical step - e.g., "Configure alerts"]
- [Link to examples - e.g., "See common scenarios"]
- [Link to reference - e.g., "Learn about all options"]

---

```text
(end of template section)
```

**Writing Guidelines**:

- Maximum 3-5 steps for quick start
- Each step has clear action verb (Configure, Run, Review, Verify)
- Show actual, tested commands and output
- Include 1 visual example for proof of concept
- End with clear next steps

---

### Section 3: Configuration Reference

**Purpose**: Document all configuration options in structured, scannable format.

```markdown
## Configuration Reference

Complete reference for all [feature] configuration options.

### File Location

Configuration is stored in `~/.finfocus/config.yaml`.

### Schema Reference

For IDE autocomplete and validation, add this comment to your config file:

```yaml
# yaml-language-server: $schema=https://rshade.github.io/finfocus/schemas/config.json
```

Your editor will provide autocompletion and validation with the yaml-language-server installed.

### Configuration Options

| Option | Type | Default | Required | Description |
|--------|------|---------|----------|-------------|
| `amount` | number | - | Yes | Budget amount in specified currency |
| `currency` | string | `"USD"` | No | ISO 4217 currency code (USD, EUR, GBP) |
| `period` | string | `"monthly"` | No | Budget period (daily, weekly, monthly, yearly) |
| `alerts` | list | `[]` | No | Alert thresholds (see Alerts Options below) |

### Alerts Options

| Option | Type | Default | Required | Description |
|--------|------|---------|----------|-------------|
| `threshold` | number | - | Yes | Percentage of budget (1-100) |
| `type` | string | `"actual"` | No | Alert type (actual, forecasted) |

### Environment Variables

Override configuration with environment variables:

| Variable | Description | Example |
|----------|-------------|---------|
| `FINFOCUS_BUDGET_AMOUNT` | Override budget amount | `500.00` |
| `FINFOCUS_BUDGET_CURRENCY` | Override currency | `EUR` |

See [Configuration Reference](../reference/config-reference.md#budgets) for complete details.

---

```text
(end of template section)
```

**Writing Guidelines**:

- Use tables for option documentation (easier to scan)
- Include: option name, type, default, required, description
- Group related options (e.g., main config vs. alert config)
- Document environment variable overrides
- Link to full configuration reference for details

---

### Section 4: Examples

**Purpose**: Demonstrate common scenarios with complete, runnable configurations.

```markdown
## Examples

Practical examples for common [feature] scenarios.

### Example 1: [Scenario Name - e.g., "Single Budget Threshold"]

**Use Case**: [When to use this configuration]

**Configuration:**

```yaml
# yaml-language-server: $schema=https://rshade.github.io/finfocus/schemas/config.json

cost:
  budgets:
    amount: 500.00
    currency: USD
    period: monthly
    alerts:
      - threshold: 100
        type: actual
```

**Usage:**

```bash
finfocus cost projected --pulumi-json plan.json
```

**Output:**

![Descriptive alt text](../assets/screenshots/example1-output.png)

**Figure 2**: [Caption explaining output]

**Explanation:**

[2-3 sentences explaining why this works and when to use it]

See [complete example](../examples/config-budgets/single-threshold.yaml) for full configuration.

---

### Example 2: [Scenario Name - e.g., "Multiple Alert Thresholds"]

[Same structure as Example 1]

---

### Example 3: [Scenario Name - e.g., "CI/CD Integration"]

[Same structure as Example 1]

---

```text
(end of template section)
```

**Writing Guidelines**:

- Minimum 3 examples covering basic, intermediate, advanced
- Each example has: use case, configuration, usage, output, explanation
- Use actual, tested configurations (no placeholders)
- Include visual proof of output
- Link to complete example files in docs/examples/

---

### Section 5: Advanced Usage (Optional)

**Purpose**: Cover complex scenarios, edge cases, and power-user features.

```markdown
## Advanced Usage

Advanced [feature] patterns for specific scenarios.

### Pattern 1: [Advanced Pattern Name]

[Description, configuration, explanation]

### Pattern 2: [Advanced Pattern Name]

[Description, configuration, explanation]

### Integration with [Other Feature]

[How this feature works with other FinFocus features]

---
```

**Writing Guidelines**:

- Only include if there are genuinely advanced patterns
- Explain why someone would use advanced approach
- Document edge cases and limitations
- Link to architecture docs for deep technical details

---

### Section 6: Troubleshooting

**Purpose**: Solve common problems users encounter.

```markdown
## Troubleshooting

Common issues and solutions for [feature].

### Issue: [Problem Description]

**Symptoms:**

- [Symptom 1]
- [Symptom 2]

**Cause:**

[Explanation of why this happens]

**Solution:**

```bash
# Command or configuration fix
```

**Verification:**

[How to confirm the fix worked]

---

### Issue: [Another Problem]

[Same structure]

---

### Getting Help

If you encounter issues not covered here:

1. Check [FAQ](../support/faq.md) for general questions
2. Search [GitHub Issues](https://github.com/rshade/finfocus/issues)
3. Review [Troubleshooting Guide](../support/troubleshooting.md)
4. Open a [new issue](https://github.com/rshade/finfocus/issues/new) with details

---

```text
(end of template section)
```

**Writing Guidelines**:

- Focus on common problems (not every edge case)
- Use "Issue/Symptoms/Cause/Solution" structure
- Provide copy-paste fixes when possible
- Link to broader troubleshooting resources
- Include verification steps

---

### Section 7: See Also

**Purpose**: Provide cross-links to related documentation for discovery.

```markdown
## See Also

**Related Guides:**

- [Recommendations Guide](./recommendations.md) - Cost optimization suggestions
- [Accessibility Guide](./accessibility.md) - Terminal display options

**CLI Reference:**

- [cost projected](../reference/cli-commands.md#cost-projected) - Estimate projected costs
- [cost recommendations](../reference/cli-commands.md#cost-recommendations) - Display recommendations

**Configuration Reference:**

- [Budget Configuration](../reference/config-reference.md#budgets) - Complete option reference
- [Alert Configuration](../reference/config-reference.md#alerts) - Alert threshold settings

**Examples:**

- [Budget Examples](../examples/config-budgets/) - Runnable configuration files
- [CI/CD Integration Examples](../examples/cicd/) - Pipeline configurations

**Architecture Documentation:**

- [TUI Architecture](../architecture/tui-architecture.md) - Terminal UI design
- [Budget System Design](../architecture/budget-system.md) - Budget implementation

---
```

**Writing Guidelines**:

- Group links by category (Guides, Reference, Examples, Architecture)
- Use descriptive link text (not "click here")
- Prioritize most relevant links (3-5 per category)
- Link to specific sections with anchors when appropriate

---

### Section 8: Footer

```markdown
---

**Last Updated:** [YYYY-MM-DD]
**FinFocus Version:** [vX.Y.Z when guide written]
**Feedback:** [Open an issue](https://github.com/rshade/finfocus/issues/new) to improve this guide
```

**Footer Guidelines**:

- Include last updated date for freshness indicator
- Note FinFocus version to track feature availability
- Provide feedback mechanism

---

## Complete Example

See [budgets.md example](../examples/guide-budgets-example.md) for complete guide following this template.

---

## Validation Checklist

Before publishing, verify:

- [ ] Front matter includes layout, title, description
- [ ] No H1 (`#`) in content (title in front matter becomes H1)
- [ ] Overview includes target audience, prerequisites, learning objectives
- [ ] Quick Start has 3-5 steps with copy-paste examples
- [ ] Configuration Reference has table with all options
- [ ] Minimum 3 examples with use cases, configs, outputs
- [ ] Troubleshooting covers 3-5 common issues
- [ ] See Also section links to 10-15 related docs
- [ ] All code blocks have language tags (bash, yaml, text)
- [ ] All images have descriptive alt text
- [ ] All links verified with markdown-link-check
- [ ] File ends with newline
- [ ] Passes `make docs-lint` validation

---

## Style Guidelines

**Tone**: Professional, helpful, conversational (not formal or academic)

**Voice**: Active voice, present tense ("Configure the budget" not "The budget can be configured")

**Perspective**: Second person ("You can configure" not "Users can configure")

**Formatting**:

- Use `-` for unordered lists (not `*` or `+`)
- Use fenced code blocks with language tags
- Keep line length under 120 characters (except code/tables)
- Add blank lines before and after headings
- End file with newline

**Terminology**:

- "CLI" (not "command line" or "terminal")
- "configuration" (not "config" in prose, "config" in code)
- "FinFocus" (capital F)
- "Pulumi" (capital P)

**Cross-Linking**:

- Use relative links: `[text](../guides/guide.md)`
- Include `.md` extension (Jekyll converts to HTML)
- Link to specific sections: `[text](file.md#section-name)`
- Use descriptive link text (not "here" or "this link")

---

**Template Version**: 1.0.0
**Last Updated**: 2026-01-20
**Usage**: Copy to `docs/guides/[slug].md` and customize
