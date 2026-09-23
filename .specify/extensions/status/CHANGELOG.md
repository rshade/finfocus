# Changelog

All notable changes to this extension will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [1.0.0] - 2026-03-08

### Added

- Initial release of spec-kit-status extension
- Command: `/speckit.status.status` (alias `/speckit.status`) — workflow status dashboard
- Feature context detection via `check-prerequisites.sh`
- Constitution health check (exists, version, placeholder detection)
- Artifact scanning for all speckit pipeline outputs (spec, plan, tasks, checklists, contracts, etc.)
- Checklist completion analysis with pass/fail per checklist
- Task progress breakdown by phase
- Visual pipeline position indicator
- Workflow state determination with recommended next action
- Argument support: `--all`, `--tasks`, `--checklists`, branch name

---

[Unreleased]: https://github.com/rshade/spec-kit-status/compare/v1.0.0...HEAD
[1.0.0]: https://github.com/rshade/spec-kit-status/releases/tag/v1.0.0
