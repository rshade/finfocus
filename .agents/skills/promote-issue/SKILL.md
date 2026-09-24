---
name: promote-issue
description: >
  Triage the finfocus roadmap backlog and promote the next issue(s) to roadmap/current.
  Use when the user asks "what should I work on next", "what's next on the roadmap",
  "promote an issue to current", or wants to triage roadmap/next and roadmap/future
  before starting unplanned work. Ranks open roadmap/next and roadmap/future issues by
  a documented scoring rubric (effort, unblocks, epic promotion, triggers), respects
  suppression rules (parked, timebox/watch, unfired trigger-pending), and presents the top
  candidates for selection. Selection and promotion only; to claim and implement an
  issue, use pick-issue. Triggers on: "promote issue", "roadmap promote",
  "promote to current", "what's ready to work", "triage the backlog".
---

# Promote Issue

Interactive selection of the next issue(s) to work, with optional promotion to
`roadmap/current`. This is the standalone promotion workflow extracted from the
`/roadmap sync` promotion gate — it does not rewrite ROADMAP.md wholesale or
sync labels repo-wide.

## Preconditions

- `gh` CLI authenticated for `rshade/finfocus`.
- Repo root has `ROADMAP.md` and `CONTEXT.md`. If either is missing, skip the
  file-update steps below and limit yourself to label flips — say so in the
  summary.
- Read `CONTEXT.md` first. Any candidate that would cross a CONTEXT.md boundary
  must be flagged, not silently promoted.

## Step 1 — Gather candidates

```bash
# Phase labels: roadmap/current | roadmap/next | roadmap/future (mutually exclusive)
gh issue list --state open --label roadmap/next --limit 100 \
  --json number,title,labels,body,milestone
gh issue list --state open --label roadmap/future --limit 100 \
  --json number,title,labels,body,milestone
# Cross-check what is already in flight
gh issue list --state open --label roadmap/current --limit 100 --json number,title
```

Excluded from candidacy (never present them):

- Any label in the noise set: `nightly-failure`, `dependencies`, `agentic-workflows`.
- `roadmap/exclude` — dominant; if it coexists with a phase label, report the
  conflict and leave it to the operator.
- Already `roadmap/current` (already in flight; listed only for WIP context).

## Step 2 — Parse `roadmap-meta`

Issues may carry structured metadata in an HTML comment in the body:

```html
<!-- roadmap-meta
trigger-pending: <YYYY-MM-DD | git-tag | #issue>
unblocks: 38, 39, 40
epic-parent: 36
gates: 53, 263
spike-parent: 524
verdict: go | needs-clarification | kill | parked
-->
```

Extract text between `<!-- roadmap-meta` and `-->`, then read `key: value`
lines. All fields optional; absent block = "no metadata, always eligible".
Adoption is low — every rule below must work when the field is absent.

## Step 3 — Apply suppression

A candidate is **not workable now** if any hold:

- `parked` label, or `verdict: parked` — deliberately not being worked.
- `timebox/watch` — gated on a third party; it would block the slot, not clear it.
- `trigger-pending:` present and unfired. Fires when: a `YYYY-MM-DD` today or
  past; a git tag that exists (`git tag --list <value>` non-empty); an `#issue`
  that is closed. Free-form values never fire automatically.
- `spike-parent:` naming another **open** spike — the parent resolves first.
- A `trigger-pending:` naming a milestone (`v0.1.0`, `MVP`) suppresses ripeness
  entirely until the milestone lands — scope decisions are not reversible by
  scoring.

Suppressed candidates are reported as blocked with the reason, never promoted.

## Step 4 — Score remaining candidates

| Signal | Score | Detection |
| ------ | ----- | --------- |
| Fills missing `composition_required` slot | +30 | Label listed in CONTEXT.md `composition_required` that no open `roadmap/current` issue has |
| Epic-promotion-eligible | +15 | `epic-parent:` references a **closed** issue |
| `effort/small` | +10 | Label present |
| `effort/medium` | +5 | Label present |
| Has dependents | +10 | Another open issue's `unblocks:` lists this number |
| Community-leverage when pipeline thin | +8 | `community` label AND < 2 `community` issues closed in last 90d |
| `spec-first` but no `roadmap-meta` block | -3 | Soft nudge |
| Trigger pending, unfired | -20 | Step 3 |
| Trigger fired | +25 | `trigger-pending:` condition has arrived |
| `spike` gating an `effort/large` item | +12 | Spike ripens a gated large issue |
| `spike` gating an `effort/medium` item | +6 | Same, medium |
| `timebox/watch` | -25 | Step 3 |

## Step 5 — Present candidates

Present the top **3–4** ranked candidates via `AskUserQuestion`.

- **Label:** `#NN [S|M|L] label-summary — one-line headline`. Top-scored item
  gets a " (Recommended)" suffix.
- **Description:** top two positive contributors, any negative contributor, and
  a one-line "why this slot" rationale.

Selection mode:

- `roadmap/current` has fewer open issues than the WIP target (default target:
  CONTEXT.md `target_focus_depth`, else 3) → **single-select** if one slot,
  **multi-select** (max = open slots) if the gate bumped capacity.
- If everything is suppressed, present the blocked reasons and the escape hatch
  only.

Always append the escape hatch as the final option:

- **Label:** "Deliberately empty (release in flight)"
- **Description:** "Records 'paused' state. Resume promotion on next run."

## Step 6 — Resolve and promote

1. **Escape hatch selected:** follow up with a single `AskUserQuestion` for a
   one-line reason, then write into ROADMAP.md Immediate Focus:

   ```text
   *Deliberately empty (YYYY-MM-DD): <reason>. Set by /promote-issue.*
   ```

   Replace any existing such annotation in-place.

2. **Candidate(s) selected:** for each, flip labels and update the roadmap:

   ```bash
   gh issue edit <n> --add-label roadmap/current --remove-label roadmap/next
   # or --remove-label roadmap/future, whichever it carried
   ```

   Move each entry from its current ROADMAP.md section into "Immediate Focus",
   preserving description and `[S|M|L]` indicator, ordered by score (highest
   first). Append provenance: `*Promoted by /promote-issue on YYYY-MM-DD — <rationale>*`.

3. **No selection (multi-select only):** no file or label changes. Note "no
   decision — re-prompts next run" in the summary.

## Summary output

Report: candidates presented with scores, the operator's decision, issues
promoted (with new labels), any suppression/block reasons for near-miss
candidates, and any CONTEXT.md boundary flags.
