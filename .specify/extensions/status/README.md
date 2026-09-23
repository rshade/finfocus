# Spec Kit Status

A [Spec Kit](https://github.com/statsperform/spec-kit) extension that provides a
workflow status dashboard for feature branches. Shows artifact completeness,
checklist progress, task breakdown, and recommends the next action in the
speckit pipeline.

## Installation

```bash
# Install from catalog
specify extension add status

# Or install from GitHub
specify extension add --dev /path/to/spec-kit-status
```

## Usage

```bash
# Show status for current feature branch
> /speckit.status

# Full command name also works
> /speckit.status.status
```

### Arguments

| Argument        | Description                                 |
|-----------------|---------------------------------------------|
| *(none)* | Show status for the current feature branch |
| `--all` | List all feature branches with artifact counts |
| `--tasks` | Show detailed task-by-task status with checkboxes |
| `--checklists` | Show detailed checklist item status |
| `<branch-name>` | Show status for a specific feature branch |

### Examples

```bash
# Check where you are in the workflow
> /speckit.status

# See all features across the repo
> /speckit.status --all

# Detailed task progress
> /speckit.status --tasks

# Check a specific branch
> /speckit.status feature/auth-system
```

## Pipeline Overview

The status command tracks your position in the 8-stage speckit workflow:

```text
constitution -> specify -> clarify -> plan -> checklist -> tasks -> analyze -> implement
```

It determines your current state by checking which artifacts exist and their completeness:

| State                      | Condition                    | Next Step          |
|----------------------------|------------------------------|--------------------|
| `NOT_STARTED` | No spec.md | `/speckit.specify` |
| `SPEC_NEEDS_CLARIFY` | spec.md has NEEDS CLARIFICATION | `/speckit.clarify` |
| `SPEC_COMPLETE` | spec.md exists, no plan | `/speckit.plan` |
| `PLAN_COMPLETE` | plan.md exists, no tasks | `/speckit.tasks` |
| `TASKS_READY` | tasks.md at 0% | `/speckit.analyze` |
| `IMPLEMENTING` | tasks.md partially complete | `/speckit.implement` |
| `IMPLEMENTATION_COMPLETE` | tasks.md at 100% | PR review / merge |

## Output Example

```text
# Speckit Status: feature/auth-system

## Workflow Position

constitution -> specify -> clarify -> [plan] -> checklist -> tasks -> analyze -> implement
                                        ^^
                                   YOU ARE HERE

## Feature Artifacts

| Artifact | Status | Details |
|----------|--------|---------|
| spec.md | exists | 0 clarifications needed |
| plan.md | exists | |
| research.md | exists | |
| tasks.md | missing | |

## Recommended Next Action

PLAN_COMPLETE: Specification and plan are ready.

**Next step**: `/speckit.tasks` — Break the plan into implementable tasks
```

## License

MIT License - see [LICENSE](LICENSE) file.

## Support

- **Issues**: <https://github.com/rshade/spec-kit-status/issues>
- **Spec Kit**: <https://github.com/statsperform/spec-kit>

## Changelog

See [CHANGELOG.md](CHANGELOG.md) for version history.
