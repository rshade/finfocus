# Documentation Quick Reference

**Purpose**: Fast reference guide for documentation authors working on FinFocus TUI documentation.

**Audience**: Developers and technical writers creating or updating documentation in the `docs/` directory.

---

## Quick Navigation

- [Creating a New Guide](#creating-a-new-guide)
- [Adding CLI Reference Entry](#adding-cli-reference-entry)
- [Creating Configuration Examples](#creating-configuration-examples)
- [Capturing Screenshots](#capturing-screenshots)
- [Cross-Linking Best Practices](#cross-linking-best-practices)
- [Validation and Publishing](#validation-and-publishing)

---

## Creating a New Guide

**When to create a guide**: For major features requiring tutorial-style documentation (budgets, recommendations, accessibility).

### Guide Creation Steps

1. **Copy template**

   ```bash
   cp specs/119-tui-docs-update/contracts/guide-template.md docs/guides/[slug].md
   ```

2. **Fill front matter**

   ```yaml
   ---
   layout: default
   title: [Guide Title]
   description: [Brief one-sentence description for SEO - 50-160 characters]
   ---
   ```

3. **Write content following template sections**

   Required sections:
   - Overview (target audience, prerequisites, learning objectives)
   - Quick Start (copy-paste example in <5 steps)
   - Configuration Reference (table with all options)
   - Examples (3+ practical scenarios)
   - Troubleshooting (common issues and solutions)
   - See Also (cross-links to related docs)

4. **Add cross-links**

   ```markdown
   ## See Also

   - [CLI Reference](../reference/cli-commands.md#command-name)
   - [Configuration Reference](../reference/config-reference.md#section)
   - [Examples](../examples/config-section/)
   ```

5. **Update Table of Contents**

   Edit `docs/TABLE-OF-CONTENTS.md`:

   ```markdown
   ### 📚 Guides

   - [Budget Configuration Guide](guides/budgets.md)
   - [Recommendations Guide](guides/recommendations.md)
   - [Accessibility Guide](guides/accessibility.md)
   ```

6. **Validate**

   ```bash
   make docs-lint
   ```

**Expected outcome**: Complete guide following project markdown conventions, passing linting, with all cross-links working.

---

## Adding CLI Reference Entry

**When to add entry**: When documenting new CLI command or updating existing command flags.

### CLI Documentation Steps

1. **Open reference file**

   ```bash
   vim docs/reference/cli-commands.md
   ```

2. **Copy CLI reference template**

   Use structure from `contracts/cli-reference-entry.md`:

   ```markdown
   ## finfocus [command] [subcommand]

   [One-sentence description]

   ### Synopsis

   \```bash
   finfocus [command] [subcommand] [flags]
   \```

   ### Flags

   | Flag | Type | Default | Required | Description |
   |------|------|---------|----------|-------------|
   | `--flag-name` | string | `"default"` | No | [Description] |
   ```

3. **Document all flags**

   Test against actual CLI to ensure accuracy:

   ```bash
   finfocus [command] --help
   ```

   Include flag table with:
   - Flag name (with `--` prefix)
   - Type (string, boolean, integer, number)
   - Default value (use backticks)
   - Required (Yes/No)
   - Description (include allowed values for enums)

4. **Provide 3-5 examples**

   Cover common scenarios:
   - Basic usage (minimal flags)
   - Filtering or options (key flags)
   - Automation (JSON output, piping)
   - CI/CD integration (exit codes, error handling)
   - Accessibility (plain text, no color)

5. **Add exit codes and environment variables**

   Document non-standard exit codes:

   ```markdown
   ### Exit Codes

   | Code | Meaning |
   |------|---------|
   | `0` | Success |
   | `1` | General error |
   | `2` | Budget threshold exceeded |
   ```

6. **Link to guides**

   ```markdown
   ### See Also

   - [Budget Guide](../guides/budgets.md) - Complete tutorial
   - [Configuration Reference](./config-reference.md#section)
   ```

7. **Validate**

   ```bash
   # Test all commands in examples
   finfocus cost recommendations --pulumi-json plan.json

   # Run linting
   make docs-lint
   ```

**Expected outcome**: Complete CLI reference entry with tested examples, accurate flag documentation, and working cross-links.

---

## Creating Configuration Examples

**When to create example**: To demonstrate specific configuration scenarios (single threshold, multiple alerts, CI/CD integration).

### Configuration Example Steps

1. **Copy template**

   ```bash
   cp specs/119-tui-docs-update/contracts/config-example-template.yaml docs/examples/config-budgets/[scenario-name].yaml
   ```

2. **Fill header section**

   ```yaml
   # [Scenario Name]
   # Use case: [When to use this configuration]
   # Related docs: [Link to guide section]
   # Tested with: FinFocus v[version]

   # yaml-language-server: $schema=https://rshade.github.io/finfocus/schemas/config.json
   ```

3. **Write complete, valid configuration**

   ```yaml
   cost:
     budgets:
       amount: 500.00
       currency: USD
       period: monthly
       alerts:
         - threshold: 80
           type: actual
           # Alert at 80% to catch overruns early
         - threshold: 100
           type: forecasted
           # Alert when forecast exceeds budget
   ```

4. **Add inline comments**

   Explain each field:
   - Why this value is chosen
   - What allowed values are
   - How this relates to use case

5. **Add usage examples section**

   ```yaml
   # ============================================================================
   # USAGE EXAMPLES
   # ============================================================================
   #
   # Basic usage:
   #   finfocus cost projected --pulumi-json plan.json
   #
   # CI/CD integration:
   #   finfocus cost projected --pulumi-json plan.json || exit 1
   ```

6. **Link to guide section**

   ```yaml
   # For more information, see:
   # - Budget Guide: docs/guides/budgets.md#cicd-integration
   # - Configuration Reference: docs/reference/config-reference.md#budgets
   ```

7. **Test configuration**

   ```bash
   # Copy to config location
   cp docs/examples/config-budgets/[scenario-name].yaml ~/.finfocus/config.yaml

   # Test with actual command
   finfocus cost projected --pulumi-json plan.json

   # Verify behavior matches documentation
   ```

8. **Validate against schema** (if schema exists)

   Open in VS Code with yaml-language-server installed to verify autocompletion and validation.

9. **Update examples index**

   Edit `docs/examples/config-budgets/README.md`:

   ```markdown
   ## Budget Configuration Examples

   - [single-threshold.yaml](single-threshold.yaml) - Simple budget limit
   - [multiple-thresholds.yaml](multiple-thresholds.yaml) - Progressive alerts
   - [cicd-integration.yaml](cicd-integration.yaml) - Pipeline cost gates
   ```

**Expected outcome**: Tested, valid configuration example with clear use case, inline comments, and links to guides.

---

## Capturing Screenshots

**When to capture**: For visual proof of features (TUI displays, error messages, loading states).

### Screenshot Capture Steps

1. **Review screenshot checklist**

   See `contracts/screenshot-checklist.md` for complete requirements:
   - Resolution: 1600x900 minimum
   - Format: PNG (static), GIF (animations <5MB)
   - File size: PNG <500KB, GIF <1MB
   - Alt text: <125 characters, descriptive

2. **Prepare terminal**

   ```bash
   # Clean terminal
   clear

   # Set reasonable terminal size (100x30 minimum)
   # Use readable font size (14pt+)

   # Verify no personal info visible (API keys, paths)
   ```

3. **Run command**

   ```bash
   finfocus cost recommendations --pulumi-json plan.json
   ```

   Wait for output to stabilize before capturing.

4. **Capture screenshot**

   **macOS**: `Cmd+Shift+4` (selection) or `Cmd+Shift+4 Space` (window)
   **Linux**: `flameshot gui`
   **Windows**: `Win+Shift+S`

5. **Crop and edit** (if needed)

   - Crop to relevant content
   - Add annotations (red boxes, arrows) if highlighting specific elements
   - Verify text readability

6. **Optimize file size**

   ```bash
   # PNG compression
   pngquant --quality=80-95 --strip input.png -o output.png

   # Or lossless
   optipng -o7 input.png

   # GIF optimization
   gifsicle -O3 --colors 256 --lossy=80 input.gif -o output.gif
   ```

7. **Name file following convention**

   Pattern: `feature-component-purpose.{png,gif}`

   Examples:
   - `budget-tty-mode.png`
   - `recommendations-table.png`
   - `loading-spinner.gif`
   - `error-invalid-json.png`

8. **Move to screenshots directory**

   ```bash
   mv screenshot.png docs/assets/screenshots/budget-tty-mode.png
   ```

9. **Reference in documentation**

   ```markdown
   ![Budget status display with color-coded threshold bars at 75% usage](../assets/screenshots/budget-tty-mode.png)

   **Figure 1**: Budget display in TTY mode with adaptive colors and Unicode box drawing.
   ```

10. **Commit with descriptive message**

    ```bash
    git add docs/assets/screenshots/budget-tty-mode.png
    git commit -m "docs(assets): add budget TTY mode screenshot"
    ```

**Expected outcome**: Optimized screenshot (<500KB), descriptive filename, proper alt text, and figure caption in documentation.

---

## Cross-Linking Best Practices

**Why cross-link**: Improve discoverability, provide context, enable 2-click navigation between related docs.

### Link Types and Patterns

#### 1. Guide to CLI Reference

```markdown
See the [cost recommendations](../reference/cli-commands.md#finfocus-cost-recommendations) command for flag details.
```

#### 2. Guide to Configuration Reference

```markdown
For complete configuration options, see [Budget Configuration](../reference/config-reference.md#budgets).
```

#### 3. Guide to Examples

```markdown
See [budget configuration examples](../examples/config-budgets/) for practical scenarios.
```

#### 4. CLI Reference to Guide

```markdown
### See Also

- [Budget Guide](../guides/budgets.md) - Complete tutorial for budget configuration
```

#### 5. Example to Guide

```yaml
# For more information, see:
# - Budget Guide: docs/guides/budgets.md#cicd-integration
```

### Link Syntax Rules

- **Relative paths**: Use `../` for parent directory, `./` for same directory
- **Include `.md` extension**: Jekyll converts to HTML automatically
- **Anchor links**: Use `#section-name` (lowercase, hyphens for spaces)
- **Descriptive text**: "Budget Guide" not "click here"

### See Also Section Pattern

End guides with cross-links:

```markdown
## See Also

**Related Guides:**
- [Recommendations Guide](./recommendations.md)
- [Accessibility Guide](./accessibility.md)

**CLI Reference:**
- [cost projected](../reference/cli-commands.md#cost-projected)
- [cost recommendations](../reference/cli-commands.md#cost-recommendations)

**Configuration Reference:**
- [Budget Configuration](../reference/config-reference.md#budgets)

**Examples:**
- [Budget Examples](../examples/config-budgets/)
```

### Navigation Testing

Verify all links work:

```bash
# Serve docs locally
make docs-serve

# Open in browser: http://localhost:4000/finfocus/

# Click through all links to verify
```

**Expected outcome**: All internal links work, no 404 errors, 2-click navigation to related content.

---

## Validation and Publishing

**Before committing**: Run validation to catch errors early.

### Pre-Commit Checklist

- [ ] Front matter includes `layout: default`, `title`, `description`
- [ ] No H1 (`#`) in content (title in front matter becomes H1)
- [ ] All code blocks have language tags (bash, yaml, json, text)
- [ ] Line length under 120 characters (except code/tables)
- [ ] Blank lines before and after headings
- [ ] Unordered lists use `-` (not `*` or `+`)
- [ ] Fenced code blocks (not indented)
- [ ] File ends with newline
- [ ] All images have descriptive alt text (<125 characters)
- [ ] All links use relative paths with `.md` extension
- [ ] Examples tested against actual tool
- [ ] Screenshots optimized (PNG <500KB, GIF <1MB)

### Validation Commands

```bash
# Lint all markdown files
make docs-lint

# Expected output: No errors

# Check for broken links (requires markdown-link-check)
npx markdown-link-check docs/guides/budgets.md

# Serve locally for manual testing
make docs-serve

# Open: http://localhost:4000/finfocus/
```

### Common Linting Errors

| Error | Solution |
|-------|----------|
| MD003: Heading style | Use `##` not underline style |
| MD004: List style | Use `-` not `*` or `+` |
| MD013: Line too long | Break line under 120 chars |
| MD022: Blank lines | Add blank line before/after heading |
| MD040: Code language | Add language tag to fenced block |
| MD047: File end | Add newline at end of file |

### Publishing Workflow

1. **Create feature branch**

   ```bash
   git checkout -b docs/tui-documentation
   ```

2. **Make changes** following templates

3. **Validate locally**

   ```bash
   make docs-lint
   make docs-serve  # Manual review
   ```

4. **Commit changes**

   ```bash
   git add docs/guides/budgets.md
   git commit -m "docs(guides): add budget configuration guide"
   ```

5. **Push and create PR**

   ```bash
   git push origin docs/tui-documentation
   gh pr create --title "docs: add TUI documentation" --body "Adds budget, recommendations, and accessibility guides"
   ```

6. **Address review feedback**

7. **Merge to main**

   GitHub Pages automatically deploys from main branch.

8. **Verify deployment**

   ```bash
   # Wait 2-3 minutes for GitHub Pages build
   # Open: https://rshade.github.io/finfocus/guides/budgets/
   ```

**Expected outcome**: Documentation published to GitHub Pages, passing all validation, accessible via project URL.

---

## Common Workflows

### Workflow 1: Document New CLI Command

1. Run command: `finfocus [command] --help`
2. Copy flag table to reference doc
3. Test 3-5 example commands
4. Add to `docs/reference/cli-commands.md`
5. Link from relevant guide
6. Validate: `make docs-lint`

**Time estimate**: 30-45 minutes

---

### Workflow 2: Create Complete Guide

1. Copy `guide-template.md`
2. Fill Overview section (5 minutes)
3. Write Quick Start (10 minutes)
4. Create Configuration Reference table (15 minutes)
5. Write 3-5 examples (30 minutes)
6. Add Troubleshooting (15 minutes)
7. Add See Also cross-links (10 minutes)
8. Capture screenshots (20 minutes)
9. Validate and publish (10 minutes)

**Time estimate**: 2-3 hours

---

### Workflow 3: Add Configuration Example

1. Copy `config-example-template.yaml`
2. Write valid configuration (10 minutes)
3. Add inline comments (10 minutes)
4. Test configuration (10 minutes)
5. Add usage examples (5 minutes)
6. Update examples index (5 minutes)

**Time estimate**: 40 minutes

---

### Workflow 4: Capture and Add Screenshot

1. Prepare terminal (clean, readable font)
2. Run command
3. Capture screenshot
4. Optimize file size
5. Add to `docs/assets/screenshots/`
6. Reference in markdown with alt text
7. Commit

**Time estimate**: 10-15 minutes per screenshot

---

## Troubleshooting

### Issue: Linting fails with MD013 (line too long)

**Solution**: Break long lines. Exceptions allowed for code blocks and tables.

```markdown
# Too long (>120 chars)
This is a very long sentence that exceeds the line length limit and should be broken into multiple lines for better readability.

# Fixed
This is a very long sentence that exceeds the line length limit and should be
broken into multiple lines for better readability.
```

---

### Issue: Links broken after committing

**Solution**: Use relative paths, include `.md` extension.

```markdown
# Wrong (absolute path)
[Guide](/docs/guides/budgets.md)

# Wrong (missing extension)
[Guide](../guides/budgets)

# Correct
[Guide](../guides/budgets.md)
```

---

### Issue: Screenshots too large

**Solution**: Compress with pngquant.

```bash
pngquant --quality=80-95 --strip input.png -o output.png
```

---

### Issue: Code blocks showing syntax errors

**Solution**: Add language tag.

```markdown
# Wrong (no language tag)
\```
finfocus cost projected
\```

# Correct
\```bash
finfocus cost projected
\```
```

---

## Resources

### Templates

- [Guide Template](contracts/guide-template.md)
- [CLI Reference Entry](contracts/cli-reference-entry.md)
- [Configuration Example](contracts/config-example-template.yaml)
- [Screenshot Checklist](contracts/screenshot-checklist.md)

### Tools

- **Linting**: `make docs-lint` (markdownlint)
- **Local preview**: `make docs-serve` (Jekyll)
- **Link checking**: `npx markdown-link-check [file]`
- **Screenshot optimization**: `pngquant`, `optipng`, `gifsicle`

### Documentation

- [Jekyll Documentation](https://jekyllrb.com/)
- [Markdown Guide](https://www.markdownguide.org/)
- [GitHub Pages](https://docs.github.com/en/pages)
- [Markdownlint Rules](https://github.com/DavidAnson/markdownlint/blob/main/doc/Rules.md)

---

## Getting Help

- **Slack**: #finfocus-docs channel
- **GitHub Issues**: [Documentation label](https://github.com/rshade/finfocus/labels/documentation)
- **Style Questions**: Refer to `.markdownlint-cli2.jsonc` and existing guides

---

**Quick Reference Version**: 1.0.0
**Last Updated**: 2026-01-20
**Maintainer**: Documentation Team
