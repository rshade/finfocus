# Implementation Plan: Plugin Install/Update/Remove System

**Branch**: `006-plugin-install` | **Date**: 2025-11-23 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/006-plugin-install/spec.md`

## Summary

Implement a comprehensive plugin installation system enabling users to install, update, and remove plugins from a well-known registry or GitHub URLs. The system downloads platform-specific binaries via GitHub Releases API, manages version pinning via config.yaml, auto-installs missing plugins on startup, and resolves plugin dependencies automatically.

## Technical Context

**Language/Version**: Go 1.25.6
**Primary Dependencies**: archive/tar, archive/zip, compress/gzip, net/http, github.com/spf13/cobra, gopkg.in/yaml.v3
**Storage**: File system (~/.finfocus/plugins/, ~/.finfocus/config.yaml)
**Testing**: go test with race detection, 80% minimum coverage
**Target Platform**: Linux (amd64, arm64), macOS (amd64, arm64), Windows (amd64)
**Project Type**: Single CLI application
**Performance Goals**: Plugin install < 30 seconds on standard broadband
**Constraints**: Network retry with 3 attempts, max 7 seconds total; GitHub API rate limits (60/hr unauthenticated)
**Scale/Scope**: Typical user has 2-5 plugins installed

## Constitution Check

_GATE: Must pass before Phase 0 research. Re-check after Phase 1 design._

Verify compliance with FinFocus Core Constitution (`.specify/memory/constitution.md`):

- [x] **Plugin-First Architecture**: This is core orchestration logic for managing plugins, not a plugin itself. Correctly in core.
- [x] **Test-Driven Development**: Tests planned before implementation with 80% minimum coverage target.
- [x] **Cross-Platform Compatibility**: Platform-specific binary detection and archive handling (tar.gz/zip) implemented.
- [x] **Documentation as Code**: User guide updates planned for plugin management commands.
- [x] **Protocol Stability**: No protocol changes required; uses existing plugin manifest format.
- [x] **Quality Gates**: All CI checks (tests, lint, security) will pass before merge.
- [x] **Multi-Repo Coordination**: Uses registry.proto from finfocus-spec; no breaking changes.

**Violations Requiring Justification**: None

## Project Structure

### Documentation (this feature)

```text
specs/006-plugin-install/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # Phase 1 output (internal APIs, not REST)
└── tasks.md             # Phase 2 output (/speckit.tasks command)
```

### Source Code (repository root)

```text
internal/
├── cli/
│   ├── plugin_install.go      # Install command
│   ├── plugin_update.go       # Update command
│   ├── plugin_remove.go       # Remove command
│   └── plugin_install_test.go # Command tests
├── registry/
│   ├── embed.go               # Registry JSON embedding
│   ├── entry.go               # Registry entry types
│   ├── github.go              # GitHub API client
│   ├── archive.go             # Archive extraction
│   ├── version.go             # Semver parsing
│   ├── dependency.go          # Dependency resolution
│   └── registry_test.go       # Registry tests
├── config/
│   ├── plugins.go             # Plugin config management
│   └── config.go              # (modify for plugins section)
└── pluginhost/
    └── (existing files)

registry/
└── registry.json              # Embedded plugin registry

test/
├── integration/
│   └── plugin_install_test.go # E2E tests
└── fixtures/
    └── plugins/               # Mock plugin archives
```

**Structure Decision**: Single project structure following existing finfocus-core patterns. New `internal/registry/` package for plugin management, extending existing `internal/config/` for plugin persistence.

## Complexity Tracking

No violations - no complexity tracking needed.
