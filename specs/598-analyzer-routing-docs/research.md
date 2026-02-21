# Research: Document Routing Limits in Analyzer Mode

**Feature**: 598-analyzer-routing-docs
**Date**: 2026-02-20

## Decision 1: Callout Format in routing.md

**Decision**: Use a `> **Note:**` blockquote (no new heading, no ToC update required).

**Rationale**: The markdownlint config (`docs/.markdownlint-cli2.jsonc`) permits inline
HTML for select elements but not general `<div class="note">` patterns. A `>` blockquote
with bold label is the established callout pattern in the existing docs and passes
markdownlint without configuration changes. Adding a new `##` heading would require a
ToC update and shift the document structure unnecessarily for what is a two-paragraph
advisory note.

**Alternatives considered**:

- New `## Limitations in Analyzer Mode` heading — rejected; requires ToC update and
  adds a top-level section for content that is a side-note, not a primary topic.
- HTML `<div>` callout — rejected; only specific allowed elements in the lint config.

---

## Decision 2: Callout Placement in routing.md

**Decision**: Insert immediately after line 330 (end of "Common Configuration Patterns"
section), before line 331 ("## Validation" heading).

**Rationale**: Confirmed via file inspection — line 330 is the last line of the
"Common Configuration Patterns" section content
(`4. If no specs → return "no cost data available"`). Inserting here places the callout
exactly where the issue requests ("after the routing configuration examples") and ensures
a developer who has just read the configuration patterns encounters the limitation before
moving on to validation commands.

**Alternatives considered**:

- "Advanced Topics" section (line ~594) — rejected per user clarification (Q1 answer: C).
- "Troubleshooting" section — rejected; developer may never reach it when looking for
  configuration guidance.

---

## Decision 3: New Section Placement in analyzer-integration.md

**Decision**: Insert new `## Isolating Plugins in Analyzer Mode` section immediately
before `## See Also` (currently at line 84).

**Rationale**: The file has no Table of Contents, so no ToC update is required. The
"See Also" section is the natural terminal section. Inserting before it places the new
content as the second-to-last section, giving it prominence without disrupting the
existing technical reference flow (Architecture → How It Works → Key Technical Details
→ Internal Types → **Isolating Plugins** → See Also).

**Alternatives considered**:

- Append after "See Also" — rejected; "See Also" should always be the final section.
- Insert between existing sections — rejected; would disrupt the technical reference
  narrative flow.

---

## Decision 4: Issue #750 Reference Format

**Decision**: Use a full GitHub URL: `https://github.com/rshade/finfocus/issues/750`.

**Rationale**: The repository module path confirms `github.com/rshade/finfocus`. Full
URLs work correctly on GitHub Pages (rendered HTML) and in plain markdown readers alike.
Relative `#750` anchors do not resolve in Jekyll-rendered GitHub Pages.

**Alternatives considered**:

- Bare `#750` reference — rejected; does not hyperlink in GitHub Pages output.
- `[#750](../issues/750)` relative path — rejected; unreliable across GitHub Pages URL
  structures.

---

## Decision 5: FR-007 Shell Command Example Format

**Decision**: Inline `bash` fenced code block within `analyzer-integration.md`. No new
script files created. (Per user clarification Q2 answer: A.)

**Rationale**: Keeps scope minimal, consistent with SC-004 (no new files). An inline
example is fully actionable and avoids repo clutter for a pattern users will customize.

**Alternatives considered**:

- `scripts/run-analyzer-preview.sh` — rejected per user clarification.

---

## Decision 6: Symlink Warning Severity

**Decision**: Use `> **Warning:**` label (not `> **Note:**`) for the directory-level
symlink breakage warning within the isolation procedure.

**Rationale**: The broken-symlink behavior (issue #750) causes silent failures — the
registry loads no plugins without error. A "Warning" label is more appropriate than a
"Note" to signal that following the wrong approach has non-obvious consequences. This
distinction is purely semantic and does not affect markdownlint compliance.

**Alternatives considered**:

- `> **Note:**` for both callouts — rejected; conflates an advisory note with a
  failure-risk warning.
