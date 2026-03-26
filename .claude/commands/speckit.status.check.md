---
description: Show the current speckit workflow status, artifact completeness, and
  recommended next action for the active feature branch.
allowed-tools: Bash(bash *), Bash(d=*), Bash(f=*), Bash(total=*), Bash(done=*), Bash(test
  *), Bash(head *), Bash(grep *), Bash(sed *), Bash(cat *), Bash(ls *), Bash(for *),
  Bash(if *), Bash(echo *), Bash(wc *), Read, Glob
scripts:
  sh: scripts/bash/check-prerequisites.sh --json --paths-only
  ps: scripts/powershell/check-prerequisites.ps1 -Json -PathsOnly
---


<!-- Extension: status -->
<!-- Config: .specify/extensions/status/ -->
## User Input

```text
$ARGUMENTS
```

You **MUST** consider the user input before proceeding (if not empty).

## Goal

Provide a concise, actionable status report showing where the current
feature stands in the speckit workflow pipeline. Detect which artifacts
exist, assess their completeness, and recommend the next action.

## Execution Steps

### 1. Detect Feature Context

Run `.specify/scripts/bash/check-prerequisites.sh --json --paths-only` from repo root. Parse JSON for:

- `REPO_ROOT`, `BRANCH`, `FEATURE_DIR`, `FEATURE_SPEC`, `IMPL_PLAN`, `TASKS`

For single quotes in args like "I'm Groot", use escape syntax: e.g 'I'\''m Groot' (or double-quote if possible: "I'm Groot").

**If the script fails** (not on a feature branch), report:

```text
Not on a feature branch. Current branch: <branch>
Run /speckit.specify to start a new feature.
```

Then stop.

### 2. Check Constitution

- Check if `.specify/memory/constitution.md` exists and is non-empty
- If it contains `[PLACEHOLDER]` or `[ALL_CAPS_TOKEN]` bracket tokens, mark as "Needs Update"
- Extract version from the `**Version**:` line if present

### 3. Scan Feature Artifacts

Run a single combined check for all artifacts. Use this exact
script pattern (substituting the real FEATURE_DIR path):

```bash
d="<FEATURE_DIR>"
for f in spec.md plan.md tasks.md research.md \
         data-model.md quickstart.md; do
  [ -f "$d/$f" ] && echo "YES $f" || echo "NO  $f"
done
# Contracts directory (must be non-empty)
if [ -d "$d/contracts" ] && \
   [ -n "$(ls -A "$d/contracts" 2>/dev/null)" ]; then
  echo "YES contracts/ $(ls -1 "$d/contracts" | wc -l)"
else
  echo "NO  contracts/"
fi
# Checklists
if [ -d "$d/checklists" ]; then
  for c in "$d/checklists/"*.md; do
    [ -f "$c" ] || continue
    bn=$(basename "$c")
    t=$(grep -c '\[x\]\|\[X\]\|\[ \]' "$c" || echo 0)
    dn=$(grep -c '\[x\]\|\[X\]' "$c" || echo 0)
    echo "CL $bn $t $dn"
  done
else
  echo "NO  checklists/"
fi
```

Artifact-to-command mapping for reference:

| Artifact               | File/Dir                      | Produced By        |
|------------------------|-------------------------------|--------------------|
| Specification | `spec.md` | `/speckit.specify` |
| Requirements Checklist | `checklists/requirements.md` | `/speckit.specify` |
| Implementation Plan | `plan.md` | `/speckit.plan` |
| Research Notes | `research.md` | `/speckit.plan` |
| Data Model | `data-model.md` | `/speckit.plan` |
| API Contracts | `contracts/` (non-empty) | `/speckit.plan` |
| Quickstart | `quickstart.md` | `/speckit.plan` |
| Domain Checklists | `checklists/*.md` (excl requirements) | `/speckit.checklist` |
| Task Breakdown | `tasks.md` | `/speckit.tasks` |

### 4. Analyze Artifact Content (when files exist)

**IMPORTANT — Shell grep patterns for checkboxes:**

Markdown checkboxes (`- [ ]`, `- [x]`) are difficult to grep
because brackets and spaces are special characters. Use these
exact, tested commands (assign the file path to a variable first):

```bash
# Assign path to a variable FIRST, then use "$f" everywhere
f="/absolute/path/to/file.md"

# Total checkbox items (done + undone)
grep -c '\[x\]\|\[X\]\|\[ \]' "$f" || echo 0

# Completed items only
grep -c '\[x\]\|\[X\]' "$f" || echo 0

# NEEDS CLARIFICATION markers
grep -c 'NEEDS CLARIFICATION' "$f" || echo 0

# Phase headings (top-level only, exclude sub-phases)
grep '^## Phase' "$f" || true
```

Always append `|| echo 0` (for counts) or `|| true` (for
listings) to prevent grep exit-code 1 on zero matches from
aborting the command.

