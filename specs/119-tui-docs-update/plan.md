# Implementation Plan: TUI Documentation Updates

**Branch**: `119-tui-docs-update` | **Date**: 2026-01-20 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/119-tui-docs-update/spec.md`

## Summary

This feature delivers comprehensive documentation updates for newly implemented TUI features in FinFocus, including budget configuration, cost recommendations, and accessibility options. The documentation will enable users to discover, configure, and leverage these features effectively through dedicated guides, CLI reference updates, configuration examples, and visual demonstrations.

**Primary Deliverables**:
- Budget Configuration Guide (budgets.md) with quick start, reference tables, and CI/CD examples
- Recommendations Guide (recommendations.md) with command usage and interactive keyboard shortcuts
- Accessibility Features Guide (accessibility.md) with color/contrast options and environment variables
- Updated CLI Reference (cli-commands.md) with complete flag documentation for new commands
- Updated Configuration Reference (config-reference.md) with budget/alert YAML structures
- Updated README.md with features overview and quick start guidance
- Example configurations (docs/examples/) demonstrating common scenarios
- JSON Schema (config-schema.json) for IDE autocompletion support
- Visual examples (screenshots/GIFs) for budget displays, recommendations interface, loading states

**Technical Approach**: Documentation-first implementation using existing Jekyll/GitHub Pages infrastructure. Content will be authored in Markdown, validated with markdownlint, and cross-linked for discoverability. Visual examples will be captured from actual tool output to ensure accuracy.

## Technical Context

**Language/Version**: Markdown (CommonMark spec), YAML for configuration examples, JSON for schema
**Primary Dependencies**: Jekyll (GitHub Pages), markdownlint-cli2, yaml-language-server (for schema validation)
**Storage**: Filesystem (docs/ directory structure), GitHub Pages for hosting
**Testing**: markdownlint validation, markdown-link-check for broken links, manual review against actual CLI output
**Target Platform**: Cross-platform documentation (web browsers, offline markdown readers)
**Project Type**: Documentation project (single repository)
**Performance Goals**: Documentation pages load within 2 seconds, search results within 1 second (GitHub Pages default performance)
**Constraints**: Must maintain existing docs/ structure, preserve Jekyll front matter, follow project markdown style guide (.markdownlint-cli2.jsonc)
**Scale/Scope**: 8 new/updated documentation files, ~15-20 example configurations, 6-8 screenshots/GIFs

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Verify compliance with PulumiCost Core Constitution (`.specify/memory/constitution.md`):

- [x] **Plugin-First Architecture**: N/A - Documentation feature, no plugin implementation
- [x] **Test-Driven Development**: Documentation validation through markdownlint, link checking, and manual verification against actual tool behavior
- [x] **Cross-Platform Compatibility**: Documentation accessible on all platforms via GitHub Pages and local markdown readers
- [x] **Documentation Synchronization**: This IS the documentation synchronization feature - implements Principle IV directly
- [x] **Protocol Stability**: N/A - No protocol changes
- [x] **Implementation Completeness**: All documentation files will be complete with no TODOs or placeholder content
- [x] **Quality Gates**: markdownlint, markdown-link-check will pass before PR merge
- [x] **Multi-Repo Coordination**: N/A - Single repository documentation update

**Violations Requiring Justification**: None

**Constitution Alignment Notes**:
- This feature directly implements **Principle IV: Documentation Synchronization & Quality**
- Addresses documentation debt from features #222 (TUI), #216 (recommendations), #217 (budget alerts), #224 (accessibility)
- Ensures `README.md` and `docs/` remain synchronized with implemented features
- Provides audience-specific guides (User, Developer) as required by Principle IV

## Project Structure

### Documentation (this feature)

```text
specs/119-tui-docs-update/
├── plan.md              # This file (/speckit.plan command output)
├── research.md          # Phase 0 output: Documentation best practices, Jekyll conventions
├── data-model.md        # Phase 1 output: Documentation structure and content taxonomy
├── quickstart.md        # Phase 1 output: Quick reference for documentation authors
├── contracts/           # Phase 1 output: Content templates and style guide
└── tasks.md             # Phase 2 output (/speckit.tasks command - NOT created by /speckit.plan)
```

### Documentation Files (repository docs/ directory)

```text
docs/
├── guides/
│   ├── budgets.md           # NEW: Budget configuration guide (FR-001)
│   ├── recommendations.md   # NEW: Recommendations guide (FR-002)
│   └── accessibility.md     # NEW: Accessibility features guide (FR-003)
├── reference/
│   ├── cli-commands.md      # UPDATED: Add "cost recommendations" command (FR-004)
│   └── config-reference.md  # UPDATED: Add budgets section (FR-005)
├── examples/
│   ├── config-budgets/      # NEW: Budget configuration examples (FR-007)
│   │   ├── single-threshold.yaml
│   │   ├── multiple-thresholds.yaml
│   │   ├── cicd-integration.yaml
│   │   ├── webhook-notifications.yaml
│   │   └── provider-specific.yaml
│   └── README.md            # UPDATED: Index of examples
├── assets/
│   └── screenshots/         # NEW: Visual examples (FR-009)
│       ├── budget-tty-mode.png
│       ├── budget-plain-mode.png
│       ├── recommendations-table.png
│       ├── recommendations-detail.png
│       ├── loading-spinner.gif
│       └── error-messages.png
├── schemas/
│   └── config-schema.json   # NEW: JSON schema for config.yaml (FR-008)
├── README.md                # UPDATED: Feature overview (FR-006)
└── TABLE-OF-CONTENTS.md     # UPDATED: Add new guides

# Root directory
README.md                    # UPDATED: Link to new documentation (FR-006)
```

**Structure Decision**: Existing docs/ structure is maintained. New guides go in docs/guides/ following established patterns (architect-guide.md, developer-guide.md, user-guide.md). Reference documentation updates follow existing file organization. Examples and schemas get dedicated subdirectories for maintainability.

## Complexity Tracking

> **Fill ONLY if Constitution Check has violations that must be justified**

N/A - No constitution violations. This feature directly implements Principle IV requirements.

## Phase 0: Research & Best Practices

**Objective**: Establish documentation patterns, style conventions, and content structure templates.

### Research Tasks

1. **Jekyll & GitHub Pages Conventions**
   - **Question**: What Jekyll front matter is required for docs/* pages?
   - **Research**: Review existing docs/*.md files for front matter patterns, _config.yml settings
   - **Output**: Front matter template with required fields (layout, title, permalink)

2. **Markdown Style Guide**
   - **Question**: What markdown conventions does the project use?
   - **Research**: Analyze .markdownlint-cli2.jsonc rules, review existing guides for heading structure, code block styles
   - **Output**: Style guide section for documentation authors (heading levels, code fence languages, table formatting)

3. **Cross-Linking Strategy**
   - **Question**: How should documentation be cross-linked to maximize discoverability?
   - **Research**: Review TABLE-OF-CONTENTS.md structure, existing guide cross-references, README navigation patterns
   - **Output**: Cross-linking template with common patterns (See also sections, inline references, breadcrumb navigation)

4. **Visual Example Formats**
   - **Question**: What screenshot/GIF formats and dimensions work best in Jekyll/GitHub Pages?
   - **Research**: Check existing docs/assets/ usage, GitHub-flavored Markdown image syntax, responsive image guidelines
   - **Output**: Visual example guidelines (PNG for static, GIF for animations, max width 800px, alt text requirements)

5. **Configuration Example Patterns**
   - **Question**: How should YAML examples be structured and documented?
   - **Research**: Review existing configuration examples in docs/, analyze inline comments vs. separate documentation approaches
   - **Output**: Configuration example template with inline annotations and external reference links

6. **JSON Schema Best Practices**
   - **Question**: What schema features enable best IDE autocompletion experience?
   - **Research**: Review yaml-language-server documentation, analyze popular JSON schemas (Kubernetes, GitHub Actions)
   - **Output**: Schema structure template with descriptions, examples, enum values, pattern validation

**Deliverable**: `research.md` documenting all findings with examples and templates

## Phase 1: Content Design & Templates

**Prerequisites**: `research.md` complete

### Data Model: Documentation Content Taxonomy

Generate `data-model.md` defining:

**Documentation Entities**:
- **Guide**: Long-form tutorial/explanation document
  - Attributes: title, target_audience, prerequisites, learning_objectives, sections
  - Relationships: references CLI Reference entries, links to Examples, points to Configuration Reference

- **Reference Entry**: Factual reference documentation
  - Attributes: command_name, synopsis, flags (name, type, default, description), examples
  - Relationships: referenced by Guides, links to Configuration Reference

- **Configuration Example**: Complete YAML file demonstrating a scenario
  - Attributes: scenario_name, yaml_content, inline_comments, use_case_description
  - Relationships: referenced by Guides, validates against Schema

- **Visual Example**: Screenshot or GIF demonstrating tool output
  - Attributes: filename, alt_text, caption, context, display_mode (TTY/plain/high-contrast)
  - Relationships: embedded in Guides, referenced in README

- **JSON Schema**: Machine-readable configuration validation
  - Attributes: schema_version, definitions, required_fields, validation_rules
  - Relationships: validates Configuration Examples, enables IDE features

**Content Flow**: README → Guides → Reference + Examples ← Schema

### Contracts: Content Templates

Generate `contracts/` directory with:

1. **`guide-template.md`**: Structure for budgets.md, recommendations.md, accessibility.md
   ```markdown
   ---
   layout: default
   title: [Guide Title]
   permalink: /guides/[slug]/
   ---

   # [Guide Title]

   ## Overview
   [Brief introduction, target audience, what users will learn]

   ## Quick Start
   [Copy-paste example, minimal configuration]

   ## Configuration Reference
   [Table of all options with types, defaults, descriptions]

   ## Examples
   [Common scenarios with explanations]

   ## Troubleshooting
   [Common issues and solutions]

   ## See Also
   [Cross-links to CLI reference, config reference, examples]
   ```

2. **`cli-reference-entry.md`**: Template for "cost recommendations" documentation
   ```markdown
   ## finfocus cost recommendations

   Display cost optimization recommendations from cloud providers.

   ### Synopsis

   ```bash
   finfocus cost recommendations [flags]
   ```

   ### Flags

   | Flag | Type | Default | Description |
   |------|------|---------|-------------|
   | --pulumi-json | string | (required) | Path to Pulumi plan JSON |
   | --filter | string | "" | Filter expression |
   | --output | string | "table" | Output format (table, json, ndjson) |

   ### Examples

   ```bash
   # Basic usage
   finfocus cost recommendations --pulumi-json plan.json

   # With filters
   finfocus cost recommendations --pulumi-json plan.json --filter "priority=high"
   ```
   ```

3. **`config-example-template.yaml`**: Structure for budget examples
   ```yaml
   # [Scenario Name]
   # Use case: [Description of when to use this configuration]
   # Related docs: [Link to guide section]

   cost:
     budgets:
       amount: [value]
       currency: USD
       period: monthly
       alerts:
         - threshold: [value]
           type: [actual|forecasted]
           # [Inline explanation of this alert]
   ```

4. **`screenshot-checklist.md`**: Requirements for visual examples
   - Resolution: 1600x900 minimum for clarity
   - Format: PNG for static, GIF (<5MB) for animations
   - Annotations: Red boxes for UI elements, labels for keyboard shortcuts
   - Context: Show full terminal window with command prompt
   - Accessibility: High contrast, readable text (14pt minimum)

### Quickstart Guide

Generate `quickstart.md` for documentation authors:

```markdown
# Documentation Quick Reference