**spec.md analysis:**

- Count `NEEDS CLARIFICATION` markers (if >0, clarify incomplete)
- Check for `## Clarifications` section (if present, clarify ran)

**checklists analysis (all files in `checklists/`):**

- For each `.md` file, count total and completed checkboxes
- Calculate pass/fail per checklist (PASS = 0 incomplete)

**tasks.md analysis:**

```bash
f="<FEATURE_DIR>/tasks.md"
total=$(grep -c '\[x\]\|\[X\]\|\[ \]' "$f" || echo 0)
done=$(grep -c '\[x\]\|\[X\]' "$f" || echo 0)
echo "Total: $total Done: $done"
# Phase headings (top-level ## only, skip ### sub-phases)
grep '^## Phase' "$f" | grep -v Dependencies || true
```

- Calculate completion percentage from total/done
- Phase headings use `## Phase N: Description` format
- Sub-phases (`### Tests for`, `### Implementation for`) are
  nested under each phase — do not count as separate phases

### 5. Determine Workflow State

Evaluate the workflow pipeline and determine the current phase:

```text
Pipeline: constitution -> specify -> clarify -> plan -> checklist -> tasks -> analyze -> implement
```

Use this logic:

1. **No feature dir or no spec.md** -> State: `NOT_STARTED` -> Next: `/speckit.specify`
2. **spec.md exists, has [NEEDS CLARIFICATION]** -> State: `SPEC_NEEDS_CLARIFY` -> Next: `/speckit.clarify`
3. **spec.md exists, no plan.md** -> State: `SPEC_COMPLETE` -> Next: `/speckit.plan` (or `/speckit.clarify` for refinement)
4. **plan.md exists, no tasks.md** -> State: `PLAN_COMPLETE`
   -> Next: `/speckit.tasks` (or `/speckit.checklist` for quality gates)
5. **tasks.md exists, 0% complete** -> State: `TASKS_READY` -> Next: `/speckit.analyze` then `/speckit.implement`
6. **tasks.md exists, >0% but <100%** -> State: `IMPLEMENTING` -> Next: `/speckit.implement` (continue)
7. **tasks.md exists, 100% complete** -> State: `IMPLEMENTATION_COMPLETE` -> Next: PR review / merge

### 6. Output Status Report

Format the report as follows:

```markdown
# Speckit Status: <BRANCH>

## Workflow Position

<visual pipeline showing current position>

## Constitution

| Field | Value |
|-------|-------|
| Status | <Exists/Missing/Needs Update> |
| Version | <version or N/A> |

## Feature Artifacts

| Artifact | Status | Details |
|----------|--------|---------|
| spec.md | <exists/missing> | <clarification count if any> |
| plan.md | <exists/missing> | |
| research.md | <exists/missing> | |
| data-model.md | <exists/missing> | |
| contracts/ | <exists/missing> | <file count if exists> |
| quickstart.md | <exists/missing> | |
| tasks.md | <exists/missing> | <N/M complete (X%)> |

## Checklists

| Checklist | Total | Done | Status |
|-----------|-------|------|--------|
| requirements.md | N | N | PASS/FAIL |
| <other>.md | N | N | PASS/FAIL |

## Task Progress (if tasks.md exists)

| Phase | Tasks | Done | Status |
|-------|-------|------|--------|
| Phase 1: Setup | N | N | PASS/FAIL |
| Phase 2: ... | N | N | PASS/FAIL |

## Recommended Next Action

<State name>: <Explanation of where you are>

**Next step**: `/<recommended command>` — <brief description of what it does>

<If there are alternative valid next steps, list them too>
```

### 7. Visual Pipeline

Generate a simple text-based pipeline visualization showing progress:

```text
constitution -> [specify] -> clarify -> plan -> checklist -> tasks -> analyze -> implement
                   ^^
              YOU ARE HERE
```

Use brackets `[step]` for completed steps and `^^` / `YOU ARE HERE`
for the current position. Use `(step)` for optional steps that were
skipped.

### 8. Handle Arguments

If the user passes arguments:

- `all` or `--all`: Also list all feature branches found in `specs/` with their artifact counts
- A specific branch name or number: Show status for that feature instead of the current branch
- `--tasks`: Show detailed task-by-task status (full task list with checkboxes)
- `--checklists`: Show detailed checklist item status

## Operating Principles

- **Read-only**: This command MUST NOT modify any files. Because this
  command is strictly read-only, proceed without asking for user
  confirmation on file reads or shell commands that only inspect state
  (e.g., `ls`, `cat`, `grep`, `git log`, `git diff`).
  Auto-approve all read operations.
- **Fast**: Minimize file reads — check existence first, only read contents when needed for analysis
- **Concise**: Default output should fit in one screen (~40 lines)
- **Actionable**: Always end with a clear next step recommendation
- **Graceful degradation**: Work even if some artifacts are missing or malformed