## Creating a New Guide

1. Copy `contracts/guide-template.md` to `docs/guides/[slug].md`
2. Fill front matter: layout, title, permalink
3. Write content following template sections
4. Add cross-links to CLI reference and examples
5. Update `docs/TABLE-OF-CONTENTS.md`
6. Run `make docs-lint` to validate

## Adding CLI Reference Entry

1. Edit `docs/reference/cli-commands.md`
2. Use `contracts/cli-reference-entry.md` template
3. Include flag table with all required columns
4. Provide 3-5 examples covering common use cases
5. Cross-link from guides

## Creating Configuration Examples

1. Create YAML file in `docs/examples/config-budgets/`
2. Follow `contracts/config-example-template.yaml` structure
3. Add inline comments explaining each field
4. Reference from budget guide
5. Validate against `docs/schemas/config-schema.json`

## Capturing Screenshots

1. Review `contracts/screenshot-checklist.md`
2. Run tool in clean terminal (no extra output)
3. Capture at 2x resolution for retina displays
4. Crop to relevant content (remove extra whitespace)
5. Compress with `pngquant` or `optipng`
6. Add to `docs/assets/screenshots/`
7. Reference with descriptive alt text

## Cross-Linking Best Practices

- Use relative links: `[budgets guide](../guides/budgets.md)`
- Add "See Also" sections at guide end
- Link CLI flags to configuration reference
- Reference examples from guides
- Update README.md navigation section
```

### Agent Context Update

Run agent context update script:

```bash
.specify/scripts/bash/update-agent-context.sh claude
```

This updates `.specify/memory/claude.md` with:
- Documentation authoring workflow
- Jekyll/GitHub Pages conventions
- Markdown style guidelines
- Cross-linking patterns
- Visual example requirements

**Deliverable**: `data-model.md`, `contracts/` directory, `quickstart.md`, updated `.specify/memory/claude.md`

## Phase 2: Task Decomposition

**Prerequisites**: Phase 0 and Phase 1 complete

**Note**: Task decomposition happens via `/speckit.tasks` command (separate from `/speckit.plan`).

Expected task structure based on functional requirements:

1. **Setup & Infrastructure** (dependencies, schema, templates)
2. **Budget Configuration Guide** (budgets.md, examples, cross-links)
3. **Recommendations Guide** (recommendations.md, keyboard shortcuts table, filtering examples)
4. **Accessibility Guide** (accessibility.md, environment variable reference, visual comparisons)
5. **CLI Reference Updates** (cli-commands.md additions for cost recommendations)
6. **Configuration Reference Updates** (config-reference.md budget section)
7. **Visual Examples** (screenshots, GIFs for all modes and states)
8. **README Updates** (feature overview, quick start, navigation links)
9. **Cross-Linking & Navigation** (TABLE-OF-CONTENTS.md, See Also sections)
10. **Validation & Quality Assurance** (markdownlint, link checking, manual review)

Tasks will be generated by `/speckit.tasks` after plan approval.

## Dependencies & Coordination

### Feature Dependencies

This documentation update depends on the following features being implemented:

- **#222 - Shared TUI Package**: Bubble Tea/Lip Gloss integration (MUST be complete)
- **#216 - Recommendations Display**: Cost recommendations command and TUI (MUST be complete)
- **#217 - Budget Alerts**: Budget configuration and alert system (MUST be complete)
- **#224 - Accessibility Options**: Color/contrast/plain text flags (MUST be complete)

**Verification**: Before starting documentation work, confirm all dependency features are merged to main branch and their functionality is accessible in the CLI tool.

### External Dependencies

- Jekyll 3.9.0+ (GitHub Pages default)
- markdownlint-cli2 0.11.0+ (already in project)
- yaml-language-server (for schema validation, user-installed)
- Screenshot tools: Terminal emulator with clean output, screen capture utility
- Image optimization: `pngquant` or `optipng` (optional but recommended)

### Multi-Repo Coordination

N/A - This is a single-repository documentation update. No cross-repo protocol changes.

## Success Criteria Validation

The implementation will be considered complete when:

1. **SC-001**: New user can configure budget with alerts in under 5 minutes using Budget Configuration Guide
   - **Validation**: User testing with fresh FinFocus installation, timed configuration task

2. **SC-002**: User searching for "recommendations" or "budget" finds guide within first 3 results
   - **Validation**: Search functionality test in GitHub Pages, verify TABLE-OF-CONTENTS ordering

3. **SC-003**: All deliverables completed (budgets.md, recommendations.md, accessibility.md, updated cli.md, config-reference.md, README.md, examples, schema)
   - **Validation**: Checklist verification in PR, file existence check

4. **SC-004**: Visual examples included for all scenarios (budget TTY, budget plain, recommendations table, detail view, spinner, errors)
   - **Validation**: File count in docs/assets/screenshots/, visual review of each screenshot

5. **SC-005**: Users can enable IDE autocompletion following JSON schema instructions
   - **Validation**: Test in VS Code with yaml-language-server, verify autocompletion works

6. **SC-006**: At least 5 example configurations (single threshold, multiple thresholds, exit codes, webhooks, provider-specific)
   - **Validation**: File count in docs/examples/config-budgets/, scenario coverage check

7. **SC-007**: All CLI flags documented (type, description, default, examples)
   - **Validation**: Compare CLI --help output to documented flags, verify completeness

8. **SC-008**: Cross-links enable 2-click navigation (guides ↔ reference ↔ examples)
   - **Validation**: Navigation flow test from each major section, count clicks to reach related content

9. **SC-009**: Accessibility documentation enables no-color, high-contrast, plain text configuration without trial/error
   - **Validation**: User testing with accessibility requirements, verify single-pass configuration

10. **SC-010**: README showcases new features with visual example and links to guides
    - **Validation**: README review, verify screenshot presence and link validity

## Risk Assessment

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Dependency features not complete | Medium | High | Verify feature completion before starting documentation work; coordinate with feature owners |
| Screenshots become outdated after UI changes | High | Medium | Document screenshot capture process; version screenshots with feature version; plan refresh cycle |
| JSON schema doesn't match actual config structure | Medium | High | Generate schema from actual config parsing code if possible; validate against real config files |
| Cross-links break during docs restructuring | Low | Medium | Use markdown-link-check in CI; relative links preferred over absolute |
| Documentation doesn't match actual CLI behavior | Medium | Critical | Write docs from actual CLI output; test every example before publishing |

## Next Steps

1. **Review this plan** with stakeholders
2. **Verify dependency features** (#222, #216, #217, #224) are complete and merged
3. **Run `/speckit.tasks`** to generate actionable task list
4. **Begin Phase 0 research** to establish documentation patterns and templates
5. **Create content templates** (Phase 1) for consistent documentation structure
6. **Implement documentation** following generated task list
7. **Validate against success criteria** before marking feature complete

---

**Plan Status**: Ready for task generation via `/speckit.tasks`
**Estimated Complexity**: Medium (documentation-only, no code changes, but requires coordination with multiple feature dependencies)
**Estimated Timeline**: 2-3 days for full documentation suite (assumes dependency features complete)